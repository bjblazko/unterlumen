package library

import (
	"path/filepath"
	"sync"

	"huepattl.de/unterlumen/internal/jobs"
)

// An arrival is a file that was copied or moved into a library's folder. A
// moved one carries the path it came from.
type arrival struct {
	path, from string
}

// arrivalQueue holds what still has to go into one library's index. One
// worker drains it under one job, so files sent in several requests show as
// one count in the status line that grows as they come.
type arrivalQueue struct {
	mu          sync.Mutex
	pending     []arrival
	job         *jobs.Handle
	done, total int
}

// AddMovedFiles puts files moved into the library (as [old, new] absolute
// paths) into its index in the background and returns the ID of the job
// that shows it. A photo the library knew at its old path keeps what it knew
// and is not read again; the old path is forgotten only after the new one is
// in, so a move within the library never loses the photo's faces or things.
func (m *Manager) AddMovedFiles(id string, moved [][2]string) string {
	arrivals := make([]arrival, len(moved))
	for i, pair := range moved {
		arrivals[i] = arrival{path: pair[1], from: pair[0]}
	}
	return m.queueArrivals(id, arrivals)
}

// AddCopiedFiles puts files copied into the library into its index in the
// background and returns the ID of the job that shows it.
func (m *Manager) AddCopiedFiles(id string, paths []string) string {
	arrivals := make([]arrival, len(paths))
	for i, p := range paths {
		arrivals[i] = arrival{path: p}
	}
	return m.queueArrivals(id, arrivals)
}

func (m *Manager) queueArrivals(id string, arrivals []arrival) string {
	v, _ := m.arrivals.LoadOrStore(id, &arrivalQueue{})
	q := v.(*arrivalQueue)
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending = append(q.pending, arrivals...)
	q.total += len(arrivals)
	if q.job == nil {
		q.job = m.jobs.Start("library", m.scanTitle(id, "Adding photos to"), "libraries")
		if q.job == nil {
			q.job = &jobs.Handle{} // no register: a handle that reports nowhere
		}
		go m.drainArrivals(id, q)
	}
	q.job.Progress(q.done, q.total, "")
	return q.job.ID()
}

// drainArrivals indexes the queue's files one by one until it is empty,
// then ends the job.
func (m *Manager) drainArrivals(id string, q *arrivalQueue) {
	idx := m.arrivalIndexer(id)
	for {
		a, ok := q.next()
		if !ok {
			break
		}
		if idx != nil {
			idx.addArrival(a)
		}
		if a.from != "" {
			m.ForgetFiles([]string{a.from})
		}
		q.advance(filepath.Base(a.path))
	}
	m.InvalidateStatsCache(id)
	go func() {
		m.AnalyseMissing(id)
		m.FindFacesMissing(id)
		m.FindThingsMissing(id)
	}()
}

func (m *Manager) arrivalIndexer(id string) *Indexer {
	store, err := m.OpenStore(id)
	if err != nil {
		return nil
	}
	libInfo, err := libraryFromStore(id, store)
	if err != nil || libInfo.SourcePath == "" {
		return nil
	}
	return NewIndexer(store, m.LibDir(id), libInfo.SourcePath)
}

func (idx *Indexer) addArrival(a arrival) {
	if a.from != "" {
		_ = idx.store.MovePathCache(a.from, a.path)
	}
	_ = idx.IndexFile(a.path)
}

// next takes the next file; when there is none it ends the job, under the
// same lock that queueArrivals starts one, so no file is left behind.
func (q *arrivalQueue) next() (arrival, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		q.job.Finish(nil)
		q.job, q.done, q.total = nil, 0, 0
		return arrival{}, false
	}
	a := q.pending[0]
	q.pending = q.pending[1:]
	return a, true
}

func (q *arrivalQueue) advance(current string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.done++
	q.job.Progress(q.done, q.total, current)
}
