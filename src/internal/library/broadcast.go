package library

import "sync"

// Broadcaster fans out Progress events to multiple concurrent subscribers.
// It is safe for concurrent use.
type Broadcaster struct {
	mu   sync.Mutex
	subs []chan Progress
	last Progress
	done bool
}

func newBroadcaster() *Broadcaster { return &Broadcaster{} }

// Subscribe returns a channel that receives all future Progress events.
// If the scan is already done the returned channel is immediately closed.
// The last known Progress is sent first so the subscriber sees immediate state.
func (b *Broadcaster) Subscribe() <-chan Progress {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Progress, 16)
	if b.done {
		close(ch)
		return ch
	}
	if b.last.Total > 0 {
		ch <- b.last
	}
	b.subs = append(b.subs, ch)
	return ch
}

// Send broadcasts p to all current subscribers.
func (b *Broadcaster) Send(p Progress) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.last = p
	for _, ch := range b.subs {
		select {
		case ch <- p:
		default: // slow subscriber; drop rather than block the scan
		}
	}
	if p.Finished || p.Error != "" {
		b.closeAll()
	}
}

// Close marks the broadcaster done and closes all subscriber channels.
// Safe to call multiple times (used as a safety net for interrupted scans).
func (b *Broadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closeAll()
}

func (b *Broadcaster) closeAll() {
	if b.done {
		return
	}
	b.done = true
	for _, ch := range b.subs {
		close(ch)
	}
	b.subs = nil
}
