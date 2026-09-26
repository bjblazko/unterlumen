package jobs

import (
	"errors"
	"testing"
	"time"
)

func TestStartProgressFinish(t *testing.T) {
	r := NewRegistry()
	h := r.Start("library", `Scanning "Archive"`, "libraries")
	h.Progress(3, 10, "a.jpg")
	h.Finish(nil)

	js := r.Snapshot()
	if len(js) != 1 {
		t.Fatalf("want 1 job, got %d", len(js))
	}
	j := js[0]
	if !j.Finished || j.Error != "" || j.Done != 3 || j.Total != 10 || j.Current != "" {
		t.Fatalf("unexpected final state: %+v", j)
	}
}

func TestFinishedJobIgnoresLateChanges(t *testing.T) {
	r := NewRegistry()
	h := r.Start("deploy", "Deploying", "destinations")
	h.Finish(errors.New("host unreachable"))
	h.Progress(5, 5, "late")
	h.Finish(nil)

	j := r.Snapshot()[0]
	if j.Error != "host unreachable" || j.Done != 0 {
		t.Fatalf("finished job changed after Finish: %+v", j)
	}
}

func TestStepResetsCount(t *testing.T) {
	r := NewRegistry()
	h := r.Start("publish", "Publishing", "galleries")
	h.Progress(4, 4, "x.jpg")
	h.Step("html")
	j := r.Snapshot()[0]
	if j.Step != "html" || j.Done != 0 || j.Total != 0 || j.Current != "" {
		t.Fatalf("step did not reset the count: %+v", j)
	}
}

func TestNilHandleIsSafe(t *testing.T) {
	var r *Registry
	h := r.Start("export", "Exporting", "")
	h.Progress(1, 2, "")
	h.Step("zip")
	h.Finish(nil)
}

func TestSubscriberGetsSnapshotThenChanges(t *testing.T) {
	r := NewRegistry()
	first := r.Start("library", "Scanning", "libraries")
	snap, sub := r.Subscribe()
	defer sub.Close()
	if len(snap) != 1 || snap[0].ID != "j1" {
		t.Fatalf("snapshot should hold the running job, got %+v", snap)
	}

	first.Progress(1, 2, "a")
	<-sub.Ready()
	got := sub.Take()
	if len(got) != 1 || got[0].Done != 1 {
		t.Fatalf("want one change with done=1, got %+v", got)
	}
}

// A subscriber that does not read for a while sees each job once, in its
// latest state — and the end of a job is never lost.
func TestSlowSubscriberSeesLatestAndEnd(t *testing.T) {
	r := NewRegistry()
	_, sub := r.Subscribe()
	defer sub.Close()

	a := r.Start("library", "A", "libraries")
	b := r.Start("export", "B", "")
	for i := 1; i <= 500; i++ {
		a.Progress(i, 500, "")
	}
	a.Finish(nil)
	b.Progress(1, 3, "")

	got := sub.Take()
	if len(got) != 2 {
		t.Fatalf("want 2 merged changes, got %d", len(got))
	}
	if got[0].ID != "j1" || !got[0].Finished || got[0].Done != 500 {
		t.Fatalf("first job should arrive finished at 500, got %+v", got[0])
	}
	if got[1].ID != "j2" || got[1].Done != 1 {
		t.Fatalf("second job should arrive at 1, got %+v", got[1])
	}
	if more := sub.Take(); len(more) != 0 {
		t.Fatalf("Take should empty the queue, got %+v", more)
	}
}

func TestClosedSubscriptionGetsNothing(t *testing.T) {
	r := NewRegistry()
	_, sub := r.Subscribe()
	sub.Close()
	r.Start("library", "A", "libraries")
	if got := sub.Take(); len(got) != 0 {
		t.Fatalf("closed subscription received %+v", got)
	}
}

func TestFinishedJobsExpire(t *testing.T) {
	r := NewRegistry()
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }

	r.Start("library", "done", "libraries").Finish(nil)
	r.Start("library", "running", "libraries")

	now = now.Add(keepFinished + time.Second)
	js := r.Snapshot()
	if len(js) != 1 || js[0].Title != "running" {
		t.Fatalf("finished job should expire, running one stay: %+v", js)
	}
}

func TestSnapshotIsInStartOrder(t *testing.T) {
	r := NewRegistry()
	for _, title := range []string{"a", "b", "c", "d"} {
		r.Start("library", title, "libraries")
	}
	js := r.Snapshot()
	for i, want := range []string{"a", "b", "c", "d"} {
		if js[i].Title != want {
			t.Fatalf("position %d: want %s, got %s", i, want, js[i].Title)
		}
	}
}
