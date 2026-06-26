package library

import "time"

// Library represents a managed photo collection.
type Library struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	SourcePath  string     `json:"sourcePath"`
	CreatedAt   time.Time  `json:"createdAt"`
	PhotoCount  int        `json:"photoCount"`
	LastIndexed   *time.Time `json:"lastIndexed,omitempty"`
	LastNewPhotos *time.Time `json:"lastNewPhotos,omitempty"`
}

// Photo represents an indexed photo in a library.
type Photo struct {
	ID        string            `json:"id"`
	PathHint  string            `json:"pathHint"`
	Filename  string            `json:"filename"`
	FileSize  int64             `json:"fileSize"`
	IndexedAt time.Time         `json:"indexedAt"`
	DateTaken string            `json:"dateTaken,omitempty"`
	Status    string            `json:"status"`
	Exif      map[string]string `json:"exif,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// MetaEntry is a single user-defined key/value pair for a photo.
type MetaEntry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// LibraryPhoto is a Photo annotated with its library context, used in cross-library search results.
type LibraryPhoto struct {
	LibraryID   string `json:"libraryID"`
	LibraryName string `json:"libraryName"`
	Photo
}

// CrossLibraryResult holds paginated search results from one or more libraries.
type CrossLibraryResult struct {
	Results []LibraryPhoto `json:"results"`
	Total   int            `json:"total"`
}

// NameCount is a name/count pair used in statistics aggregations.
type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// ValueCount is a numeric value with its frequency count, used for histogram data.
type ValueCount struct {
	Value float64 `json:"value"`
	Count int     `json:"count"`
}

// CameraLensCount is a camera+lens combination with a shot count.
type CameraLensCount struct {
	Camera string `json:"camera"`
	Lens   string `json:"lens"`
	Count  int    `json:"count"`
}

// LibraryFolderStats holds DB-backed statistics for a single folder within a library.
// It is computed from indexed photos only and excludes sidecar files.
type LibraryFolderStats struct {
	PhotoCount int            `json:"photoCount"`
	TotalSize  int64          `json:"totalSize"`
	Formats    []NameCount    `json:"formats"`
	Subfolders []LibSubfolder `json:"subfolders"`
	DateFirst  string         `json:"dateFirst,omitempty"`
	DateLast   string         `json:"dateLast,omitempty"`
}

// LibSubfolder holds aggregated photo stats for one immediate subdirectory.
type LibSubfolder struct {
	Name       string `json:"name"`
	PhotoCount int    `json:"photoCount"`
	TotalSize  int64  `json:"totalSize"`
}

// LibraryStatistics holds aggregated statistics across one or more libraries.
type LibraryStatistics struct {
	TotalPhotos    int               `json:"totalPhotos"`
	IndexingPhotos int               `json:"indexingPhotos,omitempty"`
	Warnings       []string          `json:"warnings,omitempty"`
	Formats        []NameCount       `json:"formats"`
	FilmSims       []NameCount       `json:"filmSims"`
	FocalLengths   []ValueCount      `json:"focalLengths"`   // native mm: {value, count} pairs
	FocalLengths35 []ValueCount      `json:"focalLengths35"` // 35mm-equivalent: {value, count} pairs
	Apertures      []ValueCount      `json:"apertures"`
	ISOs           []ValueCount      `json:"isos"`
	CameraLens     []CameraLensCount `json:"cameraLens"`
	ShootingHours  [24]int           `json:"shootingHours"` // index = hour 0–23
	ShootingDays   map[string]int    `json:"shootingDays"`  // "YYYY-MM-DD": count
}

// LibraryTimeline holds time-series statistics across one or more libraries.
type LibraryTimeline struct {
	Granularity    string            `json:"granularity"`    // "month" or "year"
	Periods        []string          `json:"periods"`        // sorted period labels
	CameraUsage    []CameraTimeSlice `json:"cameraUsage"`
	FocalStats     []PeriodStats     `json:"focalStats"`
	ISOStats       []PeriodStats     `json:"isoStats"`
	ApertureHeat   []ApertureRow     `json:"apertureHeat"`
	AspectRatios   []AspectSlice     `json:"aspectRatios"`
	MegapixelStats []MegapixelStat   `json:"megapixelStats"`
}

// CameraTimeSlice holds per-period photo counts for one camera, aligned to LibraryTimeline.Periods.
type CameraTimeSlice struct {
	Camera string `json:"camera"`
	Counts []int  `json:"counts"`
}

// PeriodStats holds distribution statistics for a numeric EXIF field in one time period.
type PeriodStats struct {
	Period string  `json:"period"`
	Median float64 `json:"median"`
	P25    float64 `json:"p25"`
	P75    float64 `json:"p75"`
	Count  int     `json:"count"`
}

// ApertureRow holds per-f-stop-bucket counts for one time period.
type ApertureRow struct {
	Period  string         `json:"period"`
	Buckets map[string]int `json:"buckets"`
}

// AspectSlice holds per-period photo counts for one aspect ratio label, aligned to LibraryTimeline.Periods.
type AspectSlice struct {
	Ratio  string `json:"ratio"`  // "1:1", "4:3", "3:2", "16:9+", "other"
	Counts []int  `json:"counts"`
}

// MegapixelStat holds max and average megapixels for one time period.
type MegapixelStat struct {
	Period string  `json:"period"`
	Max    float64 `json:"max"`
	Avg    float64 `json:"avg"`
	Count  int     `json:"count"`
}
