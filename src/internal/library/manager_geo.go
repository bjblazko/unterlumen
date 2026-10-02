package library

// LibraryGeo holds the located photos of one library.
type LibraryGeo struct {
	LibraryID string
	Points    []GeoPoint
}

// GeoPoints returns the located photos of the requested libraries (or all
// if ids is nil) within the filter (ADR-0050). A library whose database
// cannot be read is left out rather than failing the whole map.
func (m *Manager) GeoPoints(ids []string, f Filter) ([]LibraryGeo, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	result := []LibraryGeo{}
	for _, l := range libs {
		store, err := m.OpenFiltered(l.ID, f)
		if err != nil {
			continue
		}
		points, err := store.GeoPoints()
		store.Close()
		if err != nil || len(points) == 0 {
			continue
		}
		result = append(result, LibraryGeo{LibraryID: l.ID, Points: points})
	}
	return result, nil
}
