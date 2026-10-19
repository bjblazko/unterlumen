// Package birthdays keeps the birthdays of the people named in the
// libraries (ADR-0051), by name, in people.json beside channels.json: the
// folder installations share, so a birthday given on one is known on the
// other, and in every library, since a name is the same person everywhere.
// A birthday is about a person, not a photo, so it stays out of the sidecars.
package birthdays

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/atomicfile"
)

// ErrBadDate says a birthday is not a date of the form 2006-01-02 that has
// already been.
var ErrBadDate = errors.New("a birthday is a past date like 2015-05-12")

const layout = "2006-01-02"

// Store reads and writes people.json in one folder.
type Store struct {
	path string
}

// New keeps the birthdays in dir.
func New(dir string) *Store {
	return &Store{path: filepath.Join(dir, "people.json")}
}

type file struct {
	Birthdays map[string]string `json:"birthdays"`
}

func parse(data []byte) (file, error) {
	f := file{Birthdays: map[string]string{}}
	if len(data) == 0 {
		return f, nil
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, err
	}
	if f.Birthdays == nil {
		f.Birthdays = map[string]string{}
	}
	return f, nil
}

// All returns every birthday by name as it was given.
func (s *Store) All() (map[string]string, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	f, err := parse(data)
	return f.Birthdays, err
}

// Set gives name a birthday, replacing one under the same name in any
// case; an empty date takes it away.
func (s *Store) Set(name, date string) error {
	name = strings.TrimSpace(name)
	if date != "" {
		d, err := time.Parse(layout, date)
		if err != nil || d.After(time.Now()) {
			return ErrBadDate
		}
	}
	return atomicfile.Change(s.path, func(data []byte) ([]byte, error) {
		f, err := parse(data)
		if err != nil {
			return nil, err
		}
		for k := range f.Birthdays {
			if strings.EqualFold(k, name) {
				delete(f.Birthdays, k)
			}
		}
		if date != "" {
			f.Birthdays[name] = date
		}
		out, err := json.MarshalIndent(f, "", "  ")
		return append(out, '\n'), err
	})
}

// Of returns the birthday of name, in any case; false when none was given.
func Of(all map[string]string, name string) (time.Time, bool) {
	for k, v := range all {
		if strings.EqualFold(k, name) {
			d, err := time.Parse(layout, v)
			return d, err == nil
		}
	}
	return time.Time{}, false
}

// AgeAt is how old someone born on birth was on day, in whole years.
func AgeAt(birth, day time.Time) int {
	age := day.Year() - birth.Year()
	if day.Month() < birth.Month() || (day.Month() == birth.Month() && day.Day() < birth.Day()) {
		age--
	}
	return age
}
