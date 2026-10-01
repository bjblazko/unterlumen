package library

// SpaceTime returns the located photos across the requested libraries (or
// all if ids is nil), with their main colours.
func (m *Manager) SpaceTime(ids []string, pathPrefix string) (*SpaceTime, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	key := timelineCacheKey(libraryIDs(libs), pathPrefix, "")
	if v, ok := m.spaceTimeCache.Load(key); ok {
		return v.(*SpaceTime), nil
	}
	var all []LibrarySpaceTime
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		places, err := store.LocatedPhotos(pathPrefix)
		var colours []ColourPoint
		if err == nil {
			colours, err = store.ColourPoints(pathPrefix)
		}
		store.Close()
		if err != nil {
			continue
		}
		byID := make(map[string]LCh, len(colours))
		for _, c := range colours {
			byID[c.ID] = c.Colour
		}
		all = append(all, LibrarySpaceTime{LibraryID: l.ID, Places: places, Colours: byID})
	}
	st := BuildSpaceTime(all)
	m.spaceTimeCache.Store(key, st)
	return st, nil
}
