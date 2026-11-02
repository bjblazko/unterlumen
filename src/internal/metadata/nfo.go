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
// NFO file beside it, as Kodi and Jellyfin read it (ADR-0060): title, plot,
// premiered, tag, credits and userrating (0–10, two per star). Kodi keeps
// only a day in premiered, so the time taken goes into ul:datetaken beside
// it, and the rights into ul:rights; both programs ignore elements they do
// not know. Every other element of the file stays as it is.

const ulNFONS = "https://huepattl.de/unterlumen/nfo/1.0/"

const emptyNFO = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<movie xmlns:ul="` + ulNFONS + `">
</movie>
`

// nfoRoots are the root elements of a Kodi NFO file a video can have.
var nfoRoots = []string{"movie", "musicvideo", "episodedetails"}

// NFOPath is the NFO file beside a video.
func NFOPath(videoPath string) string { return media.NFOPath(videoPath) }

// nfoElements are the elements each field is kept in.
var nfoElements = map[Field]string{
	Title:       "title",
	Description: "plot",
	Creator:     "credits",
	Rights:      "ul:rights",
	Rating:      "userrating",
}

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
	var tags []string
	dec := xml.NewDecoder(bytes.NewReader(data))
	depth := 0
	var text strings.Builder
	var premiered string
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			text.Reset()
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			if depth == 2 {
				v := strings.TrimSpace(text.String())
				name := t.Name.Local
				if t.Name.Space == ulNFONS || t.Name.Space == "ul" {
					name = "ul:" + name
				}
				switch name {
				case "tag":
					if v != "" && !slices.Contains(tags, v) {
						tags = append(tags, v)
					}
				case "premiered", "aired":
					if premiered == "" {
						premiered = v
					}
				case "ul:datetaken":
					out[DateTaken] = v
				default:
					assignNFOField(out, name, v)
				}
			}
			depth--
		}
	}
	if out[DateTaken] == "" && premiered != "" {
		out[DateTaken] = premiered + "T00:00:00"
	}
	return out, tags, nil
}

func assignNFOField(out map[Field]string, name, v string) {
	for f, el := range nfoElements {
		if el != name || v == "" {
			continue
		}
		if f == Rating {
			v = stars(v)
		}
		if _, seen := out[f]; !seen && v != "" {
			out[f] = v
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
	for _, f := range []Field{Title, Description, Creator, Rights, Rating, DateTaken} {
		v, ok := set[f]
		if !ok {
			continue
		}
		if f == DateTaken {
			s = setNFOElement(s, root, "premiered", datePart(v))
			s = setNFOElement(s, root, "ul:datetaken", v)
			continue
		}
		if f == Rating && v != "" {
			n, _ := strconv.Atoi(v)
			v = strconv.Itoa(n * 2)
		}
		s = setNFOElement(s, root, nfoElements[f], v)
	}
	s = changeNFOTags(s, root, add, remove)
	if strings.Contains(s, "<ul:") {
		s = declareUL(s, root)
	}
	return []byte(s), nil
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
	for _, k := range remove {
		re := regexp.MustCompile(`\n?[ \t]*<tag>\s*` + regexp.QuoteMeta(xmlText(k)) + `\s*</tag>`)
		s = re.ReplaceAllString(s, "")
	}
	_, have, _ := parseNFO([]byte(s))
	for _, k := range add {
		if !slices.Contains(have, k) {
			s = insertBeforeClose(s, root, "\n    <tag>"+xmlText(k)+"</tag>")
			have = append(have, k)
		}
	}
	return s
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
