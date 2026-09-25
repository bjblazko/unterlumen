package apilibrary

import (
	"os"
	"path/filepath"
	"time"

	"huepattl.de/unterlumen/internal/channels"
)

// MarkDeployed records that a channel's generated galleries were uploaded at
// `at`, so the frontend can tell "built, not uploaded" from "online"
// (ADR-0029). Deploy is per channel — rsync pushes the whole output directory
// — so every gallery in it is stamped with the same time.
//
// A site-export channel deploys only its site/ subdirectory, so only its site
// albums are stamped; any single-gallery folders in the same output directory
// were not part of that upload and keep their old (or zero) DeployedAt.
//
// Write errors are deliberately tolerated per gallery: the upload itself
// already happened, and a statefile that could not be updated must not turn a
// successful deploy into a failure. The worst case is a gallery that keeps
// saying "built, not uploaded" until the next publish.
func MarkDeployed(chStore *channels.Store, slug string, siteExport bool, at time.Time) {
	if siteExport {
		store := newSiteStore(chStore, slug)
		albums, err := store.List()
		if err != nil {
			return
		}
		for _, a := range albums {
			a.DeployedAt = at
			store.Upsert(a) //nolint:errcheck
		}
		return
	}
	channelDir := chStore.OutputDir(slug)

	entries, err := os.ReadDir(channelDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		statePath := filepath.Join(channelDir, e.Name(), "gallery.json")
		gs, gsErr := loadGalleryState(statePath)
		if gsErr != nil || gs == nil {
			continue
		}
		gs.DeployedAt = at
		saveGalleryState(statePath, gs) //nolint:errcheck
	}
}
