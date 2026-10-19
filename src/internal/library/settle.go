package library

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// settle makes the photos inside folder ("" for the whole library) what the
// files in present say, in one step at the end of a run that saw every file:
// a photo none of whose known paths is there any more is removed — also an
// older version of a file that was changed in place, whose path now belongs
// to the new content —, a path no file answers to is forgotten, a photo left
// marked missing that is there again is set back to ok, and one known by a
// path that is gone but found at another of its paths takes that one. A run
// that stops early never gets here, so it changes nothing; a library folder
// that shows no files at all while the library knows photos is taken for a
// share that is not there, and nothing is removed.
func (idx *Indexer) settle(folder string, present map[string]bool) {
	st, err := idx.store.settleState(folder)
	if err != nil || (folder == "" && len(present) == 0 && len(st.photos) > 0) {
		return
	}
	inScope := func(p string) bool {
		return folder == "" || strings.HasPrefix(p, folder+string(filepath.Separator))
	}
	var gone, found []string
	for _, ph := range st.photos {
		at, alive := ph.whereFound(present, inScope, st.owner)
		switch {
		case !alive:
			gone = append(gone, ph.id)
		case at != "" && at != ph.pathHint:
			idx.store.MarkPhotoPresent(ph.id, at, filepath.Base(at)) //nolint:errcheck
		case ph.missing:
			found = append(found, ph.id)
		}
	}
	var stale []string
	for p := range st.owner {
		if inScope(p) && !present[p] {
			stale = append(stale, p)
		}
	}
	idx.store.PurgePhotos(gone)  //nolint:errcheck
	idx.store.forgetPaths(stale) //nolint:errcheck
	idx.store.markOK(found)      //nolint:errcheck
}

// settlePhoto is a photo inside the folder being settled, with every path
// the library knows it by.
type settlePhoto struct {
	id, pathHint string
	missing      bool
	paths        []string
}

// whereFound says whether the photo is still there and at which of its known
// paths ("" when it is kept only because a path outside the settled folder
// cannot be checked). Its last path counts while no other photo owns it:
// a photo indexed before the path cache has no known path at all.
func (ph settlePhoto) whereFound(present map[string]bool, inScope func(string) bool, owner map[string]string) (string, bool) {
	if present[ph.pathHint] && owner[ph.pathHint] == ph.id {
		return ph.pathHint, true
	}
	outside := false
	for _, p := range ph.paths {
		if present[p] {
			return p, true
		}
		outside = outside || !inScope(p)
	}
	if outside {
		return "", true
	}
	if present[ph.pathHint] && owner[ph.pathHint] == "" {
		return ph.pathHint, true
	}
	return "", false
}

// settleSnapshot is what settle needs from the index: the photos whose last
// path lies in the folder, and which photo every known path belongs to.
type settleSnapshot struct {
	photos []settlePhoto
	owner  map[string]string
}

func (s *Store) settleState(folder string) (settleSnapshot, error) {
	st := settleSnapshot{owner: map[string]string{}}
	paths := map[string][]string{}
	rows, err := s.db.Query(`SELECT abs_path, photo_id FROM path_cache`)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var p, id string
		if err := rows.Scan(&p, &id); err != nil {
			rows.Close()
			return st, err
		}
		st.owner[p] = id
		paths[id] = append(paths[id], p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, err
	}

	q, args := `SELECT id, path_hint, status FROM photos`, []any{}
	if folder != "" {
		q += ` WHERE path_hint LIKE ? ESCAPE '\'`
		args = append(args, escapeLikePattern(folder)+string(filepath.Separator)+"%")
	}
	rows, err = s.db.Query(q, args...)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var ph settlePhoto
		var status string
		if err := rows.Scan(&ph.id, &ph.pathHint, &status); err != nil {
			return st, err
		}
		ph.missing = status == "missing"
		ph.paths = paths[ph.id]
		st.photos = append(st.photos, ph)
	}
	return st, rows.Err()
}

// forgetPaths drops paths from the path cache.
func (s *Store) forgetPaths(paths []string) error {
	for len(paths) > 0 {
		n := min(len(paths), 500)
		args := make([]any, n)
		for i, p := range paths[:n] {
			args[i] = p
		}
		if _, err := s.db.Exec(`DELETE FROM path_cache WHERE abs_path IN (`+placeholders(n)+`)`, args...); err != nil {
			return err
		}
		paths = paths[n:]
	}
	return nil
}

// collectInLibrary is collectFiles for a folder inside the library: one
// that is no longer there, in a library folder that is, holds no files.
func (idx *Indexer) collectInLibrary(folder string) ([]string, error) {
	files, err := collectFiles(folder)
	if os.IsNotExist(err) && folder != idx.sourcePath {
		if info, serr := os.Stat(idx.sourcePath); serr == nil && info.IsDir() {
			return nil, nil
		}
	}
	return files, err
}

// nothingRemoved explains a run that could not read every file and so took
// no photo out of the library.
func nothingRemoved(err error) string {
	if os.IsNotExist(err) {
		return "The library's folder is not there — is the drive or share connected? Nothing was taken out of the library."
	}
	return fmt.Sprintf("Not every folder could be read, so nothing was taken out of the library: %v", err)
}
