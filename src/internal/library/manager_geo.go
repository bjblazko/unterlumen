package library

// LibraryGeo holds the located photos of one library.
type LibraryGeo struct {
	LibraryID string
	Points    []GeoPoint
}

// GeoPoints returns the located photos of every library. A library whose
// database cannot be read is left out rather than failing the whole map.
func (m *Manager) GeoPoints() ([]LibraryGeo, error) {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil, err
	}
	result := []LibraryGeo{}
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
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
