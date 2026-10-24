package media

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"slices"
	"strings"
)

// Dublin Core in a sidecar: the title (ADR-0048) and the keywords, dc:subject,
// where things found in a photo are written for other programs to read
// (ADR-0054). Unterlumen keeps them in an rdf:Description of its own, told
// from another program's by its whole opening tag. A title or keywords that
// another program keeps in its own description — Lightroom keeps everything
// in one — are changed there, element by element, and the rest of that
// description is left as it is.

const (
	dcNS  = "http://purl.org/dc/elements/1.1/"
	rdfNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"

	ownDCStart = `<rdf:Description rdf:about="" xmlns:dc="` + dcNS + `">`
	descEnd    = "</rdf:Description>"
)

func ReadTitle(photoPath string) (string, error) {
	data, err := os.ReadFile(SidecarPath(photoPath))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return parseDCTitle(data)
}

// WriteTitle sets the title in the sidecar beside photoPath; an empty title
// removes it.
func WriteTitle(photoPath, title string) error {
	return changeSidecar(photoPath, func(data []byte) ([]byte, error) {
		if data == nil && title == "" {
			return nil, nil
		}
		return withTitle(data, title), nil
	})
}

// withTitle sets the title where it is kept: in another program's element
// when there is one, else in Unterlumen's own description.
func withTitle(data []byte, title string) []byte {
	s := ensureXMP(data)
	own := ownDC(s)
	if start, end := foreignElement(s, "dc:title", own); start >= 0 {
		s = s[:start] + renderTitleElement(title, "") + s[end:]
		own = ownDC(s)
		c := own.content(s)
		c.title = ""
		return []byte(own.replace(s, c))
	}
	c := own.content(s)
	c.title = title
	return []byte(own.replace(s, c))
}

// Subjects lists the keywords in the sidecar data, from any program.
func Subjects(data []byte) []string {
	return parseSubjects(data)
}

// withSubjects removes the keywords in remove and adds those in add, where
// the keywords are kept: in another program's element when there is one, else
// in Unterlumen's own description. Keywords of Unterlumen's own description
// move into another program's element when one has appeared, so a sidecar
// never holds two lists.
func withSubjects(data []byte, add, remove []string) []byte {
	s := ensureXMP(data)
	own := ownDC(s)
	c := own.content(s)
	keep := func(list []string) []string {
		out := slices.DeleteFunc(slices.Clone(list), func(k string) bool { return slices.Contains(remove, k) })
		for _, k := range add {
			if !slices.Contains(out, k) {
				out = append(out, k)
			}
		}
		return out
	}
	if start, end := foreignElement(s, "dc:subject", own); start >= 0 {
		list := keep(append(parseSubjects([]byte(wrapFragment(s[start:end]))), c.subjects...))
		c.subjects = nil
		// In place of the element, after the indentation that stands before it.
		s = s[:start] + strings.TrimLeft(renderSubjectElement(list, ""), " ") + s[end:]
		return []byte(ownDC(s).replace(s, c))
	}
	c.subjects = keep(c.subjects)
	return []byte(own.replace(s, c))
}

// dcContent is what Unterlumen's own description holds.
type dcContent struct {
	title    string
	subjects []string
}

// span is where Unterlumen's own description is in a sidecar; start is -1
// when there is none.
type span struct{ start, end int }

func ownDC(s string) span {
	start := strings.Index(s, ownDCStart)
	if start < 0 {
		return span{-1, -1}
	}
	end := strings.Index(s[start:], descEnd)
	if end < 0 {
		return span{-1, -1}
	}
	return span{start, start + end + len(descEnd)}
}

func (o span) content(s string) dcContent {
	if o.start < 0 {
		return dcContent{}
	}
	frag := []byte(wrapFragment(s[o.start:o.end]))
	title, _ := parseDCTitle(frag)
	return dcContent{title: title, subjects: parseSubjects(frag)}
}

// replace puts c in place of the own description, adds one before
// </rdf:RDF>, or removes it when c is empty.
func (o span) replace(s string, c dcContent) string {
	block := renderOwnDC(c)
	if o.start >= 0 {
		if block == "" {
			return strings.TrimRight(s[:o.start], " \t") + strings.TrimLeft(s[o.end:], "\n")
		}
		return s[:o.start] + strings.TrimLeft(block, " ") + s[o.end:]
	}
	if block == "" {
		return s
	}
	at := strings.LastIndex(s, "</rdf:RDF>")
	return s[:at] + "  " + strings.TrimLeft(block, " ") + "\n  " + s[at:]
}

