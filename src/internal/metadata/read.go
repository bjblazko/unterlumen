package metadata

import (
	"maps"
	"path/filepath"
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

// sidecarProps are the sidecar properties each field is read from and
// written to; the date goes to both of its own.
var sidecarProps = map[Field]media.Prop{
	DateTaken:   media.PropDateTaken,
	Description: media.PropDescription,
	Creator:     media.PropCreator,
	Rights:      media.PropRights,
	Rating:      media.PropRating,
}

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

// valuesFromMeta is what the file says without exiftool: the date and the
// EXIF text tags the app reads itself, and a video's date.
func valuesFromMeta(path string) fileValues {
	v := fileValues{fields: map[Field]string{}}
	data, err := media.ExtractMeta(path)
	if err != nil || data == nil {
		return v
	}
	if data.DateTaken != nil && data.Tags["DateTakenFrom"] == "" {
		v.fields[DateTaken] = *data.DateTaken
	}
	for f, tag := range map[Field]string{Description: "ImageDescription", Creator: "Artist", Rights: "Copyright"} {
		if s := strings.Trim(data.Tags[tag], `" `); s != "" {
			v.fields[f] = s
		}
	}
	return v
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
	props, err := media.ReadSidecarProps(path)
	if err != nil {
		return nil, FromSidecar, err
	}
	fields := map[Field]string{}
	for f, p := range sidecarProps {
		if v := props[p]; v != "" {
			fields[f] = v
		}
	}
	if fields[DateTaken] == "" && props[media.PropDateCreated] != "" {
		fields[DateTaken] = props[media.PropDateCreated]
	}
	if d, ok := fields[DateTaken]; ok {
		if fields[DateTaken] = normaliseDate(d); fields[DateTaken] == "" {
			delete(fields, DateTaken)
		}
	}
	return fields, FromSidecar, nil
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

// Overlay lays what lies beside a file over what the file says, the way the
// index and the info panel show it: the date taken (marked in Tags as
// DateTakenFrom), the description, creator, rights and rating. data may be
// nil; the result is nil only when there is nothing at all.
func Overlay(path string, data *media.ExifData) *media.ExifData {
	beside, from, err := besideProps(path)
	if err != nil || len(beside) == 0 {
		return data
	}
	if data == nil {
		data = &media.ExifData{}
	}
	tags := maps.Clone(data.Tags)
	if tags == nil {
		tags = map[string]string{}
	}
	if d := beside[DateTaken]; d != "" {
		data.DateTaken = &d
		tags["DateTakenFrom"] = string(from)
	}
	tags[OverlaidTag] = string(from)
	for f, tag := range map[Field]string{Description: "ImageDescription", Creator: "Artist", Rights: "Copyright", Rating: "Rating"} {
		if v := beside[f]; v != "" {
			tags[tag] = v
		}
	}
	data.Tags = tags
	return data
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
