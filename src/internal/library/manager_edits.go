package library

// MetadataWritten brings what was written about the file at absPath into
// every library that covers it: the file is indexed again — a new ID when
// its content changed, the sidecar's or NFO's fields when only they did —
// and a title written to the sidecar becomes the photo's title note, which
// a scan would otherwise only add, never remove, in a library whose notes
// are not yet in the sidecars (ADR-0048, ADR-0060). It returns the photo's
// ID in each library afterwards.
func (m *Manager) MetadataWritten(absPath string, title *string) map[string]string {
	ids := map[string]string{}
	for _, lib := range m.LibrariesForPath(absPath) {
		m.IndexFilesSync(lib.ID, []string{absPath})
		store, err := m.OpenStore(lib.ID)
		if err != nil {
			continue
		}
		id, _, _, found, err := store.GetPathCache(absPath)
		if err != nil || !found {
			continue
		}
		ids[lib.ID] = id
		if title == nil {
			continue
		}
		if *title == "" {
			store.DeleteMeta(id, "title") //nolint:errcheck // the next scan reads the sidecar again
		} else {
			store.UpsertMeta(id, "title", *title) //nolint:errcheck
		}
	}
	return ids
}
