// Package facedecisions keeps what the owner decided about faces that does
// not belong in a photo's sidecar (ADR-0051, decision 15): which faces are
// hidden, which faces are not someone (a No), and which face shows a
// person. The sidecar says only who is in a photo. The decisions live in
// face-decisions.json beside channels.json — the folder installations
// share — so the NAS and the Mac, with their own indexes, decide once.
//
// A face is its photo — the photo's SHA-256, the same on every
// installation — and its box in fractions of the upright photo. Each
// installation finds its own faces, so a box matches the face that overlaps
// it enough, as a sidecar's region does.
package facedecisions

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"huepattl.de/unterlumen/internal/atomicfile"
	"huepattl.de/unterlumen/internal/media"
)

// Face is a face on every installation: its photo and its box.
type Face struct {
	Photo string     `json:"photo"`
	Box   [4]float64 `json:"box"` // x, y, w, h
}

// Not is a No: the face is not the person of that name.
type Not struct {
	Face
	Name string `json:"name"`
}

// Cover is the face that shows a person, and when it was chosen; across
// installations, as across libraries, the cover chosen last shows them.
type Cover struct {
	Face
	Chosen string `json:"chosen"`
}

// Decisions is the whole file.
type Decisions struct {
	Hidden []Face           `json:"hidden"`
	Not    []Not            `json:"not"`
	Covers map[string]Cover `json:"covers"` // by name, in lower case
}

// Store reads and writes face-decisions.json in one folder.
type Store struct {
	path string
}

// Path is the file: a library remembers where it shared its decisions.
func (s *Store) Path() string { return s.path }

// New keeps the decisions in dir.
func New(dir string) *Store {
	return &Store{path: filepath.Join(dir, "face-decisions.json")}
}

func parse(data []byte) (Decisions, error) {
	var d Decisions
	if len(data) > 0 {
		if err := json.Unmarshal(data, &d); err != nil {
			return d, err
		}
	}
	if d.Covers == nil {
		d.Covers = map[string]Cover{}
	}
	return d, nil
}

// ErrMissing says the file is not there. An installation that has shared
// its decisions must not take that for "no decisions": the shared folder
// may only be unreachable, and following an empty file would show every
// hidden face again.
var ErrMissing = errors.New("the shared face decisions are not there (face-decisions.json)")

// Read returns the decisions, or ErrMissing when there is no file.
func (s *Store) Read() (Decisions, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Decisions{}, ErrMissing
	}
	if err != nil {
		return Decisions{}, err
	}
	return parse(data)
}

// Change applies change to the decisions as they are in the file now, and
// writes them whole (atomicfile.Change): the other installation's
// decisions made meanwhile stay.
func (s *Store) Change(change func(*Decisions)) error {
	return atomicfile.Change(s.path, func(data []byte) ([]byte, error) {
		d, err := parse(data)
		if err != nil {
			return nil, err
		}
		before, _ := json.MarshalIndent(d, "", "  ")
		change(&d)
		after, err := json.MarshalIndent(d, "", "  ")
		if err != nil || string(after) == string(before) {
			return nil, err // nothing changed: nothing written
		}
		return after, nil
	})
}

// Key is how a name is kept: without regard to case, as names are matched.
func Key(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// Same says whether two faces are one: the same photo, boxes overlapping
// enough.
func Same(a, b Face) bool {
	return a.Photo == b.Photo && media.Overlap(region(a), region(b)) >= media.RegionMatch
}

func region(f Face) media.FaceRegion {
	return media.FaceRegion{X: f.Box[0], Y: f.Box[1], W: f.Box[2], H: f.Box[3]}
}

// AddHidden hides f, once; DropHidden shows it again.
func (d *Decisions) AddHidden(f Face) {
	for _, h := range d.Hidden {
		if Same(h, f) {
			return
		}
	}
	d.Hidden = append(d.Hidden, f)
}

func (d *Decisions) DropHidden(f Face) {
	kept := d.Hidden[:0]
	for _, h := range d.Hidden {
		if !Same(h, f) {
			kept = append(kept, h)
		}
	}
	d.Hidden = kept
}

// AddNot records that f is not name, once; DropNot takes it back.
func (d *Decisions) AddNot(f Face, name string) {
	for _, n := range d.Not {
		if Same(n.Face, f) && Key(n.Name) == Key(name) {
			return
		}
	}
	d.Not = append(d.Not, Not{Face: f, Name: name})
}

func (d *Decisions) DropNot(f Face, name string) {
	kept := d.Not[:0]
	for _, n := range d.Not {
		if !Same(n.Face, f) || Key(n.Name) != Key(name) {
			kept = append(kept, n)
		}
	}
	d.Not = kept
}

// SetCover makes f the face that shows name, chosen at chosen, unless a
// later choice is already there.
func (d *Decisions) SetCover(name string, f Face, chosen string) {
	if c, ok := d.Covers[Key(name)]; ok && c.Chosen > chosen {
		return
	}
	d.Covers[Key(name)] = Cover{Face: f, Chosen: chosen}
}
