package installation

// Choice is what the setup asks: the photo folder, where the app keeps its
// own data ("" for the default), and whether destinations are shared with
// another installation that shows the same photos.
type Choice struct {
	PhotosDir string
	LibDir    string
	Share     bool
}

// Decide turns a choice into the configuration to save. cur is the saved
// configuration, fromDir the folder that holds the destinations now, and
// defaultLibDir the data folder used when none is chosen.
//
// Sharing means the shared folder in the photo folder, made here if it is
// not there yet — unless destinations are already shared somewhere else and
// the photo folder has no shared folder of its own, which is kept. Not
// sharing while the photo folder has one is written down as this
// installation's own folder, or the shared folder would be found and used
// again on the next start.
func Decide(cur Config, ch Choice, fromDir, defaultLibDir string) (Config, error) {
	next := cur
	next.PhotosDir = ch.PhotosDir
	next.LibDir = ch.LibDir
	ownDir := ch.LibDir
	if ownDir == "" {
		ownDir = defaultLibDir
	}
	found := FindShared(ch.PhotosDir)
	switch {
	case !ch.Share && found != "":
		next.ChannelsDir = ownDir
	case !ch.Share:
		next.ChannelsDir = ""
	case found == "" && cur.ChannelsDir != "" && cur.ChannelsDir != ownDir:
		// shared elsewhere already, e.g. a -channels-dir kept from an older launcher
	default:
		dir, err := Share(ch.PhotosDir, fromDir)
		if err != nil {
			return Config{}, err
		}
		next.ChannelsDir = dir
	}
	return next, nil
}
