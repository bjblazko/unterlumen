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
	Name       string            `json:"name"`
	Path       string            `json:"path"`
	Modified   time.Time         `json:"modified"`
	TotalSize  int64             `json:"totalSize"`
	FileCount  int               `json:"fileCount"`
	DirCount   int               `json:"dirCount"`
	MaxDepth   int               `json:"maxDepth"`
	Subfolders []SubfolderStats  `json:"subfolders"`
	FileTypes  map[string]int    `json:"fileTypes"`
}

// WalkFolderStats computes aggregated statistics for a directory by walking
// its tree up to maxFolderWalkDepth levels deep.
func WalkFolderStats(absPath, relPath string) (*FolderStats, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	result := &FolderStats{
		Name:       filepath.Base(absPath),
		Path:       relPath,
		Modified:   info.ModTime(),
		FileTypes:  make(map[string]int),
		Subfolders: []SubfolderStats{},
	}

	// Read immediate children to determine subfolder order.
	dirEntries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	var subDirNames []string
	for _, de := range dirEntries {
		if strings.HasPrefix(de.Name(), ".") {
			continue
		}
		if de.IsDir() {
			subDirNames = append(subDirNames, de.Name())
		}
	}

	subStats := make(map[string]*SubfolderStats, len(subDirNames))
	for _, name := range subDirNames {
		sub := &SubfolderStats{Name: name}
		subStats[name] = sub
	}

	sep := string(filepath.Separator)

	err = filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == absPath {
			return nil
		}

		rel, _ := filepath.Rel(absPath, path)
		parts := strings.Split(rel, sep)
		depth := len(parts)

		if depth > maxFolderWalkDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		topName := parts[0]
		sub := subStats[topName]

		if d.IsDir() {
			result.DirCount++
			if sub != nil && len(parts) > 1 {
				sub.DirCount++
				subDepth := depth - 1
				if subDepth > sub.MaxDepth {
					sub.MaxDepth = subDepth
				}
			}
			if depth > result.MaxDepth {
				result.MaxDepth = depth
			}
		} else {
			fi, fierr := d.Info()
			var size int64
			if fierr == nil {
				size = fi.Size()
			}
			result.FileCount++
			result.TotalSize += size
			if ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(d.Name()), ".")); ext != "" {
				result.FileTypes[ext]++
			}
			if sub != nil {
				sub.FileCount++
				sub.Size += size
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, name := range subDirNames {
		if sub, ok := subStats[name]; ok {
			result.Subfolders = append(result.Subfolders, *sub)
		}
	}

	return result, nil
}
