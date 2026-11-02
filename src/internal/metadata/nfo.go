package metadata

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"huepattl.de/unterlumen/internal/atomicfile"
	"huepattl.de/unterlumen/internal/media"
)

// A video is never changed (ADR-0058); what is written about it goes into an
// NFO file beside it, as Kodi and Jellyfin read it (ADR-0060, ADR-0061):
// title, plot, premiered, tag, credits, country, tagline, actor and
// userrating (0–10, two per star), as the field table names them. Kodi keeps
// only a day in premiered, so the time taken goes into ul:datetaken beside
// it; what Kodi has no element for goes into ul: elements, which both
// programs ignore. Every other element of the file stays as it is, and an
// actor keeps its role and picture while its name is shown.

const ulNFONS = "https://huepattl.de/unterlumen/nfo/1.0/"

const emptyNFO = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<movie xmlns:ul="` + ulNFONS + `">
</movie>
`

// nfoRoots are the root elements of a Kodi NFO file a video can have.
var nfoRoots = []string{"movie", "musicvideo", "episodedetails"}

// NFOPath is the NFO file beside a video.
func NFOPath(videoPath string) string { return media.NFOPath(videoPath) }

// readNFO reads the fields and the keywords of the NFO beside videoPath; no
// file is no fields.
func readNFO(videoPath string) (map[Field]string, []string, error) {
	data, err := os.ReadFile(NFOPath(videoPath))
	if os.IsNotExist(err) {
		return map[Field]string{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return parseNFO(data)
}

func parseNFO(data []byte) (map[Field]string, []string, error) {
	out := map[Field]string{}
	lists := map[Field][]string{}
	var tags []string
	dec := xml.NewDecoder(bytes.NewReader(data))
	depth := 0
	var text strings.Builder
	var premiered, parent string
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				parent = nfoName(t.Name)
			}
			text.Reset()
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			v := strings.TrimSpace(text.String())
			switch {
			case depth == 3:
				addNFOChild(lists, parent, nfoName(t.Name), v)
			case depth == 2:
				switch name := nfoName(t.Name); name {
				case "tag":
					tags = appendNew(tags, v)
				case "premiered", "aired":
					if premiered == "" {
						premiered = v
					}
				default:
					assignNFOField(out, lists, name, v)
				}
			}
			depth--
		}
	}
	for f, list := range lists {
		out[f] = strings.Join(list, "; ")
	}
	if out[DateTaken] == "" && premiered != "" {
		out[DateTaken] = premiered + "T00:00:00"
	}
	return out, tags, nil
}

// nfoName is an element's name as the field table writes it: ul:city.
func nfoName(n xml.Name) string {
	if n.Space == ulNFONS || n.Space == "ul" {
		return "ul:" + n.Local
	}
	return n.Local
}

func appendNew(list []string, v string) []string {
	if v == "" || slices.Contains(list, v) {
		return list
	}
	return append(list, v)
}

// addNFOChild reads the value inside an element: an actor's name.
func addNFOChild(lists map[Field][]string, parent, name, v string) {
	for _, spec := range table {
		if spec.nfo == parent && spec.nfoChild == name {
			lists[spec.field] = appendNew(lists[spec.field], v)
		}
	}
}

func assignNFOField(out map[Field]string, lists map[Field][]string, name, v string) {
	for _, spec := range table {
		if spec.nfo != name || spec.nfoChild != "" || v == "" {
			continue
		}
		switch {
		case spec.prop.IsList():
			lists[spec.field] = appendNew(lists[spec.field], v)
		case spec.field == Rating:
			v = stars(v)
		}
		if _, seen := out[spec.field]; !seen && v != "" && !spec.prop.IsList() {
			out[spec.field] = v
		}
	}
}

// stars turns Kodi's 0–10 into 0–5.
func stars(v string) string {
	n, err := strconv.Atoi(v)
	if err != nil {
		return ""
	}
	return strconv.Itoa((n + 1) / 2)
}

// writeNFO applies the change to the NFO beside videoPath, made when there
// is none. Without the video nothing is written.
func writeNFO(videoPath string, set map[Field]string, add, remove []string) error {
	if info, err := os.Stat(videoPath); err != nil || info.IsDir() {
		return fmt.Errorf("%s: %w; scan the library to find it again", filepath.Base(videoPath), media.ErrPhotoGone)
	}
	return atomicfile.Change(NFOPath(videoPath), func(data []byte) ([]byte, error) {
		if data == nil && allEmpty(set) && len(add) == 0 {
			return nil, nil
		}
		return withNFO(data, set, add, remove)
	})
}

func withNFO(data []byte, set map[Field]string, add, remove []string) ([]byte, error) {
	s := string(data)
	if len(bytes.TrimSpace(data)) == 0 {
		s = emptyNFO
	}
	root := nfoRoot(s)
	if root == "" {
		return nil, fmt.Errorf("the NFO file beside the video is not one Kodi reads, so it was left as it is")
	}
	for _, spec := range table {
		if v, ok := set[spec.field]; ok && spec.nfo != "" {
			s = setNFOField(s, root, spec, v)
		}
	}
	s = changeNFOTags(s, root, add, remove)
	if strings.Contains(s, "<ul:") {
		s = declareUL(s, root)
	}
	return []byte(s), nil
}

func setNFOField(s, root string, spec fieldSpec, v string) string {
	switch {
	case spec.field == DateTaken:
		s = setNFOElement(s, root, "premiered", datePart(v))
		return setNFOElement(s, root, spec.nfo, v)
	case spec.field == Rating && v != "":
		n, _ := strconv.Atoi(v)
		return setNFOElement(s, root, spec.nfo, strconv.Itoa(n*2))
	case spec.prop.IsList():
		return setNFOList(s, root, spec.nfo, spec.nfoChild, media.SplitList(v))
	}
	return setNFOElement(s, root, spec.nfo, v)
}

func nfoRoot(s string) string {
	for _, r := range nfoRoots {
		if strings.Contains(s, "</"+r+">") {
			return r
		}
	}
	return ""
}

func elementRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)\n?[ \t]*<` + regexp.QuoteMeta(name) + `(?:\s[^>]*)?(?:/>|>.*?</` + regexp.QuoteMeta(name) + `>)`)
}

