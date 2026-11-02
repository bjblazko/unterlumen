package metadata

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// DateFix corrects the date taken of many files at once, each from its own
// date: Day puts every file on one day and keeps each one's time of day and
// offset; Shift moves every file by the same amount, so the time between
// them stays.
type DateFix struct {
	Mode string `json:"mode"` // "day" or "shift"
	Day  string `json:"day,omitempty"`
	// Time is the time of day for a file that has no date yet; without it
	// such a file is skipped by Day.
	Time         string `json:"time,omitempty"`
	ShiftSeconds int64  `json:"shiftSeconds,omitempty"`
}

// ErrNoDate says a file has no date to correct.
var ErrNoDate = errors.New("it has no date taken to correct")

const (
	layoutLocal = "2006-01-02T15:04:05"
	layoutDay   = "2006-01-02"
)

// Apply gives the new date for a file whose date taken is old ("" for none),
// in the same form: with its offset when it had one.
func (f DateFix) Apply(old string) (string, error) {
	switch f.Mode {
	case "day":
		return f.setDay(old)
	case "shift":
		return f.shift(old)
	}
	return "", fmt.Errorf("unknown date correction %q", f.Mode)
}

func (f DateFix) setDay(old string) (string, error) {
	day, err := time.Parse(layoutDay, f.Day)
	if err != nil {
		return "", fmt.Errorf("the day %q is not a date like 2024-05-01", f.Day)
	}
	clock, offset, err := splitDate(old)
	if errors.Is(err, ErrNoDate) && f.Time != "" {
		clock, err = time.Parse("15:04:05", normaliseClock(f.Time))
	}
	if err != nil {
		return "", err
	}
	t := time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), clock.Second(), 0, time.UTC)
	return t.Format(layoutLocal) + offset, nil
}

func (f DateFix) shift(old string) (string, error) {
	t, offset, err := splitDate(old)
	if err != nil {
		return "", err
	}
	return t.Add(time.Duration(f.ShiftSeconds)*time.Second).Format(layoutLocal) + offset, nil
}

// splitDate reads a date taken as its wall-clock time (in UTC, so no
// daylight saving moves it) and its offset as written ("+02:00", "Z", "").
func splitDate(s string) (time.Time, string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, "", ErrNoDate
	}
	if len(s) == len(layoutDay) {
		s += "T00:00:00"
	}
	if len(s) < len(layoutLocal) {
		return time.Time{}, "", fmt.Errorf("its date taken %q cannot be read", s)
	}
	head, offset := s[:len(layoutLocal)], s[len(layoutLocal):]
	if i := strings.IndexAny(offset, "+-Z"); i > 0 {
		offset = offset[i:] // drop fractions of a second
	} else if i < 0 {
		offset = ""
	}
	t, err := time.Parse(layoutLocal, strings.Replace(head, " ", "T", 1))
	if err != nil {
		return time.Time{}, "", fmt.Errorf("its date taken %q cannot be read", s)
	}
	return t, offset, nil
}

// normaliseClock accepts 9:05 and 09:05:00.
func normaliseClock(s string) string {
	parts := strings.Split(strings.TrimSpace(s), ":")
	for len(parts) < 3 {
		parts = append(parts, "00")
	}
	for i, p := range parts {
		if len(p) == 1 {
			parts[i] = "0" + p
		}
	}
	return strings.Join(parts[:3], ":")
}

// normaliseDate writes a date as other programs keep it in a sidecar — with
// fractions, without seconds, in EXIF's "2024:05:01 12:30:00" as darktable
// does — the way the index keeps dates: "2024-05-01T12:30:00" and the
// offset when there is one. "" for a date that cannot be read.
func normaliseDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 && s[4] == ':' && s[7] == ':' {
		s = s[:4] + "-" + s[5:7] + "-" + s[8:]
	}
	if len(s) >= 16 && (len(s) == 16 || s[16] != ':') && s[13] == ':' {
		s = s[:16] + ":00" + s[16:]
	}
	t, offset, err := splitDate(s)
	if err != nil {
		return ""
	}
	return t.Format(layoutLocal) + offset
}
