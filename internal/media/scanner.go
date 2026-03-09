package media

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EntryMeta holds lightweight EXIF metadata extracted during background scanning.
type EntryMeta struct {
	HasGPS         bool   `json:"hasGPS,omitempty"`
	FilmSimulation string `json:"filmSimulation,omitempty"`
	AspectRatio    string `json:"aspectRatio,omitempty"`
}

type EntryType string

const (
	EntryDir   EntryType = "dir"
	EntryImage EntryType = "image"
)

type Entry struct {
	Name     string     `json:"name"`
	Type     EntryType  `json:"type"`
	Date     time.Time  `json:"date"`
	ExifDate *time.Time `json:"exifDate,omitempty"`
	Size     int64      `json:"size,omitempty"`
}

// ScanDirectoryFast lists subdirectories and supported image files using file
// mod-times only (no EXIF extraction). This returns near-instantly even for
// large directories.
func ScanDirectoryFast(dirPath string) ([]Entry, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, de := range dirEntries {
		name := de.Name()

		if strings.HasPrefix(name, ".") {
			continue
		}

		info, err := de.Info()
		if err != nil {
			continue
		}

		if de.IsDir() {
			entries = append(entries, Entry{
				Name: name,
				Type: EntryDir,
				Date: info.ModTime(),
			})
		} else if IsSupportedImage(name) {
			entries = append(entries, Entry{
				Name: name,
				Type: EntryImage,
				Date: info.ModTime(),
				Size: info.Size(),
			})
		}
	}

	return entries, nil
}

// ScanDirectory lists subdirectories and supported image files in the given directory.
func ScanDirectory(dirPath string) ([]Entry, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, de := range dirEntries {
		name := de.Name()

		// Skip hidden files/directories
		if strings.HasPrefix(name, ".") {
			continue
		}

		info, err := de.Info()
		if err != nil {
			continue
		}

		if de.IsDir() {
			entries = append(entries, Entry{
				Name: name,
				Type: EntryDir,
				Date: info.ModTime(),
			})
		} else if IsSupportedImage(name) {
			// Try to get EXIF date, fall back to mod time
			date := info.ModTime()
			fullPath := filepath.Join(dirPath, name)
			if exifDate, err := ExtractDateTaken(fullPath); err == nil {
				date = exifDate
			}

			entries = append(entries, Entry{
				Name: name,
				Type: EntryImage,
				Date: date,
				Size: info.Size(),
			})
		}
	}

	return entries, nil
}

type SortField string
type SortOrder string

const (
	SortByName  SortField = "name"
	SortByDate  SortField = "date"
	SortByTaken SortField = "taken"
	SortBySize  SortField = "size"
	OrderAsc    SortOrder = "asc"
	OrderDesc   SortOrder = "desc"
)

func SortEntries(entries []Entry, field SortField, order SortOrder) {
	sort.SliceStable(entries, func(i, j int) bool {
		// Directories always come first
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == EntryDir
		}

		var less bool
		switch field {
		case SortByDate:
			less = entries[i].Date.Before(entries[j].Date)
		case SortByTaken:
			if entries[i].ExifDate == nil && entries[j].ExifDate == nil {
				less = false
			} else if entries[i].ExifDate == nil {
				return false // nil always last
			} else if entries[j].ExifDate == nil {
				return true // nil always last
			} else {
				less = entries[i].ExifDate.Before(*entries[j].ExifDate)
			}
		case SortBySize:
			less = entries[i].Size < entries[j].Size
		default:
			less = strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		}

		if order == OrderDesc {
			return !less
		}
		return less
	})
}
