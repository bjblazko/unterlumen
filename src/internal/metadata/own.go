package metadata

import "strings"

// Own fields are key and value for what no standard names (ADR-0061). They
// live only in the sidecar's ul:Fields, since exiftool knows no free
// namespace, and never beside a video. One can also be a keyword, so other
// programs find it: a machine tag "key=value", with "ul:" before a key that
// has no namespace of its own ("film:stock=Portra" stays, "stock=Portra"
// becomes "ul:stock=Portra"). Whether a field is one is read from the
// keywords themselves, so there is no second record to keep in step.

// MachineTag is the keyword an own field is mirrored as.
func MachineTag(key, value string) string {
	if !strings.Contains(key, ":") {
		key = "ul:" + key
	}
	return key + "=" + value
}

// planOwn works out, for one file, the own fields to write and the keywords
// their mirroring adds and removes.
func planOwn(plan *Plan, change Change, cur Current) {
	if cur.Video || len(change.Own) == 0 {
		return
	}
	for key, oc := range change.Own {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		old := cur.Own[key]
		value := old
		if oc.Value != nil {
			value = strings.TrimSpace(*oc.Value)
		}
		if value != old {
			if plan.Own == nil {
				plan.Own = map[string]string{}
			}
			plan.Own[key] = value
		}
		mirrored := old != "" && hasKeyword(cur.Keywords, MachineTag(key, old))
		mirror := mirrored
		if oc.Mirror != nil {
			mirror = *oc.Mirror
		}
		if mirrored && (!mirror || value != old) {
			plan.KeywordsRemove = appendUnique(plan.KeywordsRemove, MachineTag(key, old))
		}
		if mirror && value != "" && (!mirrored || value != old) {
			plan.KeywordsAdd = appendUnique(plan.KeywordsAdd, MachineTag(key, value))
		}
	}
}

func hasKeyword(list []string, k string) bool {
	for _, e := range list {
		if e == k {
			return true
		}
	}
	return false
}

func appendUnique(list []string, k string) []string {
	if hasKeyword(list, k) {
		return list
	}
	return append(list, k)
}
