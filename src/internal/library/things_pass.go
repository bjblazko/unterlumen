package library

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/things"
)

const thingsBatchSize = 50 // photos stored per transaction

// thingsLook is what a things pass looks with: the engine, and the word
// list's vectors.
type thingsLook struct {
	engine *things.Engine
	words  [][]float32
}

// RunFindThingsMissing looks for things in every photo that has a thumbnail
// and was not looked at by the current models and word list (ADR-0054). A
// thumbnail that cannot be read is passed over; the next pass tries again.
func (idx *Indexer) RunFindThingsMissing(ctx context.Context, look thingsLook, progress chan<- Progress) {
	defer close(progress)
	tasks, err := idx.store.PhotosNeedingThings()
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}
	total, done := len(tasks), 0
	var batch []FoundThings
	for found := range idx.findThings(ctx, look, tasks) {
		done++
		if found.PhotoID != "" {
			batch = append(batch, found)
		}
		if len(batch) >= thingsBatchSize {
			idx.store.SaveFoundThings(batch) //nolint:errcheck // looked at again next pass
			batch = batch[:0]
		}
		progress <- Progress{Done: done, Total: total}
	}
	if len(batch) > 0 {
		idx.store.SaveFoundThings(batch) //nolint:errcheck
	}
	progress <- Progress{Done: done, Total: total, Finished: true}
}

// findThings looks at the tasks on analyseWorkers goroutines, one result per
// task; a result without a photo ID is a thumbnail that could not be read.
func (idx *Indexer) findThings(ctx context.Context, look thingsLook, tasks []ThingTask) <-chan FoundThings {
	in := make(chan ThingTask)
	out := make(chan FoundThings)
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
				out <- idx.findThingsIn(look, t)
			}
		})
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// findThingsIn looks at one photo, then writes what it found into the
// sidecar — first, as ADR-0052 asks — unless the sidecar already holds
// what current models found there, on the other installation: that is
// taken over.
func (idx *Indexer) findThingsIn(look thingsLook, t ThingTask) FoundThings {
	res, ok := idx.lookAt(look, t)
	if !ok {
		return FoundThings{}
	}
	sc, err := media.ReadThings(t.PathHint)
	if err != nil {
		return FoundThings{}
	}
	if things.Current(sc.Record.Version) {
		res.Things, res.Written = takeOver(res.Things, sc.Things()), true
		return res
	}
	res.Things = slices.DeleteFunc(res.Things, func(f things.Found) bool { return slices.Contains(sc.Record.Not, f.Name) })
	res.Written, _ = media.WriteFoundThings(t.PathHint, thingNames(res.Things), things.RecordVersion())
	return res
}

// lookAt scores the photo: from its stored vector and boxes when only the
// word list changed, otherwise by running the models on its thumbnail.
func (idx *Indexer) lookAt(look thingsLook, t ThingTask) (FoundThings, bool) {
	if t.Vector != nil {
		boxes, err := idx.store.boxesOf(t.ID)
		if err != nil {
			return FoundThings{}, false
		}
		return FoundThings{PhotoID: t.ID, Things: things.Score(t.Vector, look.words, boxes)}, true
	}
	f, err := os.Open(filepath.Join(idx.libDir, t.ThumbPath))
	if err != nil {
		return FoundThings{}, false
	}
	defer f.Close()
	seen, err := look.engine.LookJPEG(f)
	if err != nil {
		return FoundThings{}, false
	}
	return FoundThings{PhotoID: t.ID, Things: things.Score(seen.Vector, look.words, seen.Boxes),
		Vector: seen.Vector, Boxes: seen.Boxes}, true
}

// takeOver is the things a sidecar names, with this pass's scores where it
// found them too.
func takeOver(found []things.Found, names []string) []things.Found {
	out := make([]things.Found, 0, len(names))
	for _, n := range names {
		f := things.Found{Name: n, Source: "sidecar"}
		if i := slices.IndexFunc(found, func(x things.Found) bool { return x.Name == n }); i >= 0 {
			f = found[i]
		}
		out = append(out, f)
	}
	return out
}

func thingNames(found []things.Found) []string {
	names := make([]string, len(found))
	for i, f := range found {
		names[i] = f.Name
	}
	return names
}
