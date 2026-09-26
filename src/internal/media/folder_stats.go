package media

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxFolderWalkDepth = 10

// SubfolderStats holds aggregated stats for one immediate subdirectory.
type SubfolderStats struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	FileCount int    `json:"fileCount"`
	DirCount  int    `json:"dirCount"`
	MaxDepth  int    `json:"maxDepth"`
}

// FolderStats holds aggregated stats for a directory and its contents.
type FolderStats struct {
	Name       string           `json:"name"`
	Path       string           `json:"path"`
	Modified   time.Time        `json:"modified"`
	TotalSize  int64            `json:"totalSize"`
	FileCount  int              `json:"fileCount"`
	DirCount   int              `json:"dirCount"`
	MaxDepth   int              `json:"maxDepth"`
	Subfolders []SubfolderStats `json:"subfolders"`
	FileTypes  map[string]int   `json:"fileTypes"`
}

// WalkFolderStats computes aggregated statistics for a directory by walking
// its tree up to maxFolderWalkDepth levels deep.
func WalkFolderStats(absPath, relPath string) (*FolderStats, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	// Immediate children determine the subfolder order.
	dirEntries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}
	fw := &folderWalk{
		root: absPath,
		result: &FolderStats{
			Name:       filepath.Base(absPath),
			Path:       relPath,
			Modified:   info.ModTime(),
			FileTypes:  make(map[string]int),
			Subfolders: []SubfolderStats{},
		},
		subs: make(map[string]*SubfolderStats),
	}
	subDirNames := visibleSubdirs(dirEntries)
	for _, name := range subDirNames {
		fw.subs[name] = &SubfolderStats{Name: name}
	}
	if err := filepath.WalkDir(absPath, fw.visit); err != nil {
		return nil, err
	}
	for _, name := range subDirNames {
		fw.result.Subfolders = append(fw.result.Subfolders, *fw.subs[name])
	}
	return fw.result, nil
}

// visibleSubdirs returns the names of the directories among entries that are
// not hidden, in their order.
func visibleSubdirs(entries []os.DirEntry) []string {
	var names []string
	for _, de := range entries {
		if de.IsDir() && !strings.HasPrefix(de.Name(), ".") {
			names = append(names, de.Name())
		}
	}
	return names
}

// folderWalk accumulates FolderStats during one filepath.WalkDir.
type folderWalk struct {
	root   string
	result *FolderStats
	subs   map[string]*SubfolderStats // immediate subfolders by name
}

// visit counts one entry below root. Hidden entries and anything deeper than
// maxFolderWalkDepth are skipped; unreadable entries are ignored.
func (fw *folderWalk) visit(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return nil
	}
	if strings.HasPrefix(d.Name(), ".") {
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	if path == fw.root {
		return nil
	}
	rel, _ := filepath.Rel(fw.root, path)
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) > maxFolderWalkDepth {
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	if d.IsDir() {
		fw.addDir(parts)
	} else {
		fw.addFile(d, parts)
	}
	return nil
}

// addDir counts a directory at the depth of its path parts.
func (fw *folderWalk) addDir(parts []string) {
	depth := len(parts)
	fw.result.DirCount++
	fw.result.MaxDepth = max(fw.result.MaxDepth, depth)
	if sub := fw.subs[parts[0]]; sub != nil && depth > 1 {
		sub.DirCount++
		sub.MaxDepth = max(sub.MaxDepth, depth-1)
	}
}

// addFile counts a file, its size and its extension.
func (fw *folderWalk) addFile(d fs.DirEntry, parts []string) {
	var size int64
	if fi, err := d.Info(); err == nil {
		size = fi.Size()
	}
	fw.result.FileCount++
	fw.result.TotalSize += size
	if ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(d.Name()), ".")); ext != "" {
		fw.result.FileTypes[ext]++
	}
	if sub := fw.subs[parts[0]]; sub != nil {
		sub.FileCount++
		sub.Size += size
	}
}
