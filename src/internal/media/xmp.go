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
	Channel     string
	Account     string    // account ID within the channel; empty when channel has no sub-accounts
	PostID      string    // shared ID for photos published together in one action
	PublishedAt time.Time
}

// SidecarPath returns the XMP sidecar path for the given photo file.
func SidecarPath(photoPath string) string {
	ext := filepath.Ext(photoPath)
	return photoPath[:len(photoPath)-len(ext)] + ".xmp"
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
	existing, err := ReadSidecar(photoPath)
	if err != nil {
		return fmt.Errorf("read sidecar: %w", err)
	}
	pubs := append(existing, pub)

	sidecarPath := SidecarPath(photoPath)
	rawBytes, readErr := os.ReadFile(sidecarPath)
	if readErr == nil {
		merged := mergeULBlock(rawBytes, pubs)
		return os.WriteFile(sidecarPath, merged, 0o644)
	}
	return os.WriteFile(sidecarPath, []byte(renderFreshXMP(pubs)), 0o644)
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
				case "PublishedAt":
					current.PublishedAt, _ = time.Parse(time.RFC3339, val)
				}
			}
		}
	}

	return pubs, nil
}

// renderFreshXMP creates a complete XMP sidecar with just the unterlumen namespace.
func renderFreshXMP(pubs []Publication) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
` + renderULBlock(pubs) + `
  </rdf:RDF>
</x:xmpmeta>`
}

// renderULBlock produces the rdf:Description block for the unterlumen namespace.
func renderULBlock(pubs []Publication) string {
	var items strings.Builder
	for _, p := range pubs {
		items.WriteString("\n        <rdf:li rdf:parseType=\"Resource\">")
		items.WriteString("\n          <ul:Channel>" + xmlEscapeStr(p.Channel) + "</ul:Channel>")
		if p.Account != "" {
			items.WriteString("\n          <ul:Account>" + xmlEscapeStr(p.Account) + "</ul:Account>")
		}
		if p.PostID != "" {
			items.WriteString("\n          <ul:PostID>" + xmlEscapeStr(p.PostID) + "</ul:PostID>")
		}
		items.WriteString("\n          <ul:PublishedAt>" + p.PublishedAt.UTC().Format(time.RFC3339) + "</ul:PublishedAt>")
		items.WriteString("\n        </rdf:li>")
	}
	return `    <rdf:Description rdf:about="" xmlns:ul="https://unterlumen.app/xmp/1.0/">
      <ul:Publications>
        <rdf:Bag>` + items.String() + `
        </rdf:Bag>
      </ul:Publications>
    </rdf:Description>`
}

// mergeULBlock replaces the unterlumen rdf:Description block in existing XMP.
// If no unterlumen block exists, inserts one before </rdf:RDF>.
func mergeULBlock(existing []byte, pubs []Publication) []byte {
	s := string(existing)
	newBlock := renderULBlock(pubs)

	const marker = `xmlns:ul="https://unterlumen.app/xmp/1.0/"`
	idx := strings.Index(s, marker)
	if idx == -1 {
		endTag := "</rdf:RDF>"
		endIdx := strings.LastIndex(s, endTag)
		if endIdx == -1 {
			return []byte(renderFreshXMP(pubs))
		}
		return []byte(s[:endIdx] + "  " + newBlock + "\n  " + s[endIdx:])
	}

	descStart := strings.LastIndex(s[:idx], "<rdf:Description")
	if descStart == -1 {
		return []byte(renderFreshXMP(pubs))
	}

	closeTag := "</rdf:Description>"
	descEnd := strings.Index(s[descStart:], closeTag)
	if descEnd == -1 {
		selfClose := strings.Index(s[descStart:], "/>")
		if selfClose == -1 {
			return []byte(renderFreshXMP(pubs))
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
