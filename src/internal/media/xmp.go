package media

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ulNamespace = "https://unterlumen.app/xmp/1.0/"

// Publication records one publish event for a photo.
type Publication struct {
	Channel      string
	Account      string // account ID within the channel; empty when channel has no sub-accounts
	PostID       string // shared ID for photos published together in one action
	GalleryTitle string // title of the gallery/album this publish belongs to; empty for bare exports
	Slug         string // site album's folder name and URL; empty for anything that is not a site album. Never derived again once published.
	Unlisted     bool   // site album is kept out of the index and sitemap; fixed with the slug
	PublishedAt  time.Time
}

// SidecarPath returns the XMP sidecar path for the given photo file.
func SidecarPath(photoPath string) string {
	ext := filepath.Ext(photoPath)
	return photoPath[:len(photoPath)-len(ext)] + ".xmp"
}

// CarrySidecar applies op to a photo's XMP sidecar so the sidecar travels with
// the photo it belongs to. The sidecar holds the publication record (ADR-0027)
// — which album on which channel a photo was published in — and that record is
// what makes a published site rebuildable from the photos themselves. A photo
// that is renamed or moved without it loses that history silently, and leaves
// an orphaned .xmp behind.
//
// It is a no-op when there is no sidecar, and when srcPhoto is a sidecar
// itself. Failures are the caller's to report: the photo has already moved,
// so the operation is not undone for the sake of its metadata.
func CarrySidecar(op func(src, dst string) error, srcPhoto, dstPhoto string) error {
	src := SidecarPath(srcPhoto)
	if src == srcPhoto {
		return nil
	}
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	return op(src, SidecarPath(dstPhoto))
}

// ReadSidecar reads publication records from the XMP sidecar alongside photoPath.
// Returns empty slice (not error) if the sidecar does not exist.
func ReadSidecar(photoPath string) ([]Publication, error) {
	data, err := os.ReadFile(SidecarPath(photoPath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseSidecarPublications(data)
}

// AppendPublication adds a publication record to the XMP sidecar alongside photoPath.
// Creates the sidecar if it does not exist. Existing XMP namespaces are preserved.
func AppendPublication(photoPath string, pub Publication) error {
	return changeSidecar(photoPath, func(raw []byte) ([]byte, error) {
		if raw == nil {
			return []byte(renderFreshXMP([]Publication{pub})), nil
		}
		pubs, err := parseSidecarPublications(raw)
		if err != nil {
			return nil, fmt.Errorf("read sidecar: %w", err)
		}
		return mergeULBlock(raw, append(pubs, pub)), nil
	})
}

// RemovePublication drops the record of one album (channel and postID) from the
// photo's sidecar and leaves everything else in it alone. A photo without a
// sidecar, or one that never was in that album, is not an error.
func RemovePublication(photoPath, channel, postID string) error {
	return changeSidecar(photoPath, func(raw []byte) ([]byte, error) {
		if raw == nil {
			return nil, nil
		}
		pubs, err := parseSidecarPublications(raw)
		if err != nil {
			return nil, fmt.Errorf("read sidecar: %w", err)
		}
		kept := make([]Publication, 0, len(pubs))
		for _, p := range pubs {
			if p.Channel == channel && p.PostID == postID {
				continue
			}
			kept = append(kept, p)
		}
		if len(kept) == len(pubs) {
			return nil, nil
		}
		return mergeULBlock(raw, kept), nil
	})
}

// SetPublicationAddress records a site album's slug and Unlisted flag in the
// publications of one album (channel and postID) that carry none — the ones
// written before the address was recorded. An address that is already there is
// never replaced: a slug is a URL that may be shared. It reports whether it
// changed the sidecar.
func SetPublicationAddress(photoPath, channel, postID, slug string, unlisted bool) (bool, error) {
	changed := false
	err := changeSidecar(photoPath, func(raw []byte) ([]byte, error) {
		changed = false
		if raw == nil {
			return nil, nil
		}
		pubs, err := parseSidecarPublications(raw)
		if err != nil {
			return nil, fmt.Errorf("read sidecar: %w", err)
		}
		for i := range pubs {
			if pubs[i].Channel == channel && pubs[i].PostID == postID && pubs[i].Slug == "" {
				pubs[i].Slug, pubs[i].Unlisted = slug, unlisted
				changed = true
			}
		}
		if !changed {
			return nil, nil
		}
		return mergeULBlock(raw, pubs), nil
	})
	return changed && err == nil, err
}

// parseSidecarPublications extracts ul:Publications entries from XMP bytes.
func parseSidecarPublications(data []byte) ([]Publication, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))

	const rdfNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	var pubs []Publication
	var inULPublications bool
	var inRDFLi bool
	var current Publication
	var currentField string

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break // non-fatal: return what we parsed so far
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Space == ulNamespace && t.Name.Local == "Publications":
				inULPublications = true
			case inULPublications && t.Name.Space == rdfNS && t.Name.Local == "li":
				inRDFLi = true
				current = Publication{}
			case inRDFLi && t.Name.Space == ulNamespace:
				currentField = t.Name.Local
			}
		case xml.EndElement:
			switch {
			case t.Name.Space == ulNamespace && t.Name.Local == "Publications":
				inULPublications = false
			case inULPublications && t.Name.Space == rdfNS && t.Name.Local == "li":
				if current.Channel != "" {
					pubs = append(pubs, current)
				}
				inRDFLi = false
				current = Publication{}
			case inRDFLi && t.Name.Space == ulNamespace:
				currentField = ""
			}
		case xml.CharData:
			if inRDFLi {
				val := strings.TrimSpace(string(t))
				switch currentField {
				case "Channel":
					current.Channel = val
				case "Account":
					current.Account = val
				case "PostID":
					current.PostID = val
				case "GalleryTitle":
					current.GalleryTitle = val
				case "Slug":
					current.Slug = val
				case "Unlisted":
					current.Unlisted = val == "true"
				case "PublishedAt":
					current.PublishedAt, _ = time.Parse(time.RFC3339, val)
				}
			}
		}
	}

	return pubs, nil
}

