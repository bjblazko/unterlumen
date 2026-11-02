package metadata

import (
	"maps"
	"os"
	"path/filepath"
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

// Read says what each file at paths (absolute) says now. A value beside the
// file — in the sidecar of a photo, the NFO of a video — wins over the file's
// own, and the date the file's name gives comes last.
func Read(paths []string) (map[string]Current, error) {
	inFiles := map[string]fileValues{}
	var photos []string
	for _, p := range paths {
		if !media.IsVideo(p) {
			photos = append(photos, p)
		}
	}
	if len(photos) > 0 && media.CheckExiftool() {
		var err error
		if inFiles, err = readFiles(photos); err != nil {
			return nil, err
		}
	}
	out := make(map[string]Current, len(paths))
	for _, p := range paths {
		file, ok := inFiles[p]
		if !ok {
			file = valuesFromMeta(p)
		}
		beside, keywords, from, err := readBeside(p)
		if err != nil {
			return nil, err
		}
		out[p] = merge(p, file, beside, keywords, from)
	}
	return out, nil
}

// valuesFromMeta is what the file says without exiftool: its own XMP packet,
// then the date and the EXIF text tags the app reads itself.
func valuesFromMeta(path string) fileValues {
	v := fileValues{fields: map[Field]string{}}
	if !media.IsVideo(path) {
		v.fields = embeddedFields(path)
	}
	data, err := media.ExtractMeta(path)
	if err != nil || data == nil {
		return v
	}
	if data.DateTaken != nil && data.Tags["DateTakenFrom"] == "" {
		v.fields[DateTaken] = *data.DateTaken
	}
	for _, spec := range table {
		if s := strings.Trim(data.Tags[spec.tag], `" `); spec.tag != "" && s != "" && v.fields[spec.field] == "" {
			v.fields[spec.field] = s
		}
	}
	return v
}

// embeddedFields are the fields in the XMP packet inside the photo at path.
func embeddedFields(path string) map[Field]string {
	f, err := os.Open(path)
	if err != nil {
		return map[Field]string{}
	}
	defer f.Close()
	return fieldsOfProps(media.EmbeddedProps(f, sidecarProps))
}

// fieldsOfProps turns XMP properties into fields, the date normalised.
func fieldsOfProps(props map[media.Prop]string) map[Field]string {
	fields := map[Field]string{}
	for _, spec := range table {
		if v := props[spec.prop]; spec.prop != (media.Prop{}) && v != "" {
			fields[spec.field] = v
		}
	}
	if fields[DateTaken] == "" && props[propDateCreated] != "" {
		fields[DateTaken] = props[propDateCreated]
	}
	if d, ok := fields[DateTaken]; ok {
		if fields[DateTaken] = normaliseDate(d); fields[DateTaken] == "" {
			delete(fields, DateTaken)
		}
	}
	return fields
}

// readBeside reads the sidecar of a photo or the NFO of a video, with the
// title and the keywords.
func readBeside(path string) (map[Field]string, []string, Source, error) {
	fields, from, err := besideProps(path)
	if err != nil || from == FromNFO {
		_, tags, nfoErr := readNFO(path)
		if err == nil {
			err = nfoErr
		}
		return fields, tags, from, err
	}
	if title, err := media.ReadTitle(path); err == nil && title != "" {
		fields[Title] = title
	}
	keywords, err := media.ReadSidecarKeywords(path)
	return fields, keywords, from, err
}

// besideProps reads the fields beside a file in one read: the sidecar's
// properties without the title, or the NFO's fields.
func besideProps(path string) (map[Field]string, Source, error) {
	if media.IsVideo(path) {
		fields, _, err := readNFO(path)
		return fields, FromNFO, err
	}
	props, err := media.ReadSidecarProps(path, sidecarProps)
	if err != nil {
		return nil, FromSidecar, err
	}
	return fieldsOfProps(props), FromSidecar, nil
}

func merge(path string, file fileValues, beside map[Field]string, keywords []string, from Source) Current {
	c := Current{Fields: map[Field]Value{}, Video: media.IsVideo(path)}
	for _, f := range Fields {
		switch {
		case beside[f] != "":
			c.Fields[f] = Value{beside[f], from}
		case file.fields[f] != "":
			c.Fields[f] = Value{file.fields[f], FromFile}
		}
	}
	if _, ok := c.Fields[DateTaken]; !ok {
		if d := media.DateFromFileName(filepath.Base(path)); d != "" {
			c.Fields[DateTaken] = Value{d, FromFileName}
		}
	}
	switch {
	case len(keywords) > 0:
		c.Keywords, c.KeywordsFrom = keywords, from
	case len(file.keywords) > 0:
		c.Keywords, c.KeywordsFrom = file.keywords, FromFile
	default:
		c.Keywords = []string{}
	}
	return c
}

// OverlaidTag marks in Tags a file's metadata that Overlay changed, so the
// index knows to read the file again when what lies beside it is gone.
const OverlaidTag = "MetadataFrom"

// HasBeside says whether anything beside the file at path has a field that
// Overlay would lay over the file's own.
func HasBeside(path string) bool {
	beside, _, err := besideProps(path)
	return err == nil && len(beside) > 0
}

// Overlay lays what the file's own XMP and what lies beside the file say
// over what its EXIF says, the way the index and the info panel show it: the
// date taken from beside (marked in Tags as DateTakenFrom), and every field
// with a tag in the table — beside first, then the file's XMP, then EXIF.
// data may be nil; the result is nil only when there is nothing at all.
func Overlay(path string, data *media.ExifData) *media.ExifData {
	var embedded map[Field]string
	if !media.IsVideo(path) {
		embedded = embeddedFields(path)
	}
	beside, from, err := besideProps(path)
	if err != nil {
		beside = nil
	}
	if len(beside) == 0 && !hasTagged(embedded) {
		return data
	}
	if data == nil {
		data = &media.ExifData{}
	}
	tags := maps.Clone(data.Tags)
	if tags == nil {
		tags = map[string]string{}
	}
	for _, spec := range table {
		if v := embedded[spec.field]; spec.tag != "" && v != "" && tags[spec.tag] == "" {
			tags[spec.tag] = v
		}
	}
	if len(beside) > 0 {
		layBeside(tags, data, beside, from)
	}
	data.Tags = tags
	return data
}

func hasTagged(fields map[Field]string) bool {
	for _, spec := range table {
		if spec.tag != "" && fields[spec.field] != "" {
			return true
		}
	}
	return false
}

func layBeside(tags map[string]string, data *media.ExifData, beside map[Field]string, from Source) {
	if d := beside[DateTaken]; d != "" {
		data.DateTaken = &d
		tags["DateTakenFrom"] = string(from)
	}
	tags[OverlaidTag] = string(from)
	for _, spec := range table {
		if v := beside[spec.field]; spec.tag != "" && v != "" {
			tags[spec.tag] = v
		}
	}
}

// BesideDateAndRating is the date taken and the rating that lie beside the
// file at path, read in one go for a folder's listing; "" for none.
func BesideDateAndRating(path string) (string, string) {
	beside, _, err := besideProps(path)
	if err != nil {
		return "", ""
	}
	return beside[DateTaken], beside[Rating]
}
