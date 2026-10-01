package library

import (
	"context"
	"errors"
	"sync/atomic"
)

// AnalyseAllMissing measures, library after library, the photos whose
// appearance is missing or outdated. It is run once at startup, for photos
// indexed before appearance was measured or before appearance.Version rose.
func (m *Manager) AnalyseAllMissing() {
	libs, err := m.ListLibraries()
	if err != nil {
		return
	}
	for _, l := range libs {
		m.AnalyseMissing(l.ID)
	}
}

// AnalyseMissing measures the appearance of a library's photos that lack it,
// and returns when done. It does not hold the index lock: a scan, or the
// cleanup after a file is deleted, must never wait for it or be turned away.
// It does not start while a scan runs, since the scan's end starts it again.
// Only one pass runs per library; a call while one runs makes it run once
// more, for the photos a scan added meanwhile.
func (m *Manager) AnalyseMissing(id string) {
	if m.IsScanning(id) {
		return
	}
	v, running := m.analysing.LoadOrStore(id, new(atomic.Bool))
	again := v.(*atomic.Bool)
	if running {
		again.Store(true)
		return
	}
	defer m.analysing.Delete(id)
	for {
		m.analyseOnce(id)
		if !again.Swap(false) {
			return
		}
	}
}

// analyseOnce runs one pass. Nothing shows in the status line when there is
// nothing to measure.
func (m *Manager) analyseOnce(id string) {
	store, err := m.OpenStore(id)
	if err != nil {
		return
	}
	if tasks, err := store.PhotosNeedingAppearance(); err != nil || len(tasks) == 0 {
		return
	}
	libInfo, err := libraryFromStore(id, store)
	if err != nil {
		return
	}
	job := m.jobs.Start("library", m.scanTitle(id, "Analysing"), "libraries")
	ch := make(chan Progress, 8)
	go NewIndexer(store, m.LibDir(id), libInfo.SourcePath).RunAnalyseMissing(context.Background(), ch)
	var last Progress
	for p := range ch {
		job.Progress(p.Done, p.Total, "")
		last = p
	}
	// New measurements change only the colours.
	dropEntriesOf(&m.colourCache, id)
	dropEntriesOf(&m.colourSpaceCache, id)
	if last.Error != "" {
		job.Finish(errors.New(last.Error))
		return
	}
	job.Finish(nil)
}
