package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

type batchRenameRequest struct {
	Files   []string `json:"files"`
	Pattern string   `json:"pattern"`
}

type batchRenameMapping struct {
	File    string `json:"file"`
	NewName string `json:"newName"`
	Error   string `json:"error,omitempty"`
}

type batchRenamePreviewResponse struct {
	Mappings  []batchRenameMapping `json:"mappings"`
	Conflicts int                  `json:"conflicts"`
}

type batchRenameResult struct {
	File    string `json:"file"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type batchRenameExecuteResponse struct {
	Results []batchRenameResult `json:"results"`
}

var seqPattern = regexp.MustCompile(`\{seq(?::(\d+))?\}`)

func handleBatchRenamePreview(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req batchRenameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if len(req.Files) == 0 {
			http.Error(w, "No files specified", http.StatusBadRequest)
			return
		}
		if req.Pattern == "" {
			http.Error(w, "No pattern specified", http.StatusBadRequest)
			return
		}

		mappings := resolveBatchMappings(root, req.Files, req.Pattern)
		conflicts := applyConflictSuffixes(mappings)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(batchRenamePreviewResponse{
			Mappings:  mappings,
			Conflicts: conflicts,
		})
	}
}

func handleBatchRenameExecute(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req batchRenameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if len(req.Files) == 0 {
			http.Error(w, "No files specified", http.StatusBadRequest)
			return
		}
		if req.Pattern == "" {
			http.Error(w, "No pattern specified", http.StatusBadRequest)
			return
		}

		mappings := resolveBatchMappings(root, req.Files, req.Pattern)
		applyConflictSuffixes(mappings)

		// Validate: check no mapping has an error
		for _, m := range mappings {
			if m.Error != "" {
				results := make([]batchRenameResult, len(mappings))
				for i, mm := range mappings {
					results[i] = batchRenameResult{File: mm.File, Success: mm.Error == "", Error: mm.Error}
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(batchRenameExecuteResponse{Results: results})
				return
			}
		}

		// Check no destination collides with existing files outside the rename set
		renameSet := make(map[string]struct{})
		for _, m := range mappings {
			abs, ok := safePath(root, m.File)
			if ok {
				renameSet[abs] = struct{}{}
			}
		}

		for _, m := range mappings {
			abs, ok := safePath(root, m.File)
			if !ok {
				continue
			}
			dir := filepath.Dir(abs)
			destPath := filepath.Join(dir, m.NewName)
			if _, exists := renameSet[destPath]; exists {
				continue // it's part of the rename set, will be handled
			}
			if _, err := os.Stat(destPath); err == nil {
				results := make([]batchRenameResult, len(mappings))
				for i, mm := range mappings {
					results[i] = batchRenameResult{File: mm.File, Error: fmt.Sprintf("destination '%s' already exists", mm.NewName)}
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(batchRenameExecuteResponse{Results: results})
				return
			}
		}

		// Two-pass rename: first to temp names, then to final names
		type renamePair struct {
			absPath  string
			dir      string
			tempName string
			newName  string
			relFile  string
		}
		pairs := make([]renamePair, 0, len(mappings))
		for i, m := range mappings {
			abs, ok := safePath(root, m.File)
			if !ok {
				continue
			}
			dir := filepath.Dir(abs)
			ext := filepath.Ext(abs)
			tempName := fmt.Sprintf("_batch_tmp_%03d_%s%s", i, filepath.Base(abs), ext)
			pairs = append(pairs, renamePair{
				absPath:  abs,
				dir:      dir,
				tempName: tempName,
				newName:  m.NewName,
				relFile:  m.File,
			})
		}

		results := make([]batchRenameResult, len(pairs))
		dirsToInvalidate := make(map[string]struct{})

		// Pass 1: rename to temp
		for i, p := range pairs {
			tempPath := filepath.Join(p.dir, p.tempName)
			if err := os.Rename(p.absPath, tempPath); err != nil {
				results[i] = batchRenameResult{File: p.relFile, Error: fmt.Sprintf("temp rename failed: %v", err)}
			}
		}

		// Pass 2: rename to final
		for i, p := range pairs {
			if results[i].Error != "" {
				continue // skip if pass 1 failed
			}
			tempPath := filepath.Join(p.dir, p.tempName)
			finalPath := filepath.Join(p.dir, p.newName)
			if err := os.Rename(tempPath, finalPath); err != nil {
				results[i] = batchRenameResult{File: p.relFile, Error: fmt.Sprintf("final rename failed: %v", err)}
				// Try to restore original name
				os.Rename(tempPath, p.absPath)
			} else {
				results[i] = batchRenameResult{File: p.relFile, Success: true}
				dirsToInvalidate[p.dir] = struct{}{}
			}
		}

		// Invalidate cache
		for dir := range dirsToInvalidate {
			cache.Invalidate(dir)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(batchRenameExecuteResponse{Results: results})
	}
}

func resolveBatchMappings(root string, files []string, pattern string) []batchRenameMapping {
	mappings := make([]batchRenameMapping, len(files))

	for i, file := range files {
		abs, ok := safePath(root, file)
		if !ok {
			mappings[i] = batchRenameMapping{File: file, Error: "invalid path"}
			continue
		}

		originalName := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
		ext := strings.ToLower(filepath.Ext(abs))

		// Extract EXIF
		exifData, err := media.ExtractAllEXIF(abs)
		var tags map[string]string
		if err == nil && exifData != nil {
			tags = exifData.Tags
		} else {
			tags = make(map[string]string)
		}

		// Get date taken
		var dateTaken string
		if exifData != nil && exifData.DateTaken != nil {
			dateTaken = *exifData.DateTaken
		}

		resolved := resolvePattern(pattern, tags, dateTaken, originalName, i+1)
		sanitized := sanitizeFilename(resolved)
		newName := sanitized + ext

		mappings[i] = batchRenameMapping{File: file, NewName: newName}
	}

	return mappings
}

func applyConflictSuffixes(mappings []batchRenameMapping) int {
	// Group by newName to detect duplicates
	nameCount := make(map[string][]int)
	for i, m := range mappings {
		if m.Error != "" {
			continue
		}
		nameCount[m.NewName] = append(nameCount[m.NewName], i)
	}

	conflicts := 0
	for _, indices := range nameCount {
		if len(indices) <= 1 {
			continue
		}
		conflicts += len(indices)
		for j, idx := range indices {
			m := &mappings[idx]
			ext := filepath.Ext(m.NewName)
			base := strings.TrimSuffix(m.NewName, ext)
			m.NewName = fmt.Sprintf("%s_%03d%s", base, j+1, ext)
		}
	}

	return conflicts
}

func resolvePattern(pattern string, tags map[string]string, dateTaken string, originalName string, seq int) string {
	year, month, day, hour, min, sec := parseDateComponents(dateTaken)

	result := pattern

	// Date placeholders
	result = strings.ReplaceAll(result, "{YYYY}", year)
	result = strings.ReplaceAll(result, "{MM}", month)
	result = strings.ReplaceAll(result, "{DD}", day)
	result = strings.ReplaceAll(result, "{hh}", hour)
	result = strings.ReplaceAll(result, "{mm}", min)
	result = strings.ReplaceAll(result, "{ss}", sec)

	// EXIF placeholders
	result = strings.ReplaceAll(result, "{make}", exifTagValue(tags, "Make"))
	result = strings.ReplaceAll(result, "{model}", exifTagValue(tags, "Model"))
	result = strings.ReplaceAll(result, "{lens}", exifTagValue(tags, "LensModel"))
	result = strings.ReplaceAll(result, "{filmsim}", exifTagValue(tags, "FilmSimulation"))
	result = strings.ReplaceAll(result, "{iso}", exifTagValue(tags, "ISOSpeedRatings"))
	result = strings.ReplaceAll(result, "{aperture}", formatAperture(tags))
	result = strings.ReplaceAll(result, "{focal}", formatFocal(tags))
	result = strings.ReplaceAll(result, "{shutter}", formatShutter(tags))
	result = strings.ReplaceAll(result, "{original}", originalName)

	// Sequence placeholder with optional width
	result = seqPattern.ReplaceAllStringFunc(result, func(match string) string {
		sub := seqPattern.FindStringSubmatch(match)
		width := 3
		if len(sub) > 1 && sub[1] != "" {
			fmt.Sscanf(sub[1], "%d", &width)
		}
		return fmt.Sprintf("%0*d", width, seq)
	})

	return result
}

func parseDateComponents(dateTaken string) (year, month, day, hour, min, sec string) {
	unknown := "unknown"
	if dateTaken == "" {
		return unknown, unknown, unknown, unknown, unknown, unknown
	}

	// ISO 8601: "2026-03-20T14:07:42" or "2026-03-20T14:07:42+01:00"
	// Minimum length: "2026-03-20T14:07:42" = 19 chars
	if len(dateTaken) >= 19 {
		year = dateTaken[0:4]
		month = dateTaken[5:7]
		day = dateTaken[8:10]
		hour = dateTaken[11:13]
		min = dateTaken[14:16]
		sec = dateTaken[17:19]
		return
	}

	return unknown, unknown, unknown, unknown, unknown, unknown
}

func exifTagValue(tags map[string]string, key string) string {
	v, ok := tags[key]
	if !ok || v == "" {
		return "unknown"
	}
	// EXIF values are often wrapped in quotes
	v = strings.Trim(v, `"`)
	if v == "" {
		return "unknown"
	}
	return v
}

