package jobs

import "sync"

// Subscription receives every change to a job. Changes to the same job are
// merged while the subscriber is busy, so a slow reader sees fewer steps but
// always the latest state, and never misses how a job ended.
type Subscription struct {
	reg     *Registry
	notify  chan struct{}
	mu      sync.Mutex
	pending map[string]Job
	order   []string
}

// Subscribe returns the jobs as they are now and a subscription for what
// changes from here on. Close the subscription when done.
func (r *Registry) Subscribe() ([]Job, *Subscription) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dropExpired()
	s := &Subscription{reg: r, notify: make(chan struct{}, 1), pending: map[string]Job{}}
	r.subs[s] = struct{}{}
	return r.snapshotLocked(), s
}

// Ready is signalled when there are changes to Take.
func (s *Subscription) Ready() <-chan struct{} { return s.notify }

// Take returns the changes since the last call, in the order the jobs first
// changed, each in its latest state.
func (s *Subscription) Take() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Job, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.pending[id])
	}
	s.pending = map[string]Job{}
	s.order = nil
	return out
}

// Close ends the subscription.
func (s *Subscription) Close() {
	s.reg.mu.Lock()
	defer s.reg.mu.Unlock()
	delete(s.reg.subs, s)
}

func (s *Subscription) offer(j Job) {
	s.mu.Lock()
	if _, seen := s.pending[j.ID]; !seen {
		s.order = append(s.order, j.ID)
	}
	s.pending[j.ID] = j
	s.mu.Unlock()
	select {
	case s.notify <- struct{}{}:
	default: // already signalled
	}
}

// publish hands a change to every subscriber. The caller holds r.mu.
func (r *Registry) publish(j Job) {
	for s := range r.subs {
		s.offer(j)
	}
}
