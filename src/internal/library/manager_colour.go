package library

// Colour returns the Colour statistics across the requested libraries (or
// all if ids is nil). The photos of every library are aggregated together,
// so medians and means span them all.
func (m *Manager) Colour(ids []string, pathPrefix, granularity string) (*LibraryColour, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	key := timelineCacheKey(libraryIDs(libs), pathPrefix, granularity)
	if v, ok := m.colourCache.Load(key); ok {
		return v.(*LibraryColour), nil
	}
	all := &ColourSource{Photos: []ColourPhoto{}, Swatches: []ColourSwatch{}}
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
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
