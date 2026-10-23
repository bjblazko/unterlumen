package media

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// Things found in a photo go into the sidecar twice (ADR-0054): as keywords,
// dc:subject, which Lightroom, digiKam and others read, and as a record in
// Unterlumen's own description that says which keywords Unterlumen wrote —
// so the owner's own keywords are never taken for found things nor removed —
// which things the owner took away, so no later pass brings them back, and
// which models and word list looked.

// ThingsRecord is Unterlumen's record of the things in a photo.
type ThingsRecord struct {
	Found   []string // written as keywords by Unterlumen
	Not     []string // taken away by the owner
	Version string   // the models and the word list that looked, "" for none
}

func (r ThingsRecord) empty() bool {
	return len(r.Found) == 0 && len(r.Not) == 0 && r.Version == ""
}

// SidecarThings is what a sidecar says about the things in a photo.
type SidecarThings struct {
	Record   ThingsRecord
	Keywords []string // every keyword, from any program
}

// Things are the things the sidecar says the photo shows: those Unterlumen
// wrote whose keyword is still there. A keyword taken away in another
// program takes the thing with it.
func (s SidecarThings) Things() []string {
	var out []string
	for _, t := range s.Record.Found {
		if slices.Contains(s.Keywords, t) && !slices.Contains(s.Record.Not, t) {
			out = append(out, t)
		}
	}
	return out
}

// ReadThings reads the things record and the keywords from the sidecar
// beside photoPath. No sidecar is no things, not an error.
func ReadThings(photoPath string) (SidecarThings, error) {
	data, err := os.ReadFile(SidecarPath(photoPath))
	if errors.Is(err, fs.ErrNotExist) {
		return SidecarThings{}, nil
	}
	if err != nil {
		return SidecarThings{}, err
	}
	return SidecarThings{Record: parseThingsRecord(data), Keywords: parseSubjects(data)}, nil
}

// WriteFoundThings records what a pass of version found in the photo, as the
// sidecar is now: things the owner took away stay away, keywords Unterlumen
// wrote before and no longer finds are removed, every other keyword stays.
// It writes nothing for a photo without a sidecar and without things, and
// reports whether the sidecar now holds the record.
func WriteFoundThings(photoPath string, found []string, version string) (bool, error) {
	written := false
	err := changeSidecar(photoPath, func(data []byte) ([]byte, error) {
		written = false
		if data == nil && len(found) == 0 {
			return nil, nil
		}
		written = true
		c := readULContent(data)
		keep := slices.DeleteFunc(slices.Clone(found), func(t string) bool { return slices.Contains(c.things.Not, t) })
		var gone []string
		for _, t := range c.things.Found {
			if !slices.Contains(keep, t) {
				gone = append(gone, t)
			}
		}
		c.things.Found, c.things.Version = keep, version
		return withThings(data, c, keep, gone), nil
	})
	return written && err == nil, err
}

// RemoveThing takes a thing away from the photo for good: its keyword goes,
// and the record keeps it from being found again.
func RemoveThing(photoPath, thing string) error {
	return changeSidecar(photoPath, func(data []byte) ([]byte, error) {
		c := readULContent(data)
		c.things.Found = slices.DeleteFunc(c.things.Found, func(t string) bool { return t == thing })
		if !slices.Contains(c.things.Not, thing) {
			c.things.Not = append(c.things.Not, thing)
		}
		return withThings(data, c, nil, []string{thing}), nil
	})
}

// withThings writes the keywords and Unterlumen's description with c.
func withThings(data []byte, c ulContent, add, remove []string) []byte {
	out := withSubjects(data, add, remove)
	return spliceULBlock(out, c)
}

func parseThingsRecord(data []byte) ThingsRecord {
	var r ThingsRecord
	dec := xml.NewDecoder(bytes.NewReader(data))
	var list *[]string
	var inLi, inVersion bool
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF || err != nil {
			return r
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Space == ulNamespace && t.Name.Local == "Things":
				list = &r.Found
			case t.Name.Space == ulNamespace && t.Name.Local == "ThingsNot":
				list = &r.Not
			case t.Name.Space == ulNamespace && t.Name.Local == "ThingsVersion":
				inVersion = true
				text.Reset()
			case list != nil && t.Name.Space == rdfNS && t.Name.Local == "li":
				inLi = true
				text.Reset()
			}
		case xml.EndElement:
			switch {
			case t.Name.Space == ulNamespace && (t.Name.Local == "Things" || t.Name.Local == "ThingsNot"):
				list = nil
			case t.Name.Space == ulNamespace && t.Name.Local == "ThingsVersion":
				inVersion = false
				r.Version = strings.TrimSpace(text.String())
			case inLi && t.Name.Space == rdfNS && t.Name.Local == "li":
				inLi = false
				if v := strings.TrimSpace(text.String()); v != "" {
					*list = append(*list, v)
				}
			}
		case xml.CharData:
			if inLi || inVersion {
				text.Write(t)
			}
		}
	}
}

func renderThingsRecord(r ThingsRecord) string {
	bag := func(name string, items []string) string {
		if len(items) == 0 {
			return ""
		}
		var b strings.Builder
		for _, it := range items {
			b.WriteString("\n          <rdf:li>" + xmlEscapeStr(it) + "</rdf:li>")
		}
		return "\n      <ul:" + name + ">\n        <rdf:Bag>" + b.String() + "\n        </rdf:Bag>\n      </ul:" + name + ">"
	}
	out := bag("Things", r.Found) + bag("ThingsNot", r.Not)
	if r.Version != "" {
		out += "\n      <ul:ThingsVersion>" + xmlEscapeStr(r.Version) + "</ul:ThingsVersion>"
	}
	return out
}
