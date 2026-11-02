package media

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// The descriptive properties the metadata editor writes into a sidecar
// besides the title and the keywords (ADR-0060, ADR-0061): when the photo was
// taken, its description, creator, rights, rating and the IPTC fields, under
// the names Lightroom, digiKam and darktable read. Which properties there are
// is the metadata package's field table; this file only knows how a property
// is kept. A property another program already keeps — as an element or as an
// attribute of its description — is changed where it is; any other goes into
// a description of Unterlumen's own, which holds nothing else.

// A Prop is one property of a sidecar.
type Prop struct {
	prefix, ns, local string
	kind              PropKind
}

// PropKind is how a property holds its value.
type PropKind int

const (
	PropSimple PropKind = iota // text in the element or an attribute
	PropAlt                    // rdf:Alt with one x-default rdf:li
	PropSeq                    // rdf:Seq, one rdf:li per entry, read joined by "; "
	PropBag                    // rdf:Bag, like PropSeq without an order
)

// xmpNamespaces are the namespaces a Prop can be in, by their usual prefix,
// in the order Unterlumen's own description declares them.
var xmpNamespaces = []struct{ prefix, ns string }{
	{"dc", dcNS},
	{"exif", "http://ns.adobe.com/exif/1.0/"},
	{"photoshop", "http://ns.adobe.com/photoshop/1.0/"},
	{"xmp", "http://ns.adobe.com/xap/1.0/"},
	{"Iptc4xmpCore", "http://iptc.org/std/Iptc4xmpCore/1.0/xmlns/"},
	{"Iptc4xmpExt", "http://iptc.org/std/Iptc4xmpExt/2008-02-29/"},
	{"xmpRights", "http://ns.adobe.com/xap/1.0/rights/"},
}

// NewProp is the property local in the namespace of prefix, one of
// xmpNamespaces; any other prefix is a mistake in the field table.
func NewProp(prefix, local string, kind PropKind) Prop {
	for _, n := range xmpNamespaces {
		if n.prefix == prefix {
			return Prop{prefix, n.ns, local, kind}
		}
	}
	panic("media: no XMP namespace for prefix " + prefix)
}

// PropRating is the one property media reads itself, from a file's own XMP.
var PropRating = NewProp("xmp", "Rating", PropSimple)

// IsList says whether a property holds several entries.
func (p Prop) IsList() bool { return p.kind == PropSeq || p.kind == PropBag }

// ownPropsStart opens Unterlumen's own description. The one before the IPTC
// fields declared fewer namespaces and is still found (legacyOwnPropsStart).
var ownPropsStart = func() string {
	var b strings.Builder
	b.WriteString(`<rdf:Description rdf:about=""`)
	for _, n := range xmpNamespaces {
		b.WriteString(` xmlns:` + n.prefix + `="` + n.ns + `"`)
	}
	return b.String() + ">"
}()

const legacyOwnPropsStart = `<rdf:Description rdf:about="" xmlns:dc="` + dcNS +
	`" xmlns:exif="http://ns.adobe.com/exif/1.0/" xmlns:photoshop="http://ns.adobe.com/photoshop/1.0/" xmlns:xmp="http://ns.adobe.com/xap/1.0/">`

func (p Prop) qname() string { return p.prefix + ":" + p.local }