func formatAperture(tags map[string]string) string {
	v := exifTagValue(tags, "FNumber")
	if v == "unknown" {
		return v
	}
	// FNumber might be a rational like "14/10", parse it
	v = strings.Trim(v, `"`)
	if strings.Contains(v, "/") {
		var num, den float64
		fmt.Sscanf(v, "%f/%f", &num, &den)
		if den > 0 {
			aperture := num / den
			if aperture == float64(int(aperture)) {
				return fmt.Sprintf("f%.0f", aperture)
			}
			return fmt.Sprintf("f%.1f", aperture)
		}
	}
	return "f" + v
}

func formatFocal(tags map[string]string) string {
	v := exifTagValue(tags, "FocalLength")
	if v == "unknown" {
		return v
	}
	v = strings.Trim(v, `"`)
	if strings.Contains(v, "/") {
		var num, den float64
		fmt.Sscanf(v, "%f/%f", &num, &den)
		if den > 0 {
			focal := num / den
			if focal == float64(int(focal)) {
				return fmt.Sprintf("%.0fmm", focal)
			}
			return fmt.Sprintf("%.1fmm", focal)
		}
	}
	return v + "mm"
}

func formatShutter(tags map[string]string) string {
	v := exifTagValue(tags, "ExposureTime")
	if v == "unknown" {
		return v
	}
	v = strings.Trim(v, `"`)
	if strings.Contains(v, "/") {
		var num, den float64
		fmt.Sscanf(v, "%f/%f", &num, &den)
		if den > 0 {
			if num == 1 {
				return fmt.Sprintf("1-%0.fs", den)
			}
			speed := num / den
			if speed >= 1 {
				return fmt.Sprintf("%.0fs", speed)
			}
			return fmt.Sprintf("1-%.0fs", 1/speed)
		}
	}
	return v + "s"
}

func sanitizeFilename(name string) string {
	// Replace spaces with hyphens
	name = strings.ReplaceAll(name, " ", "-")

	// Remove characters not in [a-zA-Z0-9._-]
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	name = re.ReplaceAllString(name, "")

	// Collapse consecutive hyphens into one
	reHyphen := regexp.MustCompile(`-{2,}`)
	name = reHyphen.ReplaceAllString(name, "-")

	// Collapse consecutive underscores into one
	reUnderscore := regexp.MustCompile(`_{2,}`)
	name = reUnderscore.ReplaceAllString(name, "_")

	// Trim leading/trailing hyphens and dots
	name = strings.Trim(name, "-.")

	if name == "" {
		return "unnamed"
	}

	return name
}
