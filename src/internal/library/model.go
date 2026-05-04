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
	LastIndexed *time.Time `json:"lastIndexed,omitempty"`
}

// Photo represents an indexed photo in a library.
type Photo struct {
	ID        string            `json:"id"`
	PathHint  string            `json:"pathHint"`
	Filename  string            `json:"filename"`
	FileSize  int64             `json:"fileSize"`
	IndexedAt time.Time         `json:"indexedAt"`
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

// LibraryStatistics holds aggregated statistics across one or more libraries.
type LibraryStatistics struct {
	TotalPhotos    int               `json:"totalPhotos"`
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
