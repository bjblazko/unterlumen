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

func NewBuilder(mgr *library.Manager) *Builder {
	return &Builder{mgr: mgr}
}

// Current returns the stream for the libraries as they are now.
func (b *Builder) Current() (*Stream, error) {
	libs, err := b.orderedLibraries()
	if err != nil {
		return nil, err
	}
	version := b.version(libs)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cur != nil && b.cur.Version == version {
		return b.cur, nil
	}
	b.cur = b.build(libs, version)
	return b.cur, nil
}

// orderedLibraries lists the libraries in the sidebar's order: by the
// position the owner gave them, then by name.
func (b *Builder) orderedLibraries() ([]*library.Library, error) {
	libs, err := b.mgr.ListLibraries()
	if err != nil {
		return nil, err
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
func (b *Builder) version(libs []*library.Library) string {
	h := fnv.New64a()
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

func (b *Builder) build(libs []*library.Library, version string) *Stream {
	var all []LibraryPhotos
	for _, l := range libs {
		if lp, err := b.read(l.ID); err == nil {
			all = append(all, lp)
		}
	}
	photos, undated := merge(all)
	start := assignDays(photos)
	return &Stream{Version: version, Start: start, Photos: photos, Undated: undated}
}

func (b *Builder) read(id string) (LibraryPhotos, error) {
	store, err := b.mgr.OpenStore(id)
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
