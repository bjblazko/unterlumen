package export

import (
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

// photoRef names a library photo by its library and its ID. Filter results
// span libraries and carry absolute paths, which server mode refuses; the
// index knows where each photo is.
type photoRef struct {
	Library string `json:"library"`
	ID      string `json:"id"`
	// Key is what the page calls this photo (its path); an estimate
	// answers under it, so the page can find the row.
	Key string `json:"key,omitempty"`
}

// zipItem is one file for the ZIP: where it is, and its name inside the ZIP
// ("IMG_1.jpg", or "Travel/2024/IMG_1.jpg" for a photo in a chosen folder).
type zipItem struct {
	abs  string
	name string
}

// zipSources says where the photos for a ZIP come from.
type zipSources struct {
	root       string // the browse root, or a library's source folder
	serverRole bool
	libs       *lib.Manager // nil without library support
}

// collect turns a request's files, folders and library photos into ZIP
// items. What cannot be read is returned by name, so the page can say so.
func (s zipSources) collect(req exportRequest) (items []zipItem, refused []string) {
	for _, rel := range req.Files {
		if abs, ok := resolveFilePath(s.root, s.serverRole, rel); ok {
			items = append(items, zipItem{abs: abs, name: filepath.Base(abs)})
		} else {
			refused = append(refused, filepath.Base(rel))
		}
	}
	for _, dir := range req.Dirs {
		found, ok := s.walk(dir)
		if !ok {
			refused = append(refused, filepath.Base(dir))
		}
		items = append(items, found...)
	}
	for _, ref := range req.Photos {
		if abs, ok := s.libraryPhoto(ref); ok {
			items = append(items, zipItem{abs: abs, name: filepath.Base(abs)})
		} else {
			refused = append(refused, ref.ID)
		}
	}
	return items, refused
}

// walk finds every photo under a folder (relative to the root), keeping the
// folder's own name as the top of the paths inside the ZIP. Hidden files
// and folders (.unterlumen, .DS_Store) are left out.
func (s zipSources) walk(rel string) ([]zipItem, bool) {
	abs, ok := pathguard.SafePath(s.root, rel)
	if !ok {
		return nil, false
	}
	top := filepath.Base(abs)
	var items []zipItem
	err := filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable subfolder does not stop the rest
		}
		if strings.HasPrefix(d.Name(), ".") && p != abs {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !media.IsSupportedImage(d.Name()) {
			return nil
		}
		inner, _ := filepath.Rel(abs, p)
		items = append(items, zipItem{abs: p, name: path.Join(top, filepath.ToSlash(inner))})
		return nil
	})
	return items, err == nil
}

func (s zipSources) libraryPhoto(ref photoRef) (string, bool) {
	if s.libs == nil {
		return "", false
	}
	store, err := s.libs.OpenStore(ref.Library)
	if err != nil {
		return "", false
	}
	defer store.Close()
	p, err := store.GetPhotoPathHint(ref.ID)
	if err != nil || p == "" {
		return "", false
	}
	return p, true
}
