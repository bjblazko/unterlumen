package batchrename

import (
	"fmt"
	"path/filepath"
	"strings"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

func resolveBatchMappings(root string, files []string, pattern string) []batchRenameMapping {
	mappings := make([]batchRenameMapping, len(files))
	for i, file := range files {
		mappings[i] = resolveOneMapping(root, file, pattern, i+1)
	}
	return mappings
}

func resolveOneMapping(root, file, pattern string, seq int) batchRenameMapping {
	abs, ok := pathguard.SafePath(root, file)
	if !ok {
		return batchRenameMapping{File: file, Error: "invalid path"}
	}

	originalName := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	ext := strings.ToLower(filepath.Ext(abs))

	exifData, err := media.ExtractAllEXIF(abs)
	var tags map[string]string
	var dateTaken string
	if err == nil && exifData != nil {
		tags = exifData.Tags
		if exifData.DateTaken != nil {
			dateTaken = *exifData.DateTaken
		}
	} else {
		tags = make(map[string]string)
	}

	resolved := resolvePattern(pattern, tags, dateTaken, originalName, seq)
	newName := sanitizeFilename(resolved) + ext
	return batchRenameMapping{File: file, NewName: newName}
}

func applyConflictSuffixes(mappings []batchRenameMapping) int {
	nameCount := make(map[string][]int)
	for i, m := range mappings {
		if m.Error == "" {
			nameCount[m.NewName] = append(nameCount[m.NewName], i)
		}
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
