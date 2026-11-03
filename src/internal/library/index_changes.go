package library

import (
	"strconv"
	"strings"
	"sync"
)

// changesOf is how often the topic's tables ("people": faces and people;
// "things": photo_things) have changed in each library, as one key part.
// Triggers count every change (store.go), so no writer has to remember to.
func (m *Manager) changesOf(libs []*Library, topic string) string {
	parts := make([]string, len(libs))
	for i, l := range libs {
		parts[i] = "-"
		store, err := m.OpenReader(l.ID)
		if err != nil {
			continue
		}
		var n int64
		if store.db.QueryRow(`SELECT n FROM index_changes WHERE topic = ?`, topic).Scan(&n) == nil {
			parts[i] = strconv.FormatInt(n, 10)
		} else {
			parts[i] = "0"
		}
	}
	return strings.Join(parts, ",")
}

// changing is a cached value with the changes it was read at.
type changing struct {
	changes string
	value   any
}

// loadChanging returns the value cached under key if it was read at changes.
func loadChanging(cache *sync.Map, key, changes string) (any, bool) {
	v, ok := cache.Load(key)
	if !ok || v.(changing).changes != changes {
		return nil, false
	}
	return v.(changing).value, true
}

func storeChanging(cache *sync.Map, key, changes string, value any) {
	cache.Store(key, changing{changes, value})
}
