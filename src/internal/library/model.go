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
