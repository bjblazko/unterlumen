package library

import (
	"context"
	"sync"
)

// Statistics, read before anybody asks (ADR-0063): after a library's scan
// and what follows it (appearance, faces, things), the overview and every
// topic of all libraries and of that library alone are worked out in the
// background, through the readers, so the first visit finds them ready.
// What is still cached costs nothing; the rest is read once.

// prewarming runs one prewarm at a time; a request while one runs asks for
// one more after it, so libraries finishing together (at start) are read
// once, not once each.
type prewarming struct {
	mu      sync.Mutex
	running bool
	again   map[string]bool
}

// PrewarmAllStatistics works out Statistics of every library, as after a
// start: nothing is cached then, and no scan may come for a while.
func (m *Manager) PrewarmAllStatistics() {
	libs, err := m.ListLibraries()
	if err != nil {
		return
	}
	m.prewarmStatistics(libraryIDs(libs)...)
}

// prewarmStatistics works out Statistics of all libraries and of each of ids.
func (m *Manager) prewarmStatistics(ids ...string) {
	p := &m.prewarm
	p.mu.Lock()
	if p.again == nil {
		p.again = map[string]bool{}
	}
	for _, id := range ids {
		p.again[id] = true
	}
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	for len(p.again) > 0 {
		ids := p.again
		p.again = map[string]bool{}
		p.mu.Unlock()
		scopes := [][]string{nil}
		for one := range ids {
			scopes = append(scopes, []string{one})
		}
		for _, ids := range scopes {
			m.readOverview(ids)
		}
		p.mu.Lock()
	}
	p.running = false
	p.mu.Unlock()
}

// readOverview reads what the Statistics overview and every topic ask for,
// as they ask: no folder, no filter, the periods chosen by the span.
func (m *Manager) readOverview(ids []string) {
	f := Filter{}
	m.Statistics(ids, "", f)        //nolint:errcheck
	m.Timeline(ids, "", "", f)      //nolint:errcheck
	m.Colour(ids, "", "", f)        //nolint:errcheck
	m.ColourSpace(ids, "", "", f)   //nolint:errcheck
	m.ExposureSpace(ids, "", "", f) //nolint:errcheck
	m.SpaceTime(ids, "", f)         //nolint:errcheck
	m.PeopleStats(ids, "", "", f)   //nolint:errcheck
	m.ThingsStats(ids, "", "", f)   //nolint:errcheck
	m.ScopeValues(ids, nil)         //nolint:errcheck
	for group := range insightGroups {
		m.Insights(ids, "", group, f) //nolint:errcheck
	}
	for _, view := range []string{"palette", "meaning"} {
		m.Space(context.Background(), ids, "", view, f) //nolint:errcheck
	}
}
