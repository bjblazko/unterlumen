package library

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Duplicates are versions of one photo in the libraries (ADR-0059): the same
// file at several paths, the same picture at another size or saved again,
// or a lightly edited copy. All of it is read from the index, nothing is
// measured again.

// Kinds of duplicate group, from the closest to the loosest, and the
// relations of a version to the group's largest.
const (
	DupIdentical = "identical" // the same bytes
	DupSame      = "same"      // the same picture in another file: scaled or re-saved
	DupSimilar   = "similar"   // an edited or cropped copy

	DupScaled  = "scaled"  // the same picture at fewer pixels
	DupResaved = "resaved" // the same picture at as many pixels, saved again
)

// DupFile is one version: a file on disk, in one library or several.
type DupFile struct {
	Path      string     `json:"path"`
	Name      string     `json:"name"`
	Ext       string     `json:"ext"`
	Size      int64      `json:"size"`
	Width     int        `json:"width"`
	Height    int        `json:"height"`
	PhotoID   string     `json:"photoID"`
	Libraries []DupLibID `json:"libraries"`
	Relation  string     `json:"relation"` // to the group's largest; "" for that one
	taken     string
	subsec    string // the fraction of the second it was taken in, as EXIF writes it
	hash      uint64
	hashed    bool
}

// DupLibID names a library a version is in.
type DupLibID struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DupGroup is the versions of one photo, the largest first.
type DupGroup struct {
	Kind  string     `json:"kind"`
	Files []*DupFile `json:"files"`
}

// DupReport is every group, and how many photos took part only in the
// search for identical files because their look is not measured yet.
type DupReport struct {
	Groups     []DupGroup `json:"groups"`
	Unanalysed int        `json:"unanalysed"`
}

// Duplicates finds the duplicates in all libraries. A path in two libraries
// that overlap is one file there, not two.
func (m *Manager) Duplicates(ctx context.Context) (*DupReport, error) {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil, err
	}
	byPath := map[string]*DupFile{}
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		files, err := store.duplicateCandidates(ctx)
		if err != nil {
			return nil, err
		}
		in := DupLibID{ID: l.ID, Name: l.Name}
		for _, f := range files {
			if seen := byPath[f.Path]; seen != nil {
				seen.Libraries = append(seen.Libraries, in)
				continue
			}
			f.Libraries = []DupLibID{in}
			byPath[f.Path] = f
		}
	}
	report := groupDuplicates(byPath)
	report.Groups = presentGroups(report.Groups)
	return report, nil
}

// duplicateCandidates reads every path of every photo of the library that is
// not a video, with what tells versions apart.
func (s *Store) duplicateCandidates(ctx context.Context) ([]*DupFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pc.abs_path, p.id, pc.file_size, p.ext, COALESCE(p.date_taken, ''),
		       CAST(TRIM(COALESCE(px.value, iw.value, '0'), '"') AS INTEGER),
		       CAST(TRIM(COALESCE(py.value, ih.value, '0'), '"') AS INTEGER),
		       a.dhash, COALESCE(TRIM(sub.value, '"'), '')
		FROM path_cache pc
		JOIN photos p ON p.id = pc.photo_id
		LEFT JOIN photo_appearance a ON a.photo_id = p.id
		LEFT JOIN exif_index px ON px.photo_id = p.id AND px.field = 'PixelXDimension'
		LEFT JOIN exif_index py ON py.photo_id = p.id AND py.field = 'PixelYDimension'
		LEFT JOIN exif_index iw ON iw.photo_id = p.id AND iw.field = 'ImageWidth'
		LEFT JOIN exif_index ih ON ih.photo_id = p.id AND ih.field = 'ImageLength'
		LEFT JOIN exif_index sub ON sub.photo_id = p.id AND sub.field = 'SubSecTimeOriginal'
		WHERE p.status = 'ok' AND `+notVideo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*DupFile
	for rows.Next() {
		f := &DupFile{}
		var hash *int64
		if err := rows.Scan(&f.Path, &f.PhotoID, &f.Size, &f.Ext, &f.taken, &f.Width, &f.Height, &hash, &f.subsec); err != nil {
			return nil, err
		}
		f.Name = filepath.Base(f.Path)
		if hash != nil {
			f.hash, f.hashed = uint64(*hash), true
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// presentGroups keeps the versions still on disk and the groups that still
// have two, reads their sizes from the files, and puts the largest first.
func presentGroups(groups []DupGroup) []DupGroup {
	var files []*DupFile
	for _, g := range groups {
		files = append(files, g.Files...)
	}
	onDisk := checkOnDisk(files)
	out := groups[:0]
	for _, g := range groups {
		var kept []*DupFile
		for _, f := range g.Files {
			if onDisk[f] {
				kept = append(kept, f)
			}
		}
		if len(kept) < 2 {
			continue
		}
		g.Files = kept
		describeVersions(&g)
		out = append(out, g)
	}
	sortGroups(out)
	return out
}

// checkOnDisk says which files are still there and reads their sizes from
// the files themselves: EXIF keeps the old size after an edit elsewhere,
// and it is used only for a format Go cannot read. On a network share each
// look takes a while, so several are taken at once.
func checkOnDisk(files []*DupFile) map[*DupFile]bool {
	const lookers = 16
	found := make([]bool, len(files))
	next := make(chan int)
	var wg sync.WaitGroup
	for range lookers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				found[i] = readStoredSize(files[i])
			}
		}()
	}
	for i := range files {
		next <- i
	}
	close(next)
	wg.Wait()
	onDisk := make(map[*DupFile]bool, len(files))
	for i, f := range files {
		onDisk[f] = found[i]
	}
	return onDisk
}

// describeVersions puts the largest version first — most pixels, then most
// bytes — and says how each other one relates to it.
func describeVersions(g *DupGroup) {
	sort.SliceStable(g.Files, func(i, j int) bool {
		a, b := g.Files[i], g.Files[j]
		if pa, pb := a.Width*a.Height, b.Width*b.Height; pa != pb {
			return pa > pb
		}
		if a.Size != b.Size {
			return a.Size > b.Size
		}
		return a.Path < b.Path
	})
	largest := g.Files[0]
	largest.Relation = ""
	kind := DupIdentical
	for _, f := range g.Files[1:] {
		f.Relation = relationTo(largest, f)
		kind = looser(kind, f.Relation)
	}
	g.Kind = kind
}

// sortGroups orders the identical groups first, then those of the same
// picture, then the similar ones; within a kind the largest files first.
func sortGroups(groups []DupGroup) {
	rank := map[string]int{DupIdentical: 0, DupSame: 1, DupSimilar: 2}
	sort.SliceStable(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if rank[a.Kind] != rank[b.Kind] {
			return rank[a.Kind] < rank[b.Kind]
		}
		if a.Files[0].Size != b.Files[0].Size {
			return a.Files[0].Size > b.Files[0].Size
		}
		return strings.Compare(a.Files[0].Path, b.Files[0].Path) < 0
	})
}

// readStoredSize opens the file — one look at the share, which also says it
// is there — and takes its size from its header when Go can read it.
func readStoredSize(f *DupFile) bool {
	file, err := os.Open(f.Path)
	if err != nil {
		return false
	}
	defer file.Close()
	if cfg, _, err := image.DecodeConfig(file); err == nil && cfg.Width > 0 && cfg.Height > 0 {
		f.Width, f.Height = cfg.Width, cfg.Height
	}
	return true
}
