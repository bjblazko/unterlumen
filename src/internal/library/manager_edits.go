package library

// MetadataWritten brings what was written about the file at absPath into
// every library that covers it: the file is indexed again — a new ID when
// its content changed, the sidecar's or NFO's fields when only they did —
// and the notes written to the sidecar — the title, own fields; "" for one
// that went — become the photo's notes, which a scan would otherwise only
// add, never remove, in a library whose notes are not yet in the sidecars
// (ADR-0048, ADR-0060). It returns the photo's ID in each library afterwards.
func (m *Manager) MetadataWritten(absPath string, notes map[string]string) map[string]string {
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
		for key, value := range notes {
			if value == "" {
				store.DeleteMeta(id, key) //nolint:errcheck // the next scan reads the sidecar again
			} else {
				store.UpsertMeta(id, key, value) //nolint:errcheck
			}
		}
	}
	return ids
}
