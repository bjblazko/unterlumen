package batchrename

import (
	"regexp"
	"strings"
)

var (
	reUnsafe      = regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	reMultiHyphen = regexp.MustCompile(`-{2,}`)
	reMultiUnder  = regexp.MustCompile(`_{2,}`)
)

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, " ", "-")
	name = reUnsafe.ReplaceAllString(name, "")
	name = reMultiHyphen.ReplaceAllString(name, "-")
	name = reMultiUnder.ReplaceAllString(name, "_")
	name = strings.Trim(name, "-.")
	if name == "" {
		return "unnamed"
	}
	return name
}