// setNFOElement puts one element name with v in place of every one there
// is; an empty v removes them.
func setNFOElement(s, root, name, v string) string {
	re := elementRe(name)
	first := re.FindStringIndex(s)
	el := ""
	if v != "" {
		el = "\n    <" + name + ">" + xmlText(v) + "</" + name + ">"
	}
	if first == nil {
		return insertBeforeClose(s, root, el)
	}
	head, tail := s[:first[0]], re.ReplaceAllString(s[first[1]:], "")
	return head + el + tail
}

func changeNFOTags(s, root string, add, remove []string) string {
	if len(add) == 0 && len(remove) == 0 {
		return s
	}
	_, have, _ := parseNFO([]byte(s))
	want := slices.DeleteFunc(have, func(k string) bool { return slices.Contains(remove, k) })
	for _, k := range add {
		want = appendNew(want, k)
	}
	return setNFOList(s, root, "tag", "", want)
}

// setNFOList leaves one element name for each entry of want: those there
// stay as they are (an actor with its role), the others go, the missing are
// added. child is the element inside that holds the entry, or "".
func setNFOList(s, root, name, child string, want []string) string {
	re := elementRe(name)
	var have []string
	for _, m := range slices.Backward(re.FindAllStringIndex(s, -1)) {
		entry := nfoEntry(s[m[0]:m[1]], child)
		if !slices.Contains(want, entry) || slices.Contains(have, entry) {
			s = s[:m[0]] + s[m[1]:]
			continue
		}
		have = append(have, entry)
	}
	for _, e := range want {
		if slices.Contains(have, e) {
			continue
		}
		el := "<" + name + ">" + xmlText(e) + "</" + name + ">"
		if child != "" {
			el = "<" + name + "><" + child + ">" + xmlText(e) + "</" + child + "></" + name + ">"
		}
		s = insertBeforeClose(s, root, "\n    "+el)
	}
	return s
}

// nfoEntry is the text of one element, or of its child.
func nfoEntry(el, child string) string {
	if child != "" {
		m := regexp.MustCompile(`(?s)<` + regexp.QuoteMeta(child) + `>(.*?)</` + regexp.QuoteMeta(child) + `>`).FindStringSubmatch(el)
		if m == nil {
			return ""
		}
		el = m[1]
	} else {
		el = el[strings.Index(el, ">")+1:]
		if i := strings.LastIndex(el, "</"); i >= 0 {
			el = el[:i]
		}
	}
	var b strings.Builder
	dec := xml.NewDecoder(strings.NewReader("<x>" + el + "</x>"))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if cd, ok := tok.(xml.CharData); ok {
			b.Write(cd)
		}
	}
	return strings.TrimSpace(b.String())
}

func insertBeforeClose(s, root, el string) string {
	if el == "" {
		return s
	}
	at := strings.LastIndex(s, "</"+root+">")
	head := strings.TrimRight(s[:at], " \t\n")
	return head + el + "\n" + s[at:]
}

// declareUL adds the ul namespace to the root element when it lacks it.
func declareUL(s, root string) string {
	start := strings.Index(s, "<"+root)
	end := strings.Index(s[start:], ">")
	if start < 0 || end < 0 || strings.Contains(s[start:start+end], "xmlns:ul=") {
		return s
	}
	at := start + end
	if s[at-1] == '/' {
		at--
	}
	return s[:at] + ` xmlns:ul="` + ulNFONS + `"` + s[at:]
}

func xmlText(v string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(v)) //nolint:errcheck
	return b.String()
}

func datePart(iso string) string {
	if len(iso) >= 10 {
		return iso[:10]
	}
	return iso
}

func allEmpty(set map[Field]string) bool {
	for _, v := range set {
		if v != "" {
			return false
		}
	}
	return true
}
