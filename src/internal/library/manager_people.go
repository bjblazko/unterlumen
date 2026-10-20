package library

import (
	"sort"
	"strings"

	"huepattl.de/unterlumen/internal/faces"
)

// People across libraries. Each library keeps its own people, and the name
// links them: Anna in one library is Anna in every other (ADR-0051). This
// view only reads; a name is given, changed or taken away in each library,
// where it is written into that library's sidecars.

// NamedIn is a person as one library knows them.
type NamedIn struct {
	Library     string `json:"library"`
	LibraryName string `json:"libraryName"`
	Person      int64  `json:"person"`
	Faces       int    `json:"faces"`
	Photos      int    `json:"photos"`
	Cover       int64  `json:"cover"`
}

// Someone is everyone of one name, in all the libraries that have them,
// the library with the most of their faces first.
type Someone struct {
	Name      string    `json:"name"`
	Faces     int       `json:"faces"`
	Photos    int       `json:"photos"`
	Libraries []NamedIn `json:"libraries"`
}

// Everyone lists the people named in any library, those with the most
// faces first.
func (m *Manager) Everyone() ([]Someone, error) {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil, err
	}
	byName := map[string]*Someone{}
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		people, err := store.People()
		if err != nil {
			continue
		}
		for _, p := range people {
			key := strings.ToLower(p.Name)
			s := byName[key]
			if s == nil {
				s = &Someone{Name: p.Name}
				byName[key] = s
			}
			s.Faces += p.Faces
			s.Photos += p.Photos
			s.Libraries = append(s.Libraries, NamedIn{l.ID, l.Name, p.ID, p.Faces, p.Photos, p.Cover})
		}
	}
	out := make([]Someone, 0, len(byName))
	for _, s := range byName {
		sort.SliceStable(s.Libraries, func(i, j int) bool { return s.Libraries[i].Faces > s.Libraries[j].Faces })
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Faces != out[j].Faces {
			return out[i].Faces > out[j].Faces
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// SomeoneIn is a person in one library with all their faces, the newest
// first: their page manages them there.
type SomeoneIn struct {
	NamedIn
	Shown []FaceRef `json:"shown"`
}

// NotNamedIn is a library where nobody has the name. LooksLike is someone
// named otherwise there who looks like them, else Group a group of
// unnamed faces there that does; neither when nothing there does.
type NotNamedIn struct {
	Library     string     `json:"library"`
	LibraryName string     `json:"libraryName"`
	LooksLike   *Person    `json:"looksLike,omitempty"`
	Group       *FaceGroup `json:"group,omitempty"`
}

// SomeoneEverywhere is a person across all libraries: where they are named,
// and in each other library who there may be them.
type SomeoneEverywhere struct {
	Name  string       `json:"name"`
	In    []SomeoneIn  `json:"in"`
	NotIn []NotNamedIn `json:"notIn"`
	With  []Companion  `json:"with"` // who is most often in the same photos, in all libraries together
}

// companionsShown is how many of the people most often with someone are named.
const companionsShown = 12

// SomeoneNamed gathers the person of that name from every library. Their
// faces are compared, on this computer, with the people and groups of the
// libraries that do not have the name, to suggest who there is them.
func (m *Manager) SomeoneNamed(name string) (SomeoneEverywhere, error) {
	out := SomeoneEverywhere{Name: name, In: []SomeoneIn{}, NotIn: []NotNamedIn{}, With: []Companion{}}
	with := map[string]*Companion{}
	libs, err := m.ListLibraries()
	if err != nil {
		return out, err
	}
	var looks [][]float32
	var others []*Library
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		p, ok, err := store.PersonNamed(name)
		if err != nil {
			return out, err
		}
		if !ok {
			others = append(others, l)
			continue
		}
		in, embs, err := someoneIn(store, l, p)
		if err != nil {
			return out, err
		}
		out.Name = p.Name
		out.In = append(out.In, in)
		looks = append(looks, embs...)
		if err := addCompanions(store, p.ID, with); err != nil {
			return out, err
		}
	}
	sort.SliceStable(out.In, func(i, j int) bool { return out.In[i].Faces > out.In[j].Faces })
	out.With = topCompanions(with)
	centre := faces.Mean(looks)
	for _, l := range others {
		not, err := m.whoMayBe(l, centre)
		if err != nil {
			return out, err
		}
		out.NotIn = append(out.NotIn, not)
	}
	return out, nil
}

// CompanionsOf lists who is most often in the same photos as the person of
// that name, in all libraries together.
func (m *Manager) CompanionsOf(name string) ([]Companion, error) {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil, err
	}
	with := map[string]*Companion{}
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		p, ok, err := store.PersonNamed(name)
		if err != nil {
			return nil, err
		}
		if ok {
			if err := addCompanions(store, p.ID, with); err != nil {
				return nil, err
			}
		}
	}
	return topCompanions(with), nil
}

// addCompanions adds the people in the same photos as person in one
// library to with, by name in any case.
func addCompanions(store *Store, person int64, with map[string]*Companion) error {
	cs, err := store.Companions(person)
	if err != nil {
		return err
	}
	for _, c := range cs {
		key := strings.ToLower(c.Name)
		if with[key] == nil {
			with[key] = &Companion{Name: c.Name}
		}
		with[key].Photos += c.Photos
	}
	return nil
}

func topCompanions(with map[string]*Companion) []Companion {
	out := make([]Companion, 0, len(with))
	for _, c := range with {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Photos != out[j].Photos {
			return out[i].Photos > out[j].Photos
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	if len(out) > companionsShown {
		out = out[:companionsShown]
	}
	return out
}

// someoneIn returns the person p of library l with their faces, and what
// they look like.
func someoneIn(store *Store, l *Library, p Person) (SomeoneIn, [][]float32, error) {
	refs, err := store.PersonFaces(p.ID)
	if err != nil {
		return SomeoneIn{}, nil, err
	}
	embs, err := store.personEmbeddings(p.ID)
	return SomeoneIn{NamedIn{l.ID, l.Name, p.ID, p.Faces, p.Photos, p.Cover}, refs}, embs, err
}

// whoMayBe looks in library l for someone who looks like centre: a person
// named otherwise first, else a group waiting for a name.
func (m *Manager) whoMayBe(l *Library, centre []float32) (NotNamedIn, error) {
	not := NotNamedIn{Library: l.ID, LibraryName: l.Name}
	if centre == nil {
		return not, nil
	}
	store, err := m.OpenStore(l.ID)
	if err != nil {
		return not, err
	}
	if not.LooksLike, err = store.nearestPerson(centre); err != nil || not.LooksLike != nil {
		return not, err
	}
	not.Group, err = store.nearestGroup(centre)
	return not, err
}

// ToName is what waits for a name in one library.
type ToName struct {
	Library     string `json:"library"`
	LibraryName string `json:"libraryName"`
	Groups      int    `json:"groups"`  // groups of faces waiting for a name
	Pending     int    `json:"pending"` // photos whose faces were not looked for yet
	Finding     bool   `json:"finding"` // a pass is looking for faces now
}

// ToNameAll lists, for every library, what waits for a name there.
func (m *Manager) ToNameAll() ([]ToName, error) {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil, err
	}
	out := []ToName{}
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		groups, err := store.GroupCount()
		if err != nil {
			return nil, err
		}
		pending, err := store.PhotosNeedingFaces()
		if err != nil {
			return nil, err
		}
		out = append(out, ToName{l.ID, l.Name, groups, len(pending), m.FindingFaces(l.ID)})
	}
	return out, nil
}
