package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/media"
)

// What the file itself says is read and written with exiftool, which keeps
// everything else in the file — maker notes, previews, other programs' XMP.
// Each field goes into the places programs look for it, as the field table
// names them: EXIF, IPTC and the file's own XMP. A list's entries are
// written apart ("Ann; Bob" is two), with -sep.

var keywordTags = []string{"XMP-dc:Subject", "IPTC:Keywords"}

const exiftoolTimeout = 2 * time.Minute

func runExiftool(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), exiftoolTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "exiftool", args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("exiftool: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// fileValues is what one file says, from exiftool.
type fileValues struct {
	fields   map[Field]string
	keywords []string
}

// readFiles reads the fields of many files in one run of exiftool.
func readFiles(paths []string) (map[string]fileValues, error) {
	args := []string{"-j", "-charset", "iptc=UTF8", "-DateTimeOriginal", "-OffsetTimeOriginal"}
	for _, spec := range table {
		for _, tag := range spec.file {
			args = append(args, "-"+tag)
		}
	}
	for _, tag := range keywordTags {
		args = append(args, "-"+tag)
	}
	out, err := runExiftool(append(args, paths...)...)
	if err != nil && len(out) == 0 {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("exiftool gave no readable answer: %w", err)
	}
	res := make(map[string]fileValues, len(rows))
	for _, row := range rows {
		path, _ := row["SourceFile"].(string)
		res[path] = valuesFromRow(row)
	}
	return res, nil
}

func valuesFromRow(row map[string]any) fileValues {
	v := fileValues{fields: map[Field]string{}}
	if d := exifDate(text(row["DateTimeOriginal"]), text(row["OffsetTimeOriginal"])); d != "" {
		v.fields[DateTaken] = d
	}
	for _, spec := range table {
		for _, tag := range spec.file {
			if s := text(row[tagName(tag)]); s != "" {
				v.fields[spec.field] = s
				break
			}
		}
	}
	for _, tag := range keywordTags {
		if list := texts(row[tagName(tag)]); len(list) > 0 {
			v.keywords = list
			break
		}
	}
	return v
}

// tagName is the name exiftool's JSON gives a tag: without its group.
func tagName(tag string) string {
	if i := strings.Index(tag, ":"); i >= 0 {
		return tag[i+1:]
	}
	return tag
}

func text(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case []any:
		return strings.Join(texts(t), "; ")
	}
	return ""
}

func texts(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, e := range t {
			if s := text(e); s != "" {
				out = append(out, s)
			}
		}
		return out
	case nil:
		return nil
	}
	if s := text(v); s != "" {
		return []string{s}
	}
	return nil
}

// exifDate turns "2024:05:01 12:30:00" and "+02:00" into
// "2024-05-01T12:30:00+02:00".
func exifDate(d, offset string) string {
	if len(d) < 19 || strings.HasPrefix(d, "0000") {
		return ""
	}
	iso := strings.Replace(d[:10], ":", "-", 2) + "T" + d[11:19]
	return iso + offset
}

// fileArgs are exiftool's arguments for writing set, the keywords and the
// date into the file.
func fileArgs(set map[Field]string, add, remove []string) []string {
	args := fileArgsHead()
	for _, spec := range table {
		v, ok := set[spec.field]
		if !ok {
			continue
		}
		if spec.field == DateTaken {
			args = append(args, dateArgs(v)...)
			continue
		}
		for _, tag := range spec.file {
			args = append(args, "-"+tag+"="+v)
		}
	}
	for _, tag := range keywordTags {
		for _, k := range remove {
			args = append(args, "-"+tag+"-="+k)
		}
		for _, k := range add {
			// Removed first so a keyword already there is not doubled.
			args = append(args, "-"+tag+"-="+k, "-"+tag+"+="+k)
		}
	}
	return args
}

func fileArgsHead() []string {
	return []string{"-overwrite_original", "-charset", "iptc=UTF8", "-codedcharacterset=utf8", "-sep", "; "}
}

func dateArgs(iso string) []string {
	t, offset, err := splitDate(iso)
	if err != nil {
		return nil
	}
	d := t.Format("2006:01:02 15:04:05")
	args := []string{"-EXIF:DateTimeOriginal=" + d, "-EXIF:CreateDate=" + d}
	if offset != "" && offset != "Z" {
		args = append(args, "-EXIF:OffsetTimeOriginal="+offset, "-EXIF:OffsetTimeDigitized="+offset)
	}
	return args
}

// writeFile writes the change into the photo at path.
func writeFile(path string, set map[Field]string, add, remove []string) error {
	if media.IsVideo(path) {
		return fmt.Errorf("a video is never changed; its metadata goes into the NFO file beside it")
	}
	if !media.CheckExiftool() {
		return fmt.Errorf("exiftool is not installed, so nothing can be written into the file; write to the sidecar instead")
	}
	args := fileArgs(set, add, remove)
	if len(args) == len(fileArgsHead()) {
		return nil
	}
	_, err := runExiftool(append(args, path)...)
	return err
}
