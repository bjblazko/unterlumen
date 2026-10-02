package library

// Colour returns the Colour statistics across the requested libraries (or
// all if ids is nil). The photos of every library are aggregated together,
// so medians and means span them all.
func (m *Manager) Colour(ids []string, pathPrefix, granularity string, f Filter) (*LibraryColour, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	key := timelineCacheKey(libraryIDs(libs), pathPrefix+f.Key(), granularity)
	if v, ok := m.colourCache.Load(key); ok {
		return v.(*LibraryColour), nil
	}
	all := &ColourSource{Photos: []ColourPhoto{}, Swatches: []ColourSwatch{}}
	for _, l := range libs {
		store, err := m.OpenFiltered(l.ID, f)
		if err != nil {
			continue
		}
		src, err := store.ColourSource(pathPrefix)
		store.Close()
		if err != nil {
			continue
		}
		all.Photos = append(all.Photos, src.Photos...)
		all.Swatches = append(all.Swatches, src.Swatches...)
		all.Unanalysed += src.Unanalysed
	}
	c := BuildColour(all, granularity)
	m.colourCache.Store(key, c)
	return c, nil
}

// ColourSpace returns the points of the Colour space across the requested
// libraries (or all if ids is nil).
func (m *Manager) ColourSpace(ids []string, pathPrefix, granularity string, f Filter) (*ColourSpace, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	key := timelineCacheKey(libraryIDs(libs), pathPrefix+f.Key(), granularity)
	if v, ok := m.colourSpaceCache.Load(key); ok {
		return v.(*ColourSpace), nil
	}
	var all []LibraryPoints
	for _, l := range libs {
		store, err := m.OpenFiltered(l.ID, f)
		if err != nil {
			continue
		}
		points, err := store.ColourPoints(pathPrefix)
		var unanalysed int
		if err == nil {
			unanalysed, err = store.UnanalysedCount(pathPrefix)
		}
		store.Close()
		if err != nil {
			continue
		}
		all = append(all, LibraryPoints{LibraryID: l.ID, Points: points, Unanalysed: unanalysed})
	}
	cs := BuildColourSpace(all, granularity)
	m.colourSpaceCache.Store(key, cs)
	return cs, nil
}
