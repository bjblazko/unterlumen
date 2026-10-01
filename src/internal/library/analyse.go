package library

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"huepattl.de/unterlumen/internal/appearance"
)

const (
	analyseWorkers   = 2   // decoding is the cost; two keep a NAS responsive
	analyseBatchSize = 100 // measurements stored per transaction
)

// RunAnalyseMissing measures the appearance of every photo that has a
// thumbnail but no measurements, or measurements of an older version. A
// thumbnail that cannot be read is passed over; the next pass tries again.
func (idx *Indexer) RunAnalyseMissing(ctx context.Context, progress chan<- Progress) {
	defer close(progress)
	tasks, err := idx.store.PhotosNeedingAppearance()
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}
	total := len(tasks)
	done := 0
	var batch []MeasuredAppearance
	for m := range idx.measure(ctx, tasks) {
		done++
		if m.ID != "" {
			batch = append(batch, m)
		}
		if len(batch) >= analyseBatchSize {
			idx.store.SaveAppearances(batch) //nolint:errcheck // measured again next pass
			batch = batch[:0]
		}
		progress <- Progress{Done: done, Total: total}
	}
	if len(batch) > 0 {
		idx.store.SaveAppearances(batch) //nolint:errcheck
	}
	progress <- Progress{Done: done, Total: total, Finished: true}
}

// measure analyses the tasks' thumbnails on analyseWorkers goroutines. It
// sends one result per task; a result without an ID is a thumbnail that
// could not be read.
func (idx *Indexer) measure(ctx context.Context, tasks []AppearanceTask) <-chan MeasuredAppearance {
	in := make(chan AppearanceTask)
	out := make(chan MeasuredAppearance)
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
				out <- idx.measureOne(t)
			}
		})
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func (idx *Indexer) measureOne(t AppearanceTask) MeasuredAppearance {
	f, err := os.Open(filepath.Join(idx.libDir, t.ThumbPath))
	if err != nil {
		return MeasuredAppearance{}
	}
	defer f.Close()
	r, err := appearance.AnalyseJPEG(f)
	if err != nil {
		return MeasuredAppearance{}
	}
	return MeasuredAppearance{ID: t.ID, Result: r}
}

// RunAnalyseAgainInFolder forgets the appearance of the photos inside
// subfolder and measures it again. An empty subfolder covers the library.
func (idx *Indexer) RunAnalyseAgainInFolder(ctx context.Context, progress chan<- Progress, subfolder string) {
	folder, pathErr := idx.resolveSubfolder(subfolder)
	if pathErr != "" {
		progress <- Progress{Error: pathErr, Finished: true}
		close(progress)
		return
	}
	if err := idx.store.ClearAppearanceInFolder(folder); err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		close(progress)
		return
	}
	idx.RunAnalyseMissing(ctx, progress)
}
