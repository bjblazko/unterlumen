package library

import (
	"context"
	"fmt"
)

// TriggerScanNewBackground starts an incremental scan for the library in a background
// goroutine. It is a no-op when a scan is already running for that library.
func (m *Manager) TriggerScanNewBackground(id string) {
	b, started := m.StartScan(id, "Scanning")
	if !started {
		return
	}
	store, err := m.OpenStore(id)
	if err != nil {
		m.EndScan(id)
		b.Close()
		return
	}
	libInfo, err := libraryFromStore(id, store)
	if err != nil || libInfo.SourcePath == "" {
		m.EndScan(id)
		b.Close()
		return
	}
	rawCh := make(chan Progress, 8)
	go func() {
		defer b.Close()
		for p := range rawCh {
			b.Send(p)
		}
	}()
	go func() {
		defer m.EndScan(id)
		idx := NewIndexer(store, m.LibDir(id), libInfo.SourcePath)
		idx.RunScanNew(context.Background(), rawCh)
	}()
}

// IndexFilesSync synchronously indexes the given absolute file paths in the named
// library. Returns true if at least one new photo was indexed. Errors per file are
// silently skipped so partial results are still returned.
// IndexFilesSync indexes absPaths into the library and reports whether the
// caller should refresh any view of this library. This covers both brand-new
// photos and existing ones whose path_hint just changed (e.g. a rename or a
// move within the same library) — NewPhotos alone would miss the latter and
// leave callers thinking nothing happened when a file's path was in fact
// updated in the database.
func (m *Manager) IndexFilesSync(id string, absPaths []string) bool {
	store, err := m.OpenStore(id)
	if err != nil {
		return false
	}
	libInfo, err := libraryFromStore(id, store)
	if err != nil || libInfo.SourcePath == "" {
		return false
	}
	idx := NewIndexer(store, m.LibDir(id), libInfo.SourcePath)
	for _, p := range absPaths {
		_ = idx.IndexFile(p)
	}
	return len(absPaths) > 0
}

// TriggerScanNewInFolderBackground is like TriggerScanNewBackground but scoped to
// a single subfolder (relative to the library source path). A no-op if a scan is
// already running for this library.
func (m *Manager) TriggerScanNewInFolderBackground(id, subfolder string) {
	b, started := m.StartScan(id, "Scanning")
	if !started {
		return
	}
	store, err := m.OpenStore(id)
	if err != nil {
		m.EndScan(id)
		b.Close()
		return
	}
	libInfo, err := libraryFromStore(id, store)
	if err != nil || libInfo.SourcePath == "" {
		m.EndScan(id)
		b.Close()
		return
	}
	rawCh := make(chan Progress, 8)
	go func() {
		defer b.Close()
		for p := range rawCh {
			b.Send(p)
		}
	}()
	go func() {
		defer m.EndScan(id)
		idx := NewIndexer(store, m.LibDir(id), libInfo.SourcePath)
		idx.RunScanNewInFolder(context.Background(), rawCh, subfolder)
	}()
}

// TriggerCleanupInFolderBackground marks photos in subfolder as missing when their
// source files no longer exist. Used after moves to clean up the source library.
// A no-op if a scan is already running for this library.
func (m *Manager) TriggerCleanupInFolderBackground(id, subfolder string) {
	b, started := m.StartScan(id, "Checking")
	if !started {
		return
	}
	store, err := m.OpenStore(id)
	if err != nil {
		m.EndScan(id)
		b.Close()
		return
	}
	libInfo, err := libraryFromStore(id, store)
	if err != nil || libInfo.SourcePath == "" {
		m.EndScan(id)
		b.Close()
		return
	}
	rawCh := make(chan Progress, 8)
	go func() {
		defer b.Close()
		for p := range rawCh {
			b.Send(p)
		}
	}()
	go func() {
		defer m.EndScan(id)
		idx := NewIndexer(store, m.LibDir(id), libInfo.SourcePath)
		idx.RunCleanupInFolder(context.Background(), rawCh, subfolder)
	}()
}

// TryLockIndex acquires the indexing lock for a library.
// Returns true if the lock was acquired (not already indexing).
func (m *Manager) TryLockIndex(id string) bool {
	_, loaded := m.indexMu.LoadOrStore(id, true)
	return !loaded
}

// UnlockIndex releases the indexing lock for a library.
func (m *Manager) UnlockIndex(id string) {
	m.indexMu.Delete(id)
}

// StartScan acquires the index lock and registers a broadcaster for the library.
// Returns the broadcaster and true on success, or nil and false if already scanning.
// verb says what the scan does ("Scanning", "Indexing"); with the library's
// name it is the job's title in the status line.
func (m *Manager) StartScan(id, verb string) (*Broadcaster, bool) {
	if !m.TryLockIndex(id) {
		return nil, false
	}
	m.InvalidateStatsCache(id)
	b := newBroadcaster()
	b.job = m.jobs.Start("library", m.scanTitle(id, verb), "libraries")
	m.scans.Store(id, b)
	return b, true
}

func (m *Manager) scanTitle(id, verb string) string {
	if l, err := m.readLibrary(id); err == nil && l.Name != "" {
		return fmt.Sprintf("%s %q", verb, l.Name)
	}
	return verb + " a library"
}

// JoinScan returns the active broadcaster for the library, if any.
func (m *Manager) JoinScan(id string) (*Broadcaster, bool) {
	if v, ok := m.scans.Load(id); ok {
		return v.(*Broadcaster), true
	}
	return nil, false
}

// EndScan removes the broadcaster and releases the index lock, then
// measures the appearance of photos the scan added or changed.
// The broadcaster itself must be closed separately (by the bridge goroutine).
func (m *Manager) EndScan(id string) {
	m.scans.Delete(id)
	m.UnlockIndex(id)
	m.InvalidateStatsCache(id)
	go m.prewarmFolderStats(id)
	go m.AnalyseMissing(id)
}

// IsScanning reports whether a scan is currently active for the library.
func (m *Manager) IsScanning(id string) bool {
	_, ok := m.scans.Load(id)
	return ok
}
