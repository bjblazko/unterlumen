package metadata

import (
	"errors"
	"maps"
	"slices"
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

// Plan is what a change writes to one file: the fields with the date worked
// out from the file's own, or why the file is skipped.
type Plan struct {
	Set     map[Field]string `json:"set"`
	Skipped string           `json:"skipped,omitempty"`
}

// PlanFor works out what change writes to a file that says now what cur
// says.
func PlanFor(change Change, cur Current) Plan {
	set := maps.Clone(change.Set)
	if set == nil {
		set = map[Field]string{}
	}
	delete(set, DateTaken) // only through a DateFix
	if change.Date != nil {
		d, err := change.Date.Apply(cur.Fields[DateTaken].Value)
		if err != nil {
			if len(set) == 0 && len(change.KeywordsAdd) == 0 && len(change.KeywordsRemove) == 0 {
				return Plan{Set: set, Skipped: skipReason(err)}
			}
		} else {
			set[DateTaken] = d
		}
	}
	return Plan{Set: set}
}

func skipReason(err error) string {
	if errors.Is(err, ErrNoDate) {
		return "no date taken"
	}
	return err.Error()
}

// Write writes the plan and the keywords of change into target for the file
// at path. A video always goes to its NFO file.
func Write(path string, target Target, plan Plan, change Change) error {
	if plan.Skipped != "" {
		return nil
	}
	add, remove := unique(change.KeywordsAdd), unique(change.KeywordsRemove)
	if media.IsVideo(path) {
		return writeNFO(path, plan.Set, add, remove)
	}
	if target == TargetFile {
		return writeFile(path, plan.Set, add, remove)
	}
	return writeSidecar(path, plan.Set, add, remove)
}

func writeSidecar(path string, set map[Field]string, add, remove []string) error {
	if title, ok := set[Title]; ok {
		if err := media.WriteTitle(path, title); err != nil {
			return err
		}
	}
	props := map[media.Prop]string{}
	for f, p := range sidecarProps {
		if v, ok := set[f]; ok {
			props[p] = v
		}
	}
	if d, ok := set[DateTaken]; ok {
		props[media.PropDateCreated] = d
	}
	if len(props) > 0 {
		if err := media.WriteSidecarProps(path, props); err != nil {
			return err
		}
	}
	if len(add) > 0 || len(remove) > 0 {
		return media.WriteKeywords(path, add, remove)
	}
	return nil
}

// unique is list without empty or repeated keywords, in its order.
func unique(list []string) []string {
	var out []string
	for _, k := range list {
		if k = strings.TrimSpace(k); k != "" && !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}