// ReadSidecarProps reads props from the sidecar beside photoPath, wherever a
// program keeps them. No sidecar is no properties, not an error.
func ReadSidecarProps(photoPath string, props []Prop) (map[Prop]string, error) {
	data, err := os.ReadFile(SidecarPath(photoPath))
	if errors.Is(err, fs.ErrNotExist) {
		return map[Prop]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return parseProps(data, props), nil
}

// ReadSidecarKeywords reads the keywords from the sidecar beside photoPath.
func ReadSidecarKeywords(photoPath string) ([]string, error) {
	data, err := os.ReadFile(SidecarPath(photoPath))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseSubjects(data), nil
}

// WriteSidecarProps sets the properties in set in the sidecar beside
// photoPath; an empty value removes one. props are all the properties there
// are, in the order of Unterlumen's own description, which keeps the others.
// Without a sidecar and with nothing to set, nothing is written.
func WriteSidecarProps(photoPath string, props []Prop, set map[Prop]string) error {
	return changeSidecar(photoPath, func(data []byte) ([]byte, error) {
		if data == nil && allEmpty(set) {
			return nil, nil
		}
		return withProps(data, props, set), nil
	})
}

// WriteKeywords adds and removes keywords in the sidecar beside photoPath,
// where they are kept; every other keyword stays.
func WriteKeywords(photoPath string, add, remove []string) error {
	return changeSidecar(photoPath, func(data []byte) ([]byte, error) {
		if data == nil && len(add) == 0 {
			return nil, nil
		}
		return withSubjects(data, add, remove), nil
	})
}

func allEmpty(set map[Prop]string) bool {
	for _, v := range set {
		if v != "" {
			return false
		}
	}
	return true
}

// withProps changes each property where another program keeps it, and puts
// the rest into Unterlumen's own description.
func withProps(data []byte, props []Prop, set map[Prop]string) []byte {
	s := ensureXMP(data)
	own := ownPropsSpan(s)
	values := parseProps([]byte(own.text(s)), props)
	for _, p := range props {
		v, ok := set[p]
		if !ok {
			continue
		}
		if changed, done := changeForeign(s, own, p, v); done {
			s = changed
			own = ownPropsSpan(s)
			delete(values, p)
			continue
		}
		values[p] = v
	}
	return []byte(replaceOwnProps(s, own, props, values))
}

// changeForeign changes p where another program keeps it; done is false
// when no other program keeps it.
func changeForeign(s string, own span, p Prop, v string) (string, bool) {
	if start, end := foreignElement(s, p.qname(), own); start >= 0 {
		if v == "" {
			return strings.TrimRight(s[:start], " \t") + strings.TrimLeft(s[end:], " \t\n"), true
		}
		return s[:start] + renderProp(p, v) + s[end:], true
	}
	if start, end, valStart, valEnd := foreignAttr(s, p, own); start >= 0 {
		if v == "" {
			return s[:start] + s[end:], true
		}
		return s[:valStart] + xmlEscapeAttr(v) + s[valEnd:], true
	}
	return s, false
}

// foreignAttr finds p as an attribute of a description outside own: the
// whole attribute with its leading space, and its value.
func foreignAttr(s string, p Prop, own span) (int, int, int, int) {
	if p.kind != PropSimple {
		return -1, -1, -1, -1
	}
	re := regexp.MustCompile(`\s` + regexp.QuoteMeta(p.qname()) + `="([^"]*)"`)
	for _, m := range re.FindAllStringSubmatchIndex(s, -1) {
		if own.start >= 0 && m[0] >= own.start && m[0] < own.end {
			continue
		}
		return m[0], m[1], m[2], m[3]
	}
	return -1, -1, -1, -1
}

func ownPropsSpan(s string) span {
	start := strings.Index(s, ownPropsStart)
	if start < 0 {
		start = strings.Index(s, legacyOwnPropsStart)
	}
	if start < 0 {
		return span{-1, -1}
	}
	end := strings.Index(s[start:], descEnd)
	if end < 0 {
		return span{-1, -1}
	}
	return span{start, start + end + len(descEnd)}
}

func (o span) text(s string) string {
	if o.start < 0 {
		return ""
	}
	return s[o.start:o.end]
}

// replaceOwnProps writes values as Unterlumen's own description, in place
// of the one there is, before </rdf:RDF>, or not at all when it is empty.
func replaceOwnProps(s string, own span, props []Prop, values map[Prop]string) string {
	var body strings.Builder
	for _, p := range props {
		if v := values[p]; v != "" {
			body.WriteString("\n      " + renderProp(p, v))
		}
	}
	block := ""
	if body.Len() > 0 {
		block = "    " + ownPropsStart + body.String() + "\n    " + descEnd
	}
	if own.start >= 0 {
		if block == "" {
			return strings.TrimRight(s[:own.start], " \t") + strings.TrimLeft(s[own.end:], "\n")
		}
		return s[:own.start] + strings.TrimLeft(block, " ") + s[own.end:]
	}
	if block == "" {
		return s
	}
	at := strings.LastIndex(s, "</rdf:RDF>")
	return s[:at] + "  " + strings.TrimLeft(block, " ") + "\n  " + s[at:]
}

func renderProp(p Prop, v string) string {
	q := p.qname()
	switch p.kind {
	case PropAlt:
		return "<" + q + "><rdf:Alt><rdf:li xml:lang=\"x-default\">" + xmlEscapeStr(v) + "</rdf:li></rdf:Alt></" + q + ">"
	case PropSeq, PropBag:
		list := "rdf:Seq"
		if p.kind == PropBag {
			list = "rdf:Bag"
		}
		var items strings.Builder
		for _, e := range SplitList(v) {
			items.WriteString("<rdf:li>" + xmlEscapeStr(e) + "</rdf:li>")
		}
		return "<" + q + "><" + list + ">" + items.String() + "</" + list + "></" + q + ">"
	}
	return "<" + q + ">" + xmlEscapeStr(v) + "</" + q + ">"
}

// SplitList splits "Ann; Bob", a list property's value, into its entries.
func SplitList(v string) []string {
	var out []string
	for _, e := range strings.Split(v, ";") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

func xmlEscapeAttr(s string) string {
	return strings.ReplaceAll(xmlEscapeStr(s), `"`, "&quot;")
}

// parseProps reads props in data, from attributes and elements of any
// description; the first one found counts.
func parseProps(data []byte, props []Prop) map[Prop]string {
	out := map[Prop]string{}
	if len(data) == 0 {
		return out
	}
	dec := xml.NewDecoder(bytes.NewReader(wrapIfFragment(data)))
	var cur *Prop
	var items []string
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space == rdfNS && t.Name.Local == "Description" {
				readPropAttrs(t.Attr, props, out)
				continue
			}
			if p, ok := propNamed(props, t.Name); ok && cur == nil {
				cur, items = &p, nil
				text.Reset()
				continue
			}
			if cur != nil && t.Name.Space == rdfNS && t.Name.Local == "li" {
				text.Reset()
			}
		case xml.CharData:
			if cur != nil {
				text.Write(t)
			}
		case xml.EndElement:
			if cur == nil {
				continue
			}
			if t.Name.Space == rdfNS && t.Name.Local == "li" {
				if v := strings.TrimSpace(text.String()); v != "" {
					items = append(items, v)
				}
				text.Reset()
				continue
			}
			if t.Name.Space == cur.ns && t.Name.Local == cur.local {
				v := strings.TrimSpace(text.String())
				if cur.kind != PropSimple {
					v = strings.Join(items, "; ")
				}
				if _, seen := out[*cur]; !seen && v != "" {
					out[*cur] = v
				}
				cur = nil
			}
		}
	}
}

