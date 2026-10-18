package library

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"huepattl.de/unterlumen/internal/faces"
	"huepattl.de/unterlumen/internal/media"
)

const facesBatchSize = 50 // photos stored per transaction

// regionMatch is how much a face and a region in the sidecar must overlap
// to be the same face. Other programs draw their boxes a little differently.
const regionMatch = 0.3

// RunFindFacesMissing finds the faces in every photo that has a thumbnail
// but was not looked at, or was looked at by an older faces.Version, and
// then groups the unnamed faces anew. A thumbnail that cannot be read is
// passed over; the next pass tries again.
func (idx *Indexer) RunFindFacesMissing(ctx context.Context, engine *faces.Engine, progress chan<- Progress) {
	defer close(progress)
	tasks, err := idx.store.PhotosNeedingFaces()
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}
	total, done := len(tasks), 0
	var batch []FoundFaces
	for found := range idx.findFaces(ctx, engine, tasks) {
		done++
		if found.PhotoID != "" {
			batch = append(batch, found)
		}
		if len(batch) >= facesBatchSize {
			idx.store.SaveFoundFaces(batch) //nolint:errcheck // looked at again next pass
			batch = batch[:0]
		}
		progress <- Progress{Done: done, Total: total}
	}
	if len(batch) > 0 {
		idx.store.SaveFoundFaces(batch) //nolint:errcheck
	}
	if err := idx.store.Regroup(); err != nil {
		progress <- Progress{Done: done, Total: total, Error: err.Error(), Finished: true}
		return
	}
	progress <- Progress{Done: done, Total: total, Finished: true}
}

// findFaces looks at the tasks' thumbnails on analyseWorkers goroutines. It
// sends one result per task; a result without a photo ID is a thumbnail
// that could not be read.
func (idx *Indexer) findFaces(ctx context.Context, engine *faces.Engine, tasks []FaceTask) <-chan FoundFaces {
	in := make(chan FaceTask)
	out := make(chan FoundFaces)
	go func() {
		defer close(in)
		for _, t := range tasks {
			select {
			case in <- t:
			case <-ctx.Done():
				return
			}
		}
	}()
	var wg sync.WaitGroup
	for range analyseWorkers {
		wg.Go(func() {
			for t := range in {
				out <- idx.findFacesIn(engine, t)
			}
		})
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func (idx *Indexer) findFacesIn(engine *faces.Engine, t FaceTask) FoundFaces {
	f, err := os.Open(filepath.Join(idx.libDir, t.ThumbPath))
	if err != nil {
		return FoundFaces{}
	}
	defer f.Close()
	found, err := engine.FindJPEG(f)
	if err != nil {
		return FoundFaces{}
	}
	regions, _ := media.ReadFaceRegions(t.PathHint, t.Orientation)
	return FoundFaces{PhotoID: t.ID, Faces: found, Names: namesFromRegions(found, regions)}
}

// namesFromRegions gives each face the name of the region it overlaps
// most, if it overlaps one enough.
func namesFromRegions(found []faces.Face, regions []media.FaceRegion) []string {
	names := make([]string, len(found))
	for i, f := range found {
		best := regionMatch
		for _, r := range regions {
			if o := overlap(f.X, f.Y, f.W, f.H, r.X, r.Y, r.W, r.H); o >= best {
				best, names[i] = o, r.Name
			}
		}
	}
	return names
}

// overlap is the intersection of two boxes over their union.
func overlap(ax, ay, aw, ah, bx, by, bw, bh float64) float64 {
	x1, y1 := max(ax, bx), max(ay, by)
	x2, y2 := min(ax+aw, bx+bw), min(ay+ah, by+bh)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	inter := (x2 - x1) * (y2 - y1)
	return inter / (aw*ah + bw*bh - inter)
}
