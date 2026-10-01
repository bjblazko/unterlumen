package library

// ExposureSpace returns the points of the Exposure space across the
// requested libraries (or all if ids is nil).
func (m *Manager) ExposureSpace(ids []string, pathPrefix, granularity string) (*ExposureSpace, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	key := timelineCacheKey(libraryIDs(libs), pathPrefix, granularity)
	if v, ok := m.exposureCache.Load(key); ok {
		return v.(*ExposureSpace), nil
	}
	var all []LibraryExposure
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		points, err := store.ExposurePoints(pathPrefix)
		store.Close()
		if err != nil {
			continue
		}
		all = append(all, LibraryExposure{LibraryID: l.ID, Points: points})
	}
	es := BuildExposureSpace(all, granularity)
	m.exposureCache.Store(key, es)
	return es, nil
}
