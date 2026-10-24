package library

import (
	"context"
	"math/rand/v2"
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

// lookLoader gives the pass what it looks with, loaded on first use: a pass
// that only takes over what the other installation found — most of them,
// once one installation looked — never loads the models.
type lookLoader func() (*thingsLook, error)

// RunFindThingsMissing looks for things in every photo that has a thumbnail
// and was not looked at by the current models and word list (ADR-0054). A
// thumbnail that cannot be read is passed over; the next pass tries again.
func (idx *Indexer) RunFindThingsMissing(ctx context.Context, look lookLoader, progress chan<- Progress) {
	defer close(progress)
	tasks, err := idx.store.PhotosNeedingThings()
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}
	// Each pass goes through the photos in an order of its own: two
	// installations sharing a library then look at different photos first,
	// and each takes over what the other wrote, instead of both running the
	// models on the same photos side by side.
	rand.Shuffle(len(tasks), func(i, j int) { tasks[i], tasks[j] = tasks[j], tasks[i] })
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
func (idx *Indexer) findThings(ctx context.Context, look lookLoader, tasks []ThingTask) <-chan FoundThings {
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

// findThingsIn reads the photo's sidecar first. When it holds what current
// models found — the other installation looked first — that is taken over,
// and nothing is run. Otherwise the photo is looked at, and what was found
// is written into the sidecar first, as ADR-0052 asks, then into the index.
func (idx *Indexer) findThingsIn(load lookLoader, t ThingTask) FoundThings {
	sc, err := media.ReadThings(t.PathHint)
	if err != nil {
		return FoundThings{}
	}
	if things.Current(sc.Record.Version) {
		return takeOverThings(t, sc, idx.store.lexicon())
	}
	look, err := load()
	if err != nil {
		return FoundThings{}
	}
	res, ok := idx.lookAt(*look, t)
	if !ok {
		return FoundThings{}
	}
	vector := res.Vector
	if vector == nil {
		vector = t.Vector
	}
	res.Things = slices.DeleteFunc(res.Things, func(f things.Found) bool { return slices.Contains(sc.Record.Not, f.Name) })
	res.Written, _ = media.WriteFoundThings(t.PathHint, thingNames(res.Things), things.RecordVersion(), vector, idx.store.lexicon())
	res.VectorInSidecar = res.Written && vector != nil
	return res
}

// takeOverThings is what a current sidecar says of the photo, with its
// vector. A sidecar without the vector gets this index's, when there is one.
func takeOverThings(t ThingTask, sc media.SidecarThings, lex media.Lexicon) FoundThings {
	res := FoundThings{PhotoID: t.ID, Written: true}
	for _, n := range sc.Things(lex) {
		res.Things = append(res.Things, things.Found{Name: n, Source: "sidecar"})
	}
	switch {
	case sc.Record.Vector != nil:
		res.Vector, res.VectorInSidecar = sc.Record.Vector, true
	case t.Vector != nil:
		res.VectorInSidecar, _ = media.WriteThingsVector(t.PathHint, t.Vector)
	}
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
		Vector: seen.Vector, Boxes: append([]things.Box{}, seen.Boxes...)}, true // none found replaces old boxes too
}

func thingNames(found []things.Found) []string {
	names := make([]string, len(found))
	for i, f := range found {
		names[i] = f.Name
	}
	return names
}
