package media

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Videos are listed and played, never edited (ADR-0058). The browser plays
// them itself; a format it cannot play still gets its still frame and is
// handed over as a download. Nothing is transcoded.

// videoTypes maps a video's extension to the type it is served with, or ""
// when browsers cannot play it and it is only downloaded.
var videoTypes = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".mov":  "video/quicktime",
	".webm": "video/webm",
	".mkv":  "",
	".avi":  "",
	".mts":  "",
	".m2ts": "",
	".3gp":  "",
}

// VideoExts are the videos' extensions as an index stores them: lower case,
// without the dot, .m4v as mp4.
func VideoExts() []string {
	var exts []string
	for ext := range videoTypes {
		if ext != ".m4v" {
			exts = append(exts, ext[1:])
		}
	}
	sort.Strings(exts)
	return exts
}

// IsVideo says whether name is a video Unterlumen lists.
func IsVideo(name string) bool {
	_, ok := videoTypes[strings.ToLower(filepath.Ext(name))]
	return ok
}

// IsSupportedMedia says whether name is a photo or a video Unterlumen lists.
func IsSupportedMedia(name string) bool {
	return IsSupportedImage(name) || IsVideo(name)
}

// VideoContentType is the type a video is played with, "" when the browser
// cannot play it.
func VideoContentType(name string) string {
	return videoTypes[strings.ToLower(filepath.Ext(name))]
}

// livePhotoExts are the photos an iPhone pairs a Live Photo's clip with.
var livePhotoExts = map[string]bool{".heic": true, ".heif": true, ".jpg": true, ".jpeg": true}

// LiveCompanions pairs the clips of Live Photos with their photos among the
// names of one folder: a .mov with the same name as a photo beside it. The
// map goes from the clip's name to the photo's.
func LiveCompanions(names []string) map[string]string {
	photos := map[string]string{}
	for _, n := range names {
		if livePhotoExts[strings.ToLower(filepath.Ext(n))] {
			photos[strings.ToLower(stem(n))] = n
		}
	}
	pairs := map[string]string{}
	for _, n := range names {
		if !strings.EqualFold(filepath.Ext(n), ".mov") {
			continue
		}
		if photo, ok := photos[strings.ToLower(stem(n))]; ok {
			pairs[n] = photo
		}
	}
	return pairs
}

func stem(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// LiveClips reads a folder and maps each Live Photo in it to the file of its
// clip. A folder that cannot be read (a share that is not mounted) has none.
func LiveClips(dir string) map[string]string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	clips := map[string]string{}
	for clip, photo := range LiveCompanions(names) {
		clips[photo] = filepath.Join(dir, clip)
	}
	return clips
}
