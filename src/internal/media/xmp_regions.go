package media

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// Face regions follow the Metadata Working Group's region schema, which
// Lightroom, digiKam and others read and write (ADR-0051). Unterlumen writes
// only the names its owner confirmed, in an rdf:Description of its own.

const (
	mwgRegionsNS = "http://www.metadataworkinggroup.com/schemas/regions/"
	stAreaNS     = "http://ns.adobe.com/xmp/sType/Area#"
	stDimNS      = "http://ns.adobe.com/xap/1.0/sType/Dimensions#"

	regionsBlockStart = `<rdf:Description rdf:about="" xmlns:mwg-rs="` + mwgRegionsNS + `"`
)

const emptyXMP = `<?xml version="1.0" encoding="UTF-8"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  </rdf:RDF>
</x:xmpmeta>`

// ErrForeignRegions means another program keeps face regions in this
// sidecar, inside a description Unterlumen does not own; it is left alone.
var ErrForeignRegions = errors.New("another program keeps the face regions of this photo")

// FaceRegion is a named face. Its box is in fractions of the photo as it is
// displayed — upright — with its origin top left.
type FaceRegion struct {
	Name       string
	X, Y, W, H float64
}

// ReadFaceRegions reads the named faces from the sidecar beside photoPath,
// from any program. orientation is the photo's EXIF orientation: regions
// are stored for the photo before it is turned upright.
func ReadFaceRegions(photoPath string, orientation int) ([]FaceRegion, error) {
	data, err := os.ReadFile(SidecarPath(photoPath))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	stored := parseFaceRegions(data)
	for i, r := range stored {
		stored[i] = storedToUpright(r, orientation)
	}
	return stored, nil
}

// WriteFaceRegions replaces the face regions Unterlumen wrote into the
// sidecar beside photoPath with regions; none removes them. width and height
// are the stored photo's pixels, before orientation is applied; 0 when
// unknown, which leaves out the dimensions.
func WriteFaceRegions(photoPath string, regions []FaceRegion, orientation, width, height int) error {
	path := SidecarPath(photoPath)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if len(regions) == 0 {
			return nil
		}
		data = []byte(emptyXMP)
	} else if err != nil {
		return err
	}
	stored := make([]FaceRegion, len(regions))
	for i, r := range regions {
		stored[i] = uprightToStored(r, orientation)
	}
	out, err := spliceRegionsBlock(data, renderRegionsBlock(stored, width, height))
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// spliceRegionsBlock puts block in place of Unterlumen's regions
// description, or before </rdf:RDF> when there is none. An empty block
// removes it.
func spliceRegionsBlock(data []byte, block string) ([]byte, error) {
	s := string(data)
	start := strings.Index(s, regionsBlockStart)
	end := start
	if start >= 0 {
		closing := strings.Index(s[start:], "</rdf:Description>")
		if closing < 0 {
			return nil, fmt.Errorf("the sidecar's face regions are not closed")
		}
		end = start + closing + len("</rdf:Description>")
	}
	rest := s
	if start >= 0 {
		rest = s[:start] + s[end:]
	}
	if strings.Contains(rest, "xmlns:mwg-rs=") || strings.Contains(rest, mwgRegionsNS) {
		return nil, ErrForeignRegions
	}
	if start >= 0 {
		if block == "" {
			return []byte(strings.TrimRight(s[:start], " \t") + strings.TrimLeft(s[end:], "\n")), nil
		}
		return []byte(s[:start] + block + s[end:]), nil
	}
	if block == "" {
		return data, nil
	}
	at := strings.LastIndex(s, "</rdf:RDF>")
	if at < 0 {
		return nil, fmt.Errorf("the sidecar has no rdf:RDF element")
	}
	return []byte(s[:at] + "  " + block + "\n  " + s[at:]), nil
}

func renderRegionsBlock(regions []FaceRegion, width, height int) string {
	if len(regions) == 0 {
		return ""
	}
	var items strings.Builder
	for _, r := range regions {
		items.WriteString("\n            <rdf:li rdf:parseType=\"Resource\">")
		items.WriteString("\n              <mwg-rs:Name>" + xmlEscapeStr(r.Name) + "</mwg-rs:Name>")
		items.WriteString("\n              <mwg-rs:Type>Face</mwg-rs:Type>")
		items.WriteString(fmt.Sprintf("\n              <mwg-rs:Area stArea:x=\"%s\" stArea:y=\"%s\" stArea:w=\"%s\" stArea:h=\"%s\" stArea:unit=\"normalized\"/>",
			fraction(r.X+r.W/2), fraction(r.Y+r.H/2), fraction(r.W), fraction(r.H)))
		items.WriteString("\n            </rdf:li>")
	}
	dims := ""
	if width > 0 && height > 0 {
		dims = `
        <mwg-rs:AppliedToDimensions stDim:w="` + strconv.Itoa(width) + `" stDim:h="` + strconv.Itoa(height) + `" stDim:unit="pixel"/>`
	}
	return regionsBlockStart + ` xmlns:stArea="` + stAreaNS + `" xmlns:stDim="` + stDimNS + `">
      <mwg-rs:Regions rdf:parseType="Resource">` + dims + `
        <mwg-rs:RegionList>
          <rdf:Bag>` + items.String() + `
          </rdf:Bag>
        </mwg-rs:RegionList>
      </mwg-rs:Regions>
    </rdf:Description>`
}