func renderOwnDC(c dcContent) string {
	if c.title == "" && len(c.subjects) == 0 {
		return ""
	}
	return "    " + ownDCStart + renderTitleElement(c.title, "\n      ") +
		renderSubjectElement(c.subjects, "\n      ") + "\n    " + descEnd
}

// renderTitleElement is dc:title, or nothing for no title.
func renderTitleElement(title, indent string) string {
	if title == "" {
		return ""
	}
	return indent + `<dc:title>
        <rdf:Alt>
          <rdf:li xml:lang="x-default">` + xmlEscapeStr(title) + `</rdf:li>
        </rdf:Alt>
      </dc:title>`
}

// renderSubjectElement is dc:subject, or nothing for no keywords.
func renderSubjectElement(subjects []string, indent string) string {
	if len(subjects) == 0 {
		return ""
	}
	var items strings.Builder
	for _, k := range subjects {
		items.WriteString("\n          <rdf:li>" + xmlEscapeStr(k) + "</rdf:li>")
	}
	return indent + "<dc:subject>\n        <rdf:Bag>" + items.String() + "\n        </rdf:Bag>\n      </dc:subject>"
}

// foreignElement finds the element named tag (with its usual prefix) outside
// Unterlumen's own description: where it starts and ends, or -1.
func foreignElement(s, tag string, own span) (int, int) {
	for from := 0; ; {
		i := strings.Index(s[from:], "<"+tag)
		if i < 0 {
			return -1, -1
		}
		start := from + i
		after := start + len(tag) + 1
		if after < len(s) && s[after] != '>' && s[after] != ' ' && s[after] != '\n' && s[after] != '\t' {
			from = after
			continue
		}
		if own.start >= 0 && start >= own.start && start < own.end {
			from = own.end
			continue
		}
		closing := strings.Index(s[start:], "</"+tag+">")
		if closing < 0 {
			if selfClosed := strings.Index(s[start:], "/>"); selfClosed >= 0 {
				return start, start + selfClosed + 2
			}
			return -1, -1
		}
		return start, start + closing + len("</"+tag+">")
	}
}

// wrapFragment declares the namespaces a fragment cut from a sidecar uses,
// so it can be parsed by itself.
func wrapFragment(frag string) string {
	return `<x xmlns:rdf="` + rdfNS + `" xmlns:dc="` + dcNS + `">` + frag + `</x>`
}

// ensureXMP is data, or an empty sidecar when there is none.
func ensureXMP(data []byte) string {
	if len(data) == 0 || !bytes.Contains(data, []byte("</rdf:RDF>")) {
		return emptyXMP
	}
	return string(data)
}

func parseDCTitle(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var inDCTitle, inAltLi bool
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", nil
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Space == dcNS && t.Name.Local == "title":
				inDCTitle = true
			case inDCTitle && t.Name.Space == rdfNS && t.Name.Local == "li":
				inAltLi = true
			}
		case xml.EndElement:
			switch {
			case t.Name.Space == dcNS && t.Name.Local == "title":
				inDCTitle = false
			case inDCTitle && t.Name.Space == rdfNS && t.Name.Local == "li":
				inAltLi = false
			}
		case xml.CharData:
			if inAltLi {
				return strings.TrimSpace(string(t)), nil
			}
		}
	}
}

// parseSubjects reads the keywords of every dc:subject in data.
func parseSubjects(data []byte) []string {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var out []string
	var inSubject, inLi bool
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF || err != nil {
			return out
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Space == dcNS && t.Name.Local == "subject":
				inSubject = true
			case inSubject && t.Name.Space == rdfNS && t.Name.Local == "li":
				inLi = true
				text.Reset()
			}
		case xml.EndElement:
			switch {
			case t.Name.Space == dcNS && t.Name.Local == "subject":
				inSubject = false
			case inLi && t.Name.Space == rdfNS && t.Name.Local == "li":
				inLi = false
				if k := strings.TrimSpace(text.String()); k != "" && !slices.Contains(out, k) {
					out = append(out, k)
				}
			}
		case xml.CharData:
			if inLi {
				text.Write(t)
			}
		}
	}
}
