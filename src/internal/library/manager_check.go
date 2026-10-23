package library

import (
	"context"
	"time"
)

// sharedCheckInterval is how often a shared library is compared with its
// folder. Checking a library whose files did not change costs one look at
// each file: seconds on the NAS's own disk, about a minute per 10 000
// photos from a Mac over SMB.
const sharedCheckInterval = 30 * time.Minute

// KeepSharedLibrariesCurrent checks every shared library against its folder
// now and then every sharedCheckInterval, so what the other installation
// changed — photos added, deleted, edited, titled, published or taken out of
// a gallery — shows here without a click. A check only reads the folder;
// the index follows it.
// It returns when ctx ends.
func (m *Manager) KeepSharedLibrariesCurrent(ctx context.Context) {
	for {
		// What the other installation hid, refused or chose as a cover:
		// first, as it takes moments where a check may take minutes; each
		// check applies it again for its library when it ends (EndScan).
		m.ApplyAllFaceDecisions()
		m.checkSharedLibraries(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(sharedCheckInterval):
		}
	}
}

func (m *Manager) checkSharedLibraries(ctx context.Context) {
	libs, err := m.ListLibraries()
	if err != nil {
		return
	}
	for _, l := range libs {
		if ctx.Err() != nil {
			return
		}
		if mk, err := ReadMarker(l.SourcePath); err == nil && mk != nil && mk.ID == l.ID {
			m.checkLibrary(ctx, l)
		}
	}
}

// checkLibrary runs a full incremental index of the library — new and
// changed files read, gone ones removed — in the status line as "Checking".
// A library already being scanned is left to that scan.
func (m *Manager) checkLibrary(ctx context.Context, l *Library) {
	b, started := m.StartScan(l.ID, "Checking")
	if !started {
		return
	}
	defer m.EndScan(l.ID)
	defer b.Close()
	store, err := m.OpenStore(l.ID)
	if err != nil {
		return
	}
	progress := make(chan Progress, 8)
	go NewIndexer(store, m.LibDir(l.ID), l.SourcePath).Run(ctx, progress)
	for p := range progress {
		b.Send(p)
	}
}
