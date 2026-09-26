package library

import (
	"testing"

	"huepattl.de/unterlumen/internal/jobs"
)

// lastEvent sends progress to a subscriber that reads nothing until the scan
// is over — a slow HTTP client — and returns the last event it then gets.
func lastEvent(t *testing.T, final Progress) (Progress, int) {
	t.Helper()
	b := newBroadcaster()
	b.job = jobs.NewRegistry().Start("library", "Scanning", "libraries")
	ch := b.Subscribe()
	for i := 0; i < 30; i++ {
		b.Send(Progress{Done: i, Total: 30, Current: "p.jpg"})
	}
	b.Send(final)
	var last Progress
	n := 0
	for p := range ch {
		last = p
		n++
	}
	return last, n
}

// Progress in between may be dropped for a slow subscriber, but its last word
// may not: without it the stream ends as if the scan had been cut short.
func TestBroadcasterDeliversFinishedToASlowSubscriber(t *testing.T) {
	last, n := lastEvent(t, Progress{Done: 30, Total: 30, Finished: true})
	if !last.Finished {
		t.Fatalf("last of %d events = %+v, want the finished event", n, last)
	}
}

func TestBroadcasterDeliversAnErrorToASlowSubscriber(t *testing.T) {
	last, n := lastEvent(t, Progress{Done: 3, Total: 30, Error: "disk full"})
	if last.Error != "disk full" {
		t.Fatalf("last of %d events = %+v, want the error", n, last)
	}
}
