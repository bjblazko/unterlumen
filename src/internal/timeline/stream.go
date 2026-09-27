package timeline

import "math"

// Photo is one photo on the timeline.
type Photo struct {
	LibraryID string
	ID        string
	Filename  string
	Taken     string  // ISO date taken
	Day       int     // days since the stream's first day
	Ratio     float64 // width ÷ height as shown
}

// Stream is every dated photo of every library, oldest first, each once.
type Stream struct {
	Version string
	Start   string // the first photo's day, YYYY-MM-DD; "" when empty
	Photos  []Photo
	Undated int
}

// Skeleton is what the browser lays the timeline out from: a day and an
// aspect ratio per photo, in stream order.
type Skeleton struct {
	Version string    `json:"version"`
	Start   string    `json:"start"`
	Days    []int     `json:"days"`
	Ratios  []float64 `json:"ratios"`
	Undated int       `json:"undated"`
}

// Skeleton returns the stream's skeleton; ratios are rounded to three
// decimals, which is finer than a pixel at any tile size.
func (s *Stream) Skeleton() Skeleton {
	sk := Skeleton{Version: s.Version, Start: s.Start, Undated: s.Undated,
		Days: make([]int, len(s.Photos)), Ratios: make([]float64, len(s.Photos))}
	for i, p := range s.Photos {
		sk.Days[i] = p.Day
		sk.Ratios[i] = math.Round(p.Ratio*1000) / 1000
	}
	return sk
}

// Page returns up to count photos from index from on; a range outside the
// stream is empty.
func (s *Stream) Page(from, count int) []Photo {
	if from < 0 || from >= len(s.Photos) || count <= 0 {
		return []Photo{}
	}
	return s.Photos[from:min(from+count, len(s.Photos))]
}
