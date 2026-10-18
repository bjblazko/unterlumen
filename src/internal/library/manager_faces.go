package library

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"

	"huepattl.de/unterlumen/internal/faces"
)

// facesState holds the face engine, loaded on first use from the
// recognition folder, and which libraries a face pass runs for.
type facesState struct {
	mu      sync.Mutex
	engine  *faces.Engine
	running sync.Map // map[libraryID]*atomic.Bool — as Manager.analysing
}

// RecognitionDir is where this installation keeps ONNX Runtime and the
// face models (ADR-0051). It is machine-local, like everything in the lib dir.
func (m *Manager) RecognitionDir() string {
	return filepath.Join(m.root, "recognition")
}

// FacesInstalled reports whether faces can be found here.
func (m *Manager) FacesInstalled() bool {
	return faces.Installed(m.RecognitionDir())
}

// faceEngine loads the engine once the recognition folder is complete.
func (m *Manager) faceEngine() (*faces.Engine, error) {
	m.faces.mu.Lock()
	defer m.faces.mu.Unlock()
	if m.faces.engine != nil {
		return m.faces.engine, nil
	}
	if !m.FacesInstalled() {
		return nil, errors.New("face recognition is not installed")
	}
	e, err := faces.Open(m.RecognitionDir(), 1)
	if err != nil {
		return nil, err
	}
	m.faces.engine = e
	return e, nil
}

// FindAllFacesMissing runs a face pass for every library, one after another.
func (m *Manager) FindAllFacesMissing() {
	if !m.FacesInstalled() {
		return
	}
	libs, err := m.ListLibraries()
	if err != nil {
		return
	}
	for _, l := range libs {
		m.FindFacesMissing(l.ID)
	}
}

// FindFacesMissing finds the faces in a library's photos that were not
// looked at, and returns when done. Like AnalyseMissing it holds no index
// lock, does not start during a scan, and runs once more when called while
// it runs.
func (m *Manager) FindFacesMissing(id string) {
	if !m.FacesInstalled() || m.IsScanning(id) {
		return
	}
	v, running := m.faces.running.LoadOrStore(id, new(atomic.Bool))
	again := v.(*atomic.Bool)
	if running {
		again.Store(true)
		return
	}
	defer m.faces.running.Delete(id)
	for {
		m.findFacesOnce(id)
		if !again.Swap(false) {
			return
		}
	}
}

// findFacesOnce runs one pass. Nothing shows in the status line when there
// is nothing to look at.
func (m *Manager) findFacesOnce(id string) {
	store, err := m.OpenStore(id)
	if err != nil {
		return
	}
	if tasks, err := store.PhotosNeedingFaces(); err != nil || len(tasks) == 0 {
		return
	}
	job := m.jobs.Start("library", m.scanTitle(id, "Finding faces in"), "people")
	engine, err := m.faceEngine()
	if err != nil {
		job.Finish(err)
		return
	}
	ch := make(chan Progress, 8)
	go NewIndexer(store, m.LibDir(id), "").RunFindFacesMissing(context.Background(), engine, ch)
	var last Progress
	for p := range ch {
		job.Progress(p.Done, p.Total, "")
		last = p
	}
	if last.Error != "" {
		job.Finish(errors.New(last.Error))
		return
	}
	job.Finish(nil)
}

// FindingFaces reports whether a face pass runs for the library.
func (m *Manager) FindingFaces(id string) bool {
	_, ok := m.faces.running.Load(id)
	return ok
}
