package media

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"slices"
	"strings"
)

// Things found in a photo go into the sidecar twice (ADR-0054): as keywords,
// dc:subject, which Lightroom, digiKam and others read, and as a record in
// Unterlumen's own description that says which keywords Unterlumen wrote —
// so the owner's own keywords are never taken for found things nor removed —
// which things the owner took away, so no later pass brings them back,
// which models and word list looked, and what CLIP saw — the photo's vector,
// so the other installation searches by words without running the models.

// ThingsRecord is Unterlumen's record of the things in a photo.
type ThingsRecord struct {
	Found   []string  // written as keywords by Unterlumen
	Not     []string  // taken away by the owner
	Version string    // the models and the word list that looked, "" for none
	Vector  []float32 // CLIP's vector of the photo; nil when not written
}

func (r ThingsRecord) empty() bool {
	return len(r.Found) == 0 && len(r.Not) == 0 && r.Version == "" && r.Vector == nil
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

// WriteFoundThings records what a pass of version found in the photo, and
// its vector, as the sidecar is now: things the owner took away stay away,
// keywords Unterlumen wrote before and no longer finds are removed, every
// other keyword stays. It writes nothing for a photo without a sidecar and
// without things, and reports whether the sidecar now holds the record.
func WriteFoundThings(photoPath string, found []string, version string, vector []float32) (bool, error) {
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
		if vector != nil {
			c.things.Vector = vector
		}
		return withThings(data, c, keep, gone), nil
	})
	return written && err == nil, err
}

// WriteThingsVector adds the photo's vector to a things record already in its
// sidecar, the rest as it is; nothing when there is no record.
func WriteThingsVector(photoPath string, vector []float32) (bool, error) {
	written := false
	err := changeSidecar(photoPath, func(data []byte) ([]byte, error) {
		written = false
		c := readULContent(data)
		if data == nil || c.things.Version == "" {
			return nil, nil
		}
		c.things.Vector, written = vector, true
		return spliceULBlock(data, c), nil
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
			case t.Name.Space == ulNamespace && (t.Name.Local == "ThingsVersion" || t.Name.Local == "ThingsVector"):
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
			case t.Name.Space == ulNamespace && t.Name.Local == "ThingsVector":
				inVersion = false
				r.Vector = decodeVector(strings.TrimSpace(text.String()))
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
	if r.Vector != nil {
		out += "\n      <ul:ThingsVector>" + encodeVector(r.Vector) + "</ul:ThingsVector>"
	}
	return out
}

// A vector is written as half-precision numbers, little-endian, in base64:
// 512 of them in 1.4 KB. Cosine similarity does not notice the precision.
func encodeVector(v []float32) string {
	b := make([]byte, 2*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint16(b[2*i:], float16(x))
	}
	return base64.StdEncoding.EncodeToString(b)
}

// decodeVector reads a vector encodeVector wrote; nil when it cannot.
func decodeVector(s string) []float32 {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b) == 0 || len(b)%2 != 0 {
		return nil
	}
	v := make([]float32, len(b)/2)
	for i := range v {
		v[i] = float32From16(binary.LittleEndian.Uint16(b[2*i:]))
	}
	return v
}

// float16 rounds x to IEEE 754 half precision. A vector of unit length has
// no value near the limits, so neither infinities nor NaN are handled.
func float16(x float32) uint16 {
	bits := math.Float32bits(x)
	sign := uint16(bits>>16) & 0x8000
	exp := int32(bits>>23&0xff) - 127 + 15
	mant := bits & 0x7fffff
	switch {
	case exp <= 0: // below the smallest normal half: as a subnormal, or zero
		if exp < -10 {
			return sign
		}
		mant |= 0x800000
		shift := uint32(14 - exp)
		half := uint16(mant >> shift)
		if mant>>(shift-1)&1 == 1 {
			half++
		}
		return sign | half
	case exp >= 31:
		return sign | 0x7bff // the largest half
	}
	half := sign | uint16(exp)<<10 | uint16(mant>>13)
	if mant&0x1000 != 0 {
		half++ // round half up; a carry into the exponent is still correct
	}
	return half
}

func float32From16(h uint16) float32 {
	sign := uint32(h&0x8000) << 16
	exp := uint32(h >> 10 & 0x1f)
	mant := uint32(h & 0x3ff)
	switch exp {
	case 0:
		if mant == 0 {
			return math.Float32frombits(sign)
		}
		return math.Float32frombits(sign) + float32(mant)/(1<<24)*signOf(sign)
	}
	return math.Float32frombits(sign | (exp-15+127)<<23 | mant<<13)
}

func signOf(sign uint32) float32 {
	if sign != 0 {
		return -1
	}
	return 1
}
