package batchrename

import (
	"fmt"
	"regexp"
	"strings"
)

var seqPattern = regexp.MustCompile(`\{seq(?::(\d+))?\}`)

func resolvePattern(pattern string, tags map[string]string, dateTaken string, originalName string, seq int) string {
	year, month, day, hour, min, sec := parseDateComponents(dateTaken)

	result := pattern

	result = strings.ReplaceAll(result, "{YYYY}", year)
	result = strings.ReplaceAll(result, "{MM}", month)
	result = strings.ReplaceAll(result, "{DD}", day)
	result = strings.ReplaceAll(result, "{hh}", hour)
	result = strings.ReplaceAll(result, "{mm}", min)
	result = strings.ReplaceAll(result, "{ss}", sec)

	result = strings.ReplaceAll(result, "{make}", exifTagValue(tags, "Make"))
	result = strings.ReplaceAll(result, "{model}", exifTagValue(tags, "Model"))
	result = strings.ReplaceAll(result, "{lens}", exifTagValue(tags, "LensModel"))
	result = strings.ReplaceAll(result, "{filmsim}", exifTagValue(tags, "FilmSimulation"))
	result = strings.ReplaceAll(result, "{iso}", exifTagValue(tags, "ISOSpeedRatings"))
	result = strings.ReplaceAll(result, "{aperture}", formatAperture(tags))
	result = strings.ReplaceAll(result, "{focal}", formatFocal(tags))
	result = strings.ReplaceAll(result, "{shutter}", formatShutter(tags))
	result = strings.ReplaceAll(result, "{original}", originalName)

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
	if len(dateTaken) >= 19 {
		return dateTaken[0:4], dateTaken[5:7], dateTaken[8:10],
			dateTaken[11:13], dateTaken[14:16], dateTaken[17:19]
	}
	return unknown, unknown, unknown, unknown, unknown, unknown
}

func exifTagValue(tags map[string]string, key string) string {
	v, ok := tags[key]
	if !ok || v == "" {
		return "unknown"
	}
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
