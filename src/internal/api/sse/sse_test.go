package sse

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStartSendsEventStreamHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	if _, ok := Start(rec); !ok {
		t.Fatal("Start refused a flushing writer")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	for k, want := range map[string]string{"Content-Type": "text/event-stream", "Cache-Control": "no-cache", "X-Accel-Buffering": "no"} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

// noFlush hides httptest.ResponseRecorder's Flush method.
type noFlush struct{ http.ResponseWriter }

func TestStartRefusesAWriterThatCannotFlush(t *testing.T) {
	rec := httptest.NewRecorder()
	if _, ok := Start(noFlush{rec}); ok {
		t.Fatal("Start accepted a writer that cannot flush")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestSendWritesOneDataEvent(t *testing.T) {
	var b strings.Builder
	if err := Send(&b, map[string]any{"step": "zip", "done": 1}); err != nil {
		t.Fatal(err)
	}
	if want := "data: {\"done\":1,\"step\":\"zip\"}\n\n"; b.String() != want {
		t.Errorf("event = %q, want %q", b.String(), want)
	}
}

func TestSendReportsAValueThatIsNotJSON(t *testing.T) {
	var b strings.Builder
	if err := Send(&b, make(chan int)); err == nil {
		t.Error("Send accepted a channel")
	}
	if b.Len() != 0 {
		t.Errorf("wrote %q for a failed event", b.String())
	}
}