// ulContent is everything Unterlumen's own description holds: the
// publication records, the fields (ADR-0048) and the things found in the
// photo (ADR-0054). A writer of one part reads the whole and writes it back,
// so the others stay.
type ulContent struct {
	pubs   []Publication
	fields map[string]string
	things ThingsRecord
}

func readULContent(data []byte) ulContent {
	pubs, _ := parseSidecarPublications(data)
	return ulContent{pubs: pubs, fields: parseSidecarFields(data), things: parseThingsRecord(data)}
}

// renderFreshXMP creates a complete XMP sidecar with just the unterlumen namespace.
func renderFreshXMP(pubs []Publication) string {
	return renderFreshXMPWith(ulContent{pubs: pubs})
}

func renderFreshXMPWith(c ulContent) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
` + renderULBlock(c) + `
  </rdf:RDF>
</x:xmpmeta>`
}

// renderULBlock produces the rdf:Description block for the unterlumen
// namespace. A sidecar that holds only fields or things has no empty
// Publications element.
func renderULBlock(c ulContent) string {
	var items strings.Builder
	for _, p := range c.pubs {
		items.WriteString("\n        <rdf:li rdf:parseType=\"Resource\">")
		items.WriteString("\n          <ul:Channel>" + xmlEscapeStr(p.Channel) + "</ul:Channel>")
		if p.Account != "" {
			items.WriteString("\n          <ul:Account>" + xmlEscapeStr(p.Account) + "</ul:Account>")
		}
		if p.PostID != "" {
			items.WriteString("\n          <ul:PostID>" + xmlEscapeStr(p.PostID) + "</ul:PostID>")
		}
		if p.GalleryTitle != "" {
			items.WriteString("\n          <ul:GalleryTitle>" + xmlEscapeStr(p.GalleryTitle) + "</ul:GalleryTitle>")
		}
		if p.Slug != "" {
			items.WriteString("\n          <ul:Slug>" + xmlEscapeStr(p.Slug) + "</ul:Slug>")
		}
		if p.Unlisted {
			items.WriteString("\n          <ul:Unlisted>true</ul:Unlisted>")
		}
		items.WriteString("\n          <ul:PublishedAt>" + p.PublishedAt.UTC().Format(time.RFC3339) + "</ul:PublishedAt>")
		items.WriteString("\n        </rdf:li>")
	}
	var publications string
	if len(c.pubs) > 0 || len(c.fields) == 0 && c.things.empty() {
		publications = `
      <ul:Publications>
        <rdf:Bag>` + items.String() + `
        </rdf:Bag>
      </ul:Publications>`
	}
	return `    <rdf:Description rdf:about="" xmlns:ul="https://unterlumen.app/xmp/1.0/">` + publications + renderFields(c.fields) + renderThingsRecord(c.things) + `
    </rdf:Description>`
}

// mergeULBlock replaces the publication records in existing XMP and keeps
// the rest of Unterlumen's description.
func mergeULBlock(existing []byte, pubs []Publication) []byte {
	c := readULContent(existing)
	c.pubs = pubs
	return spliceULBlock(existing, c)
}

// spliceULBlock replaces the unterlumen rdf:Description block in existing XMP.
// If no unterlumen block exists, inserts one before </rdf:RDF>.
func spliceULBlock(existing []byte, c ulContent) []byte {
	s := string(existing)
	newBlock := renderULBlock(c)

	const marker = `xmlns:ul="https://unterlumen.app/xmp/1.0/"`
	idx := strings.Index(s, marker)
	if idx == -1 {
		endTag := "</rdf:RDF>"
		endIdx := strings.LastIndex(s, endTag)
		if endIdx == -1 {
			return []byte(renderFreshXMPWith(c))
		}
		return []byte(s[:endIdx] + "  " + newBlock + "\n  " + s[endIdx:])
	}

	descStart := strings.LastIndex(s[:idx], "<rdf:Description")
	if descStart == -1 {
		return []byte(renderFreshXMPWith(c))
	}

	closeTag := "</rdf:Description>"
	descEnd := strings.Index(s[descStart:], closeTag)
	if descEnd == -1 {
		selfClose := strings.Index(s[descStart:], "/>")
		if selfClose == -1 {
			return []byte(renderFreshXMPWith(c))
		}
		descEnd = descStart + selfClose + 2
	} else {
		descEnd = descStart + descEnd + len(closeTag)
	}

	return []byte(s[:descStart] + newBlock + s[descEnd:])
}

func xmlEscapeStr(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s)) //nolint:errcheck
	return b.String()
}
