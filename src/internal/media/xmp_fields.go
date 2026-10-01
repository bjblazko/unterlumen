package media

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// Notes are what a person writes about a photo in Unterlumen: its title,
// kept in dc:title where other programs read it too, and free fields, kept
// as ul:Fields beside the publication records (ADR-0048). The sidecar is
// where they live; a library's index only copies them.
type Notes struct {
	Title  string
	Fields map[string]string
}

// ReadNotes reads the title and the fields from the sidecar beside photoPath.
// A photo without a sidecar has no notes, which is not an error; a sidecar
// that cannot be read is, so a caller never takes "unreadable" for "empty".
func ReadNotes(photoPath string) (Notes, error) {
	data, err := os.ReadFile(SidecarPath(photoPath))
	if errors.Is(err, fs.ErrNotExist) {
		return Notes{Fields: map[string]string{}}, nil
	}
	if err != nil {
		return Notes{}, err
	}
	title, _ := parseDCTitle(data)
	return Notes{Title: title, Fields: parseSidecarFields(data)}, nil
}

// WriteField sets one field in the sidecar beside photoPath, making the
// sidecar when there is none. An empty value removes the field.
func WriteField(photoPath, key, value string) error {
	sidecarPath := SidecarPath(photoPath)
	raw, err := os.ReadFile(sidecarPath)
	if errors.Is(err, fs.ErrNotExist) {
		if value == "" {
			return nil
		}
		return os.WriteFile(sidecarPath, []byte(renderFreshXMPWith(nil, map[string]string{key: value})), 0o644)
	}
	if err != nil {
		return err
	}
	fields := parseSidecarFields(raw)
	if value == "" {
		if _, ok := fields[key]; !ok {
			return nil
		}
		delete(fields, key)
	} else {
		fields[key] = value
	}
	pubs, _ := parseSidecarPublications(raw)
	return os.WriteFile(sidecarPath, spliceULBlock(raw, pubs, fields), 0o644)
}

// parseSidecarFields extracts ul:Fields entries from XMP bytes.
func parseSidecarFields(data []byte) map[string]string {
	const rdfNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	fields := map[string]string{}
	dec := xml.NewDecoder(bytes.NewReader(data))
	var inFields, inLi bool
	var key, value, current string
	for {
		tok, err := dec.Token()
		if err == io.EOF || err != nil {
			return fields
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Space == ulNamespace && t.Name.Local == "Fields":
				inFields = true
			case inFields && t.Name.Space == rdfNS && t.Name.Local == "li":
				inLi, key, value = true, "", ""
			case inLi && t.Name.Space == ulNamespace:
				current = t.Name.Local
			}
		case xml.EndElement:
			switch {
			case t.Name.Space == ulNamespace && t.Name.Local == "Fields":
				inFields = false
			case inLi && t.Name.Space == rdfNS && t.Name.Local == "li":
				if key != "" {
					fields[key] = value
				}
				inLi = false
			case inLi && t.Name.Space == ulNamespace:
				current = ""
			}
		case xml.CharData:
			switch current {
			case "Key":
				key += string(t)
			case "Value":
				value += string(t)
			}
		}
	}
}

// renderFields produces the ul:Fields element, sorted by key so a sidecar
// changes only where a field does.
func renderFields(fields map[string]string) string {
	if len(fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var items strings.Builder
	for _, k := range keys {
		items.WriteString("\n        <rdf:li rdf:parseType=\"Resource\">")
		items.WriteString("\n          <ul:Key>" + xmlEscapeStr(k) + "</ul:Key>")
		items.WriteString("\n          <ul:Value>" + xmlEscapeStr(fields[k]) + "</ul:Value>")
		items.WriteString("\n        </rdf:li>")
	}
	return `
      <ul:Fields>
        <rdf:Bag>` + items.String() + `
        </rdf:Bag>
      </ul:Fields>`
}
