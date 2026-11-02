package metadata

import (
	"errors"
	"testing"
)

func TestDayKeepsEachFilesTimeAndOffset(t *testing.T) {
	fix := DateFix{Mode: "day", Day: "2024-05-01"}
	for old, want := range map[string]string{
		"2023-12-31T23:59:58+01:00": "2024-05-01T23:59:58+01:00",
		"2019-03-30T11:40:49":       "2024-05-01T11:40:49",
		"2019-03-30T11:40:49.123Z":  "2024-05-01T11:40:49Z",
	} {
		got, err := fix.Apply(old)
		if err != nil || got != want {
			t.Errorf("%s → %q, %v; want %s", old, got, err, want)
		}
	}
}

func TestDayWithoutDateNeedsATime(t *testing.T) {
	if _, err := (DateFix{Mode: "day", Day: "2024-05-01"}).Apply(""); !errors.Is(err, ErrNoDate) {
		t.Errorf("a file without date was given one: %v", err)
	}
	got, err := DateFix{Mode: "day", Day: "2024-05-01", Time: "9:05"}.Apply("")
	if err != nil || got != "2024-05-01T09:05:00" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestShiftKeepsTheTimeBetweenFiles(t *testing.T) {
	fix := DateFix{Mode: "shift", ShiftSeconds: 26 * 3600}
	for old, want := range map[string]string{
		"2024-12-31T23:30:00+01:00": "2025-01-02T01:30:00+01:00",
		"2024-03-30T12:00:00":       "2024-03-31T14:00:00", // across the change to summer time
	} {
		got, err := fix.Apply(old)
		if err != nil || got != want {
			t.Errorf("%s → %q, %v; want %s", old, got, err, want)
		}
	}
	back, _ := DateFix{Mode: "shift", ShiftSeconds: -3600}.Apply("2024-01-01T00:30:00")
	if back != "2023-12-31T23:30:00" {
		t.Errorf("shift back over the year: %s", back)
	}
	if _, err := fix.Apply(""); !errors.Is(err, ErrNoDate) {
		t.Error("a file without date was shifted")
	}
}

func TestDatesFromOtherProgramsAreRead(t *testing.T) {
	for in, want := range map[string]string{
		"2024-05-01T12:30:00.12+02:00": "2024-05-01T12:30:00+02:00",
		"2024:05:01 12:30:00":          "2024-05-01T12:30:00",
		"2024-05-01T12:30+02:00":       "2024-05-01T12:30:00+02:00",
		"2024-05-01":                   "2024-05-01T00:00:00",
		"May 2024":                     "",
	} {
		if got := normaliseDate(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}
