package library

import (
	"errors"
	"os"
	"sync"
)

// --- Removing the models and the runtime (Settings → Storage) ---

// ErrModelsBusy: a pass over a library uses the models right now.
var ErrModelsBusy = errors.New("a library is being looked through with them right now; remove them when it is done")

// ErrRuntimeNeeded: face recognition or things still run on ONNX Runtime.
var ErrRuntimeNeeded = errors.New("face recognition or things still need it; remove those first")

// RemoveRecognition removes the face models. The faces found stay in the
// libraries; new photos are not looked at until they are installed again.
func (m *Manager) RemoveRecognition() error {
	return m.removeModels(&m.faces.mu, &m.faces.running, func() { m.faces.engine = nil }, m.RecognitionDir())
}

// RemoveThings removes the things models; what was found stays.
func (m *Manager) RemoveThings() error {
	return m.removeModels(&m.things.mu, &m.things.running, func() { m.things.look = nil }, m.ThingsDir())
}

// RemoveRuntime removes ONNX Runtime once neither model needs it.
func (m *Manager) RemoveRuntime() error {
	if m.FacesInstalled() || m.ThingsInstalled() {
		return ErrRuntimeNeeded
	}
	return os.RemoveAll(m.RuntimeDir())
}

// removeModels forgets the loaded engine and removes its folder, unless a
// pass is running. The engine is dropped, not closed: a request may still
// hold it, and its memory goes with the process.
func (m *Manager) removeModels(mu *sync.Mutex, running *sync.Map, forget func(), dir string) error {
	mu.Lock()
	defer mu.Unlock()
	busy := false
	running.Range(func(_, _ any) bool { busy = true; return false })
	if busy {
		return ErrModelsBusy
	}
	forget()
	return os.RemoveAll(dir)
}
