package library

import (
	"testing"

	"huepattl.de/unterlumen/internal/jobs"
)

func scanJob(t *testing.T, events ...Progress) jobs.Job {
	t.Helper()
	reg := jobs.NewRegistry()
	b := newBroadcaster()
	b.job = reg.Start("library", "Scanning", "libraries")
	for _, p := range events {
		b.Send(p)
	}
	b.Close()
	return reg.Snapshot()[0]
}

func TestBroadcasterReportsFinishedScan(t *testing.T) {
	j := scanJob(t,
		Progress{Done: 1, Total: 2, Current: "a.jpg", Parent: "2026"},
		Progress{Done: 2, Total: 2, Finished: true},
	)
	if !j.Finished || j.Error != "" || j.Done != 2 {
		t.Fatalf("want finished at 2 without error, got %+v", j)
	}
}

// An error arrives with Finished set; it must not read as a success.
func TestBroadcasterReportsScanError(t *testing.T) {
	j := scanJob(t, Progress{Error: "source folder missing", Finished: true})
	if j.Error != "source folder missing" {
		t.Fatalf("want the scan's error, got %+v", j)
	}
}

func TestBroadcasterReportsInterruptedScan(t *testing.T) {
	j := scanJob(t, Progress{Done: 1, Total: 5, Current: "a.jpg"})
	if !j.Finished || j.Error == "" {
		t.Fatalf("a scan closed without a last word must end as failed, got %+v", j)
	}
}

func TestBroadcasterShowsCurrentWithFolder(t *testing.T) {
	reg := jobs.NewRegistry()
	b := newBroadcaster()
	b.job = reg.Start("library", "Scanning", "libraries")
	b.Send(Progress{Done: 1, Total: 5, Current: "a.jpg", Parent: "2026"})
	if got := reg.Snapshot()[0].Current; got != "2026/a.jpg" {
		t.Fatalf("want 2026/a.jpg, got %q", got)
	}
}