func fraction(v float64) string { return strconv.FormatFloat(v, 'f', 5, 64) }

// parseFaceRegions reads every named region of type Face (or of no type)
// in stored orientation, turning MWG's centre into a top-left corner.
func parseFaceRegions(data []byte) []FaceRegion {
	const rdfNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	dec := xml.NewDecoder(bytes.NewReader(data))
	var regions []FaceRegion
	var inList, inLi bool
	var cur FaceRegion
	var kind, field string
	for {
		tok, err := dec.Token()
		if err == io.EOF || err != nil {
			return regions
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if inLi {
				readRegionAttrs(t.Attr, &cur, &kind)
			}
			switch {
			case t.Name.Space == mwgRegionsNS && t.Name.Local == "RegionList":
				inList = true
			case inList && !inLi && t.Name.Space == rdfNS && t.Name.Local == "li":
				inLi, cur, kind = true, FaceRegion{}, ""
				readRegionAttrs(t.Attr, &cur, &kind)
			case inLi && t.Name.Space == mwgRegionsNS && t.Name.Local == "Area":
				readArea(t.Attr, &cur)
			case inLi && t.Name.Space == mwgRegionsNS:
				field = t.Name.Local
			}
		case xml.EndElement:
			switch {
			case t.Name.Space == mwgRegionsNS && t.Name.Local == "RegionList":
				inList = false
			case inLi && t.Name.Space == rdfNS && t.Name.Local == "li":
				inLi = false
				if cur.Name != "" && (kind == "" || kind == "Face") && cur.W > 0 && cur.H > 0 {
					regions = append(regions, cur)
				}
			case inLi && t.Name.Space == mwgRegionsNS:
				field = ""
			}
		case xml.CharData:
			switch field {
			case "Name":
				cur.Name += strings.TrimSpace(string(t))
			case "Type":
				kind += strings.TrimSpace(string(t))
			}
		}
	}
}

// readRegionAttrs reads Name and Type written as attributes, as Lightroom
// does on an rdf:Description inside the rdf:li.
func readRegionAttrs(attrs []xml.Attr, r *FaceRegion, kind *string) {
	for _, a := range attrs {
		switch {
		case a.Name.Space == mwgRegionsNS && a.Name.Local == "Name":
			r.Name = strings.TrimSpace(a.Value)
		case a.Name.Space == mwgRegionsNS && a.Name.Local == "Type":
			*kind = a.Value
		}
	}
}

func readArea(attrs []xml.Attr, r *FaceRegion) {
	var cx, cy float64
	for _, a := range attrs {
		if a.Name.Space != stAreaNS {
			continue
		}
		v, _ := strconv.ParseFloat(a.Value, 64)
		switch a.Name.Local {
		case "x":
			cx = v
		case "y":
			cy = v
		case "w":
			r.W = v
		case "h":
			r.H = v
		}
	}
	r.X, r.Y = cx-r.W/2, cy-r.H/2
}

// storedToUpright moves a region of the stored photo to where it is once
// the EXIF orientation is applied.
func storedToUpright(r FaceRegion, orientation int) FaceRegion {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	w, h := r.W, r.H
	switch orientation {
	case 2:
		cx = 1 - cx
	case 3:
		cx, cy = 1-cx, 1-cy
	case 4:
		cy = 1 - cy
	case 5:
		cx, cy, w, h = cy, cx, h, w
	case 6:
		cx, cy, w, h = 1-cy, cx, h, w
	case 7:
		cx, cy, w, h = 1-cy, 1-cx, h, w
	case 8:
		cx, cy, w, h = cy, 1-cx, h, w
	}
	return FaceRegion{Name: r.Name, X: cx - w/2, Y: cy - h/2, W: w, H: h}
}

// uprightToStored undoes storedToUpright.
func uprightToStored(r FaceRegion, orientation int) FaceRegion {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	w, h := r.W, r.H
	switch orientation {
	case 2:
		cx = 1 - cx
	case 3:
		cx, cy = 1-cx, 1-cy
	case 4:
		cy = 1 - cy
	case 5:
		cx, cy, w, h = cy, cx, h, w
	case 6:
		cx, cy, w, h = cy, 1-cx, h, w
	case 7:
		cx, cy, w, h = 1-cy, 1-cx, h, w
	case 8:
		cx, cy, w, h = 1-cy, cx, h, w
	}
	return FaceRegion{Name: r.Name, X: cx - w/2, Y: cy - h/2, W: w, H: h}
}
