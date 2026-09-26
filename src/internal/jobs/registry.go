// Package jobs keeps a register of the long-running work the server is doing
// (library scans, exports, publishing, site rebuilds, deploys), so the UI can
// show it in one place even after the page that started it is gone
// (ADR-0036).
package jobs

import (
	"sort"
	"strconv"
	"sync"
	"time"
)

// keepFinished is how long a finished job stays in the register, so a page
// that reconnects shortly after still learns how it ended.
const keepFinished = 60 * time.Second

// Job is the state of one piece of work, as the UI sees it.
type Job struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`  // library, export, publish, site, deploy
	Title string `json:"title"` // what is happening, in a sentence: `Scanning "Archive"`
	Place string `json:"place"` // the place in the UI it belongs to: libraries, galleries, destinations
	Step  string `json:"step,omitempty"`

	// Done and Total count the items of the current step; Total is 0 while
	// the amount of work is not known.
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Current string `json:"current,omitempty"`

	Started  time.Time  `json:"started"`
	Finished bool       `json:"finished"`
	Ended    *time.Time `json:"ended,omitempty"`
	Error    string     `json:"error,omitempty"`

	seq int // start order, for jobs started within the same clock tick
}

// Registry holds the jobs and tells subscribers when one changes.
// It is safe for concurrent use.
type Registry struct {
	mu     sync.Mutex
	jobs   map[string]*Job
	subs   map[*Subscription]struct{}
	nextID int
	now    func() time.Time
}

// NewRegistry returns an empty register.
func NewRegistry() *Registry {
	return &Registry{
		jobs: map[string]*Job{},
		subs: map[*Subscription]struct{}{},
		now:  time.Now,
	}
}

// Handle is how the code doing the work reports on it. A nil Handle is valid
// and does nothing, so callers need no checks when no register is wired.
type Handle struct {
	reg *Registry
	id  string
}

// Start registers a new job and returns its handle.
func (r *Registry) Start(kind, title, place string) *Handle {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dropExpired()
	r.nextID++
	id := "j" + strconv.Itoa(r.nextID)
	j := &Job{ID: id, Kind: kind, Title: title, Place: place, Started: r.now(), seq: r.nextID}
	r.jobs[id] = j
	r.publish(*j)
	return &Handle{reg: r, id: id}
}

// Progress reports how far the current step is.
func (h *Handle) Progress(done, total int, current string) {
	h.update(func(j *Job) {
		j.Done, j.Total, j.Current = done, total, current
	})
}

// Step starts a new named step; its count starts again from zero.
func (h *Handle) Step(step string) {
	h.update(func(j *Job) {
		j.Step, j.Done, j.Total, j.Current = step, 0, 0, ""
	})
}

// Finish ends the job; a non-nil err says why it stopped. Only the first
// call counts.
func (h *Handle) Finish(err error) {
	h.update(func(j *Job) {
		j.Finished = true
		ended := h.reg.now()
		j.Ended = &ended
		j.Current = ""
		if err != nil {
			j.Error = err.Error()
		}
	})
}

func (h *Handle) update(change func(*Job)) {
	if h == nil || h.reg == nil {
		return
	}
	r := h.reg
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[h.id]
	// A finished job is final: late progress or a second Finish changes nothing.
	if !ok || j.Finished {
		return
	}
	change(j)
	r.publish(*j)
}

// Snapshot returns every job in the register, oldest first.
func (r *Registry) Snapshot() []Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dropExpired()
	return r.snapshotLocked()
}

func (r *Registry) snapshotLocked() []Job {
	out := make([]Job, 0, len(r.jobs))
	for _, j := range r.jobs {
		out = append(out, *j)
	}
	sortByStart(out)
	return out
}

func (r *Registry) dropExpired() {
	cutoff := r.now().Add(-keepFinished)
	for id, j := range r.jobs {
		if j.Finished && j.Ended != nil && j.Ended.Before(cutoff) {
			delete(r.jobs, id)
		}
	}
}

func sortByStart(js []Job) {
	sort.Slice(js, func(a, b int) bool { return js[a].seq < js[b].seq })
}
