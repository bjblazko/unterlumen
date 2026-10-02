package timeline

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"

	"huepattl.de/unterlumen/internal/library"
)

// Builder keeps the current stream and builds it again when a library has
// changed. A library whose database cannot be read is left out, as on the
// Map.
type Builder struct {
	mgr *library.Manager
	mu  sync.Mutex
	cur *Stream
}

// Scope is what the Timeline shows: some libraries (all when IDs is empty)
// and the shared filter's cameras and lenses (ADR-0050).
type Scope struct {
	IDs    []string
	Filter library.Filter
}

func (sc Scope) key() string {
	return fmt.Sprintf("%v|%s", sc.IDs, sc.Filter.Key())
}

func NewBuilder(mgr *library.Manager) *Builder {
	return &Builder{mgr: mgr}
}

// Current returns the stream for the libraries as they are now, within sc.
// The scope is part of the version, so a page asked for under another scope
// is refused like one of an older stream.
func (b *Builder) Current(sc Scope) (*Stream, error) {
	libs, err := b.orderedLibraries(sc.IDs)
	if err != nil {
		return nil, err
	}
	version := b.version(libs, sc)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cur != nil && b.cur.Version == version {
		return b.cur, nil
	}
	s := b.build(libs, version, sc.Filter)
	// A scan that ran while the photos were read may have left them half
	// done; such a stream is served once but not kept under a version it
	// does not match.
	if b.version(libs, sc) == version {
		b.cur = s
	}
	return s, nil
}

// orderedLibraries lists the libraries in the sidebar's order: by the
// position the owner gave them, then by name.
func (b *Builder) orderedLibraries(ids []string) ([]*library.Library, error) {
	libs, err := b.mgr.ListLibraries()
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		want := map[string]bool{}
		for _, id := range ids {
			want[id] = true
		}
		kept := libs[:0]
		for _, l := range libs {
			if want[l.ID] {
				kept = append(kept, l)
			}
		}
		libs = kept
	}
	sort.SliceStable(libs, func(i, j int) bool { return libraryBefore(libs[i], libs[j]) })
	return libs, nil
}

func libraryBefore(a, b *library.Library) bool {
	pa, pb := a.SortPosition, b.SortPosition
	if (pa == nil) != (pb == nil) {
		return pa != nil
	}
	if pa != nil && *pa != *pb {
		return *pa < *pb
	}
	return a.Name < b.Name
}

// version hashes each library's content stamp, so it changes with any scan,
// deletion or reorder.
func (b *Builder) version(libs []*library.Library, sc Scope) string {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s;", sc.key())
	for _, l := range libs {
		store, err := b.mgr.OpenStore(l.ID)
		if err != nil {
			continue
		}
		stamp, err := store.ContentStamp()
		store.Close()
		if err == nil {
			fmt.Fprintf(h, "%s=%s;", l.ID, stamp)
		}
	}
	return strconv.FormatUint(h.Sum64(), 36)
}

func (b *Builder) build(libs []*library.Library, version string, f library.Filter) *Stream {
	var all []LibraryPhotos
	for _, l := range libs {
		if lp, err := b.read(l.ID, f); err == nil {
			all = append(all, lp)
		}
	}
	photos, undated := merge(all)
	start := assignDays(photos)
	return &Stream{Version: version, Start: start, Photos: photos, Undated: undated}
}

func (b *Builder) read(id string, f library.Filter) (LibraryPhotos, error) {
	store, err := b.mgr.OpenFiltered(id, f)
	if err != nil {
		return LibraryPhotos{}, err
	}
	defer store.Close()
	dated, err := store.DatedPhotos()
	if err != nil {
		return LibraryPhotos{}, err
	}
	undated, err := store.UndatedPhotoIDs()
	if err != nil {
		return LibraryPhotos{}, err
	}
	return LibraryPhotos{LibraryID: id, Dated: dated, Undated: undated}, nil
}
