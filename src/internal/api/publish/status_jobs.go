package publish

import (
	"errors"
	"fmt"
	"net/http"

	apijobs "huepattl.de/unterlumen/internal/api/jobs"
	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/jobs"
	lib "huepattl.de/unterlumen/internal/library"
)

// Work on a destination's galleries that runs within one request, reported to
// the status line with the destination's name (ADR-0036).
func trackDestination(mgr *lib.Manager, chStore *channels.Store, verb string, h http.HandlerFunc) http.HandlerFunc {
	return apijobs.Track(mgr.Jobs(), "site", "galleries", func(r *http.Request) string {
		return fmt.Sprintf("%s %q", verb, destinationName(chStore, r.PathValue("slug")))
	}, h)
}

func destinationName(chStore *channels.Store, slug string) string {
	if ch, err := chStore.Get(slug); err == nil && ch.Name != "" {
		return ch.Name
	}
	return slug
}

// publishReporter mirrors the events of a publish run in the status line.
// Only the photo step has a count; the others are one piece of work each.
type publishReporter struct {
	job  *jobs.Handle
	step string
}

func (p *publishReporter) report(ev map[string]any) {
	if msg, ok := ev["error"].(string); ok {
		p.job.Finish(errors.New(msg))
		return
	}
	if ev["complete"] == true {
		p.job.Finish(nil)
		return
	}
	step, _ := ev["step"].(string)
	if step != p.step {
		p.step = step
		p.job.Step(step)
	}
	if step == "photo" {
		done, _ := ev["done"].(int)
		total, _ := ev["total"].(int)
		file, _ := ev["file"].(string)
		p.job.Progress(done, total, file)
	}
}