// wrapIfFragment lets parseProps read one description cut from a sidecar.
func wrapIfFragment(data []byte) []byte {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("<rdf:Description")) {
		return []byte(`<x xmlns:rdf="` + rdfNS + `">` + string(data) + `</x>`)
	}
	return data
}

func readPropAttrs(attrs []xml.Attr, props []Prop, out map[Prop]string) {
	for _, a := range attrs {
		if p, ok := propNamed(props, a.Name); ok && p.kind == PropSimple {
			if _, seen := out[p]; !seen && strings.TrimSpace(a.Value) != "" {
				out[p] = strings.TrimSpace(a.Value)
			}
		}
	}
}

func propNamed(props []Prop, n xml.Name) (Prop, bool) {
	for _, p := range props {
		if n.Space == p.ns && n.Local == p.local {
			return p, true
		}
	}
	return Prop{}, false
}

// xmpHead is how far into a file its own XMP packet is looked for: a JPEG
// keeps it in an APP1 segment near the start.
const xmpHead = 256 << 10

// EmbeddedProps reads props from the XMP packet inside a photo, written by a
// camera, Lightroom or exiftool; nothing when there is none near its start.
func EmbeddedProps(f io.ReaderAt, props []Prop) map[Prop]string {
	buf := make([]byte, xmpHead)
	n, _ := f.ReadAt(buf, 0)
	buf = buf[:n]
	start := bytes.Index(buf, []byte("<x:xmpmeta"))
	if start < 0 {
		return map[Prop]string{}
	}
	end := bytes.Index(buf[start:], []byte("</x:xmpmeta>"))
	if end < 0 {
		return map[Prop]string{}
	}
	return parseProps(buf[start:start+end+len("</x:xmpmeta>")], props)
}

// embeddedRating is the rating in the XMP packet inside a photo.
func embeddedRating(f io.ReaderAt) string {
	return EmbeddedProps(f, []Prop{PropRating})[PropRating]
}

// StarsOf reads a rating as stars, 1–5; 0 for none, rejected (-1) or
// anything else.
func StarsOf(rating string) int {
	n, err := strconv.Atoi(strings.TrimSpace(rating))
	if err != nil || n < 1 || n > 5 {
		return 0
	}
	return n
}
