package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"

	"huepattl.de/unterlumen/internal/things"
)

// thingsState holds the things engine and the word list's vectors, loaded
// on first use from the things folder, and which libraries a things pass
// runs for.
type thingsState struct {
	mu      sync.Mutex
	look    *thingsLook
	running sync.Map   // map[libraryID]*atomic.Bool — as Manager.analysing
	pass    sync.Mutex // one pass at a time, whichever library: each runs the models on every core it gets
}

// ThingsDir is where this installation keeps the things models (ADR-0054).
func (m *Manager) ThingsDir() string {
	return filepath.Join(m.root, "things")
}

// ThingsInstalled reports whether things can be found here.
func (m *Manager) ThingsInstalled() bool {
	return m.runtimeInstalled() && things.Installed(m.ThingsDir())
}

// ThingsOutdated reports whether the things folder holds models of an
// earlier version — CLIP before SigLIP 2 — that a download replaces.
func (m *Manager) ThingsOutdated() bool {
	if _, err := os.Stat(m.ThingsDir()); err != nil {
		return false
	}
	return !things.Installed(m.ThingsDir())
}

// thingsLook loads the engine and describes the word list once the things
// folder is complete.
func (m *Manager) thingsLook() (*thingsLook, error) {
	m.things.mu.Lock()
	defer m.things.mu.Unlock()
	if m.things.look != nil {
		return m.things.look, nil
	}
	if !m.ThingsInstalled() {
		return nil, errors.New("things are not installed")
	}
	e, err := things.Open(m.runtimeLib(), m.ThingsDir(), 1)
	if err != nil {
		return nil, err
	}
	words, err := e.VocabularyVectors()
	if err != nil {
		e.Close()
		return nil, err
	}
	m.things.look = &thingsLook{engine: e, words: words}
	return m.things.look, nil
}

// FindAllThingsMissing runs a things pass for every library, one after
// another.
func (m *Manager) FindAllThingsMissing() {
	if !m.ThingsInstalled() {
		return
	}
	libs, err := m.ListLibraries()
	if err != nil {
		return
	}
	for _, l := range libs {
		m.FindThingsMissing(l.ID)
	}
}

// FindThingsMissing looks for things in a library's photos that the current
// models and word list did not look at, and returns when done. Like
// FindFacesMissing it holds no index lock, does not start during a scan, and
// runs once more when called while it runs.
func (m *Manager) FindThingsMissing(id string) {
	if !m.ThingsInstalled() || m.IsScanning(id) {
		return
	}
	v, running := m.things.running.LoadOrStore(id, new(atomic.Bool))
	again := v.(*atomic.Bool)
	if running {
		again.Store(true)
		return
	}
	defer m.things.running.Delete(id)
	for {
		m.findThingsOnce(id)
		if !again.Swap(false) {
			return
		}
	}
}

// findThingsOnce runs one pass. Nothing shows in the status line when there
// is nothing to look at.
func (m *Manager) findThingsOnce(id string) {
	store, err := m.OpenStore(id)
	if err != nil {
		return
	}
	if tasks, err := store.PhotosNeedingThings(); err != nil || len(tasks) == 0 {
		return
	}
	// Every check of every library asks for a pass; side by side they took
	// the NAS's cores between them and none came to an end.
	m.things.pass.Lock()
	defer m.things.pass.Unlock()
	job := m.jobs.Start("library", m.scanTitle(id, "Finding things in"), "statistics")
	ch := make(chan Progress, 8)
	go NewIndexer(store, m.LibDir(id), "").RunFindThingsMissing(context.Background(), m.thingsLook, ch)
	var last Progress
	for p := range ch {
		job.Progress(p.Done, p.Total, "")
		last = p
	}
	m.InvalidateStatsCache(id)
	if last.Error != "" {
		job.Finish(errors.New(last.Error))
		return
	}
	job.Finish(nil)
}

// FindingThings reports whether a things pass runs for the library.
func (m *Manager) FindingThings(id string) bool {
	_, ok := m.things.running.Load(id)
	return ok
}

// ThingTotal is a thing, its category and the number of photos it is in.
type ThingTotal struct {
	Thing    string `json:"thing"`
	Category string `json:"category"`
	Photos   int    `json:"photos"`
}

// ThingTotals lists the things found in the given libraries (every library
// when ids is empty) with their photos, most first, for the filter's Things.
func (m *Manager) ThingTotals(ids []string) ([]ThingTotal, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	photos := map[string]int{}
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		counts, err := store.ThingCounts()
		if err != nil {
			continue
		}
		for _, c := range counts {
			photos[c.Thing] += c.Photos
		}
	}
	out := make([]ThingTotal, 0, len(photos))
	for name, n := range photos {
		out = append(out, ThingTotal{Thing: name, Category: string(things.CategoryOf(name)), Photos: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Photos != out[j].Photos {
			return out[i].Photos > out[j].Photos
		}
		return out[i].Thing < out[j].Thing
	})
	return out, nil
}

// NearestThing is the thing whose words lie nearest to text, in any
// language SigLIP knows, and how likely it fits: for a word no list has —
// "Dackel" is nearest to dog. ok is false when none is near enough, or
// things are not installed here.
func (m *Manager) NearestThing(text string) (thing string, probability float32, ok bool) {
	if !m.ThingsInstalled() {
		return "", 0, false
	}
	look, err := m.thingsLook()
	if err != nil {
		return "", 0, false
	}
	v, err := look.engine.Describe(text)
	if err != nil {
		return "", 0, false
	}
	best, second := -1, -1
	for i, w := range look.words {
		switch s := things.Similarity(v, w); {
		case best < 0 || s > things.Similarity(v, look.words[best]):
			best, second = i, best
		case second < 0 || s > things.Similarity(v, look.words[second]):
			second = i
		}
	}
	if best < 0 || second < 0 {
		return "", 0, false
	}
	top, next := things.Similarity(v, look.words[best]), things.Similarity(v, look.words[second])
	if top < nearestMin || top-next < nearestAhead {
		return "", 0, false
	}
	return things.Vocabulary[best].Name, top, true
}

// Text to text, every thing's words alike begin "a photo of", so all lie
// near: words mean a thing when they are near it and nearer to it than to
// any other. "Dackel" is dog, "Rathaus" nothing in particular.
const (
	nearestMin   = 0.80
	nearestAhead = 0.01
)
