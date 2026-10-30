package media

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A video's metadata comes from what `ffmpeg -i` prints: its duration, when
// it was taken, where, with which camera and its size. ffprobe would say the
// same in JSON, but the macOS installer fetches ffmpeg alone.

var (
	probeDuration = regexp.MustCompile(`Duration: (\d+):(\d{2}):(\d{2}(?:\.\d+)?)`)
	probeTag      = regexp.MustCompile(`^\s{4}([A-Za-z0-9._-]+)\s*: (.*)$`)
	probeVideo    = regexp.MustCompile(`Stream #0:\d+.*: Video: .*?, (\d{2,5})x(\d{2,5})`)
	probeRotation = regexp.MustCompile(`(?i)display ?matrix: rotation of (-?\d+(?:\.\d+)?) degrees`)
	probeISO6709  = regexp.MustCompile(`^([+-]\d+(?:\.\d+)?)([+-]\d+(?:\.\d+)?)`)
)

// ExtractVideoMeta reads a video's metadata into the same form as a photo's
// EXIF, so the Timeline, the Map and the filter treat both alike. Duration
// is in Tags as seconds.
func ExtractVideoMeta(path string) (*ExifData, error) {
	out := ffmpegProbe(path)
	if !strings.Contains(out, "Input #0") {
		return nil, fmt.Errorf("ffmpeg could not read %s", path)
	}
	return parseVideoProbe(out), nil
}

// ExtractMeta reads a photo's EXIF or a video's metadata. Without a date
// taken in them, the date the file's name gives stands in, marked in Tags as
// DateTakenFrom; a file that has only that is no error.
func ExtractMeta(path string) (*ExifData, error) {
	var data *ExifData
	var err error
	if IsVideo(path) {
		data, err = ExtractVideoMeta(path)
	} else {
		data, err = ExtractAllEXIF(path)
	}
	if data != nil && data.DateTaken != nil {
		return data, err
	}
	d := DateFromFileName(filepath.Base(path))
	if d == "" {
		return data, err
	}
	if data == nil {
		data = &ExifData{}
	}
	if data.Tags == nil {
		data.Tags = map[string]string{}
	}
	data.DateTaken = &d
	data.Tags["DateTakenFrom"] = "file name"
	return data, nil
}

func parseVideoProbe(out string) *ExifData {
	data := &ExifData{Tags: map[string]string{}}
	tags := containerTags(out)
	if d, ok := parseProbeDuration(out); ok {
		data.Tags["Duration"] = strconv.FormatFloat(d, 'f', 2, 64)
	}
	if maker := tags["com.apple.quicktime.make"]; maker != "" {
		data.Tags["Make"] = maker
	}
	if model := tags["com.apple.quicktime.model"]; model != "" {
		data.Tags["Model"] = model
	}
	data.DateTaken = videoDate(tags)
	data.Latitude, data.Longitude = videoLocation(tags)
	data.Width, data.Height = videoSize(out)
	return data
}

// containerTags are the container's metadata: the lines indented by four
// spaces before the first stream.
func containerTags(out string) map[string]string {
	tags := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Stream #") {
			break
		}
		if m := probeTag.FindStringSubmatch(strings.TrimRight(line, "\r")); m != nil {
			tags[m[1]] = strings.TrimSpace(m[2])
		}
	}
	return tags
}

func parseProbeDuration(out string) (float64, bool) {
	m := probeDuration.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	sec, _ := strconv.ParseFloat(m[3], 64)
	return float64(h*3600+min*60) + sec, true
}

// videoDate prefers the iPhone's local time with its offset; a camera's
// creation_time is UTC and is given in the local time of this machine.
func videoDate(tags map[string]string) *string {
	if raw := tags["com.apple.quicktime.creationdate"]; raw != "" {
		if t, err := time.Parse("2006-01-02T15:04:05-0700", raw); err == nil {
			iso := t.Format("2006-01-02T15:04:05-07:00")
			return &iso
		}
	}
	if raw := tags["creation_time"]; raw != "" {
		if t, err := time.Parse(time.RFC3339Nano, raw); err == nil && t.Year() > 1970 {
			iso := t.Local().Format("2006-01-02T15:04:05")
			return &iso
		}
	}
	return nil
}

// videoLocation reads an ISO 6709 position ("+52.5200+013.4050+034.000/").
func videoLocation(tags map[string]string) (*float64, *float64) {
	for _, key := range []string{"com.apple.quicktime.location.ISO6709", "location"} {
		m := probeISO6709.FindStringSubmatch(tags[key])
		if m == nil {
			continue
		}
		lat, errLat := strconv.ParseFloat(m[1], 64)
		lon, errLon := strconv.ParseFloat(m[2], 64)
		if errLat == nil && errLon == nil {
			return &lat, &lon
		}
	}
	return nil, nil
}

// videoSize is the first video stream's size as it is shown: turned a
// quarter, width and height trade places.
func videoSize(out string) (int, int) {
	m := probeVideo.FindStringSubmatch(out)
	if m == nil {
		return 0, 0
	}
	w, _ := strconv.Atoi(m[1])
	h, _ := strconv.Atoi(m[2])
	if r := probeRotation.FindStringSubmatch(out); r != nil {
		if deg, _ := strconv.ParseFloat(r[1], 64); int(deg)%180 != 0 {
			w, h = h, w
		}
	}
	return w, h
}

// videoDateAndMeta is what a folder's background pass reads of a video:
// when it was taken, whether it has a place and how long it is.
func videoDateAndMeta(path string) (time.Time, *EntryMeta, error) {
	data, err := ExtractVideoMeta(path)
	if err != nil {
		return time.Time{}, nil, err
	}
	meta := &EntryMeta{HasGPS: data.Latitude != nil}
	meta.Duration, _ = strconv.ParseFloat(data.Tags["Duration"], 64)
	if data.DateTaken == nil {
		return time.Time{}, meta, fmt.Errorf("%s has no date", path)
	}
	for _, layout := range []string{"2006-01-02T15:04:05-07:00", "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, *data.DateTaken, time.Local); err == nil {
			return t, meta, nil
		}
	}
	return time.Time{}, meta, fmt.Errorf("%s: unreadable date %q", path, *data.DateTaken)
}
