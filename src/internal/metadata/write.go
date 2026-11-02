package metadata

import (
	"errors"
	"maps"
	"slices"
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

// Plan is what a change writes to one file: the fields with the date and the
// lists worked out from the file's own, the keywords to add and remove, its
// own fields, or why the file is skipped.
type Plan struct {
	Set            map[Field]string  `json:"set"`
	KeywordsAdd    []string          `json:"keywordsAdd,omitempty"`
	KeywordsRemove []string          `json:"keywordsRemove,omitempty"`
	Own            map[string]string `json:"own,omitempty"`
	Skipped        string            `json:"skipped,omitempty"`
}

// NeedsCurrent says whether working out the change needs what the file says
// now; a rating from a key does not, and stays quick.
func NeedsCurrent(change Change) bool {
	return change.Date != nil || len(change.Add) > 0 || len(change.Remove) > 0 || len(change.Own) > 0
}

func (p Plan) empty() bool {
	return len(p.Set) == 0 && len(p.KeywordsAdd) == 0 && len(p.KeywordsRemove) == 0 && len(p.Own) == 0
}

// PlanFor works out what change writes to a file that says now what cur
// says.
func PlanFor(change Change, cur Current) Plan {
	set := maps.Clone(change.Set)
	if set == nil {
		set = map[Field]string{}
	}
	delete(set, DateTaken) // only through a DateFix
	planLists(set, change, cur)
	plan := Plan{Set: set, KeywordsAdd: unique(change.KeywordsAdd), KeywordsRemove: unique(change.KeywordsRemove)}
	planOwn(&plan, change, cur)
	if change.Date != nil {
		d, err := change.Date.Apply(cur.Fields[DateTaken].Value)
		if err != nil {
			if plan.empty() {
				plan.Skipped = skipReason(err)
			}
		} else {
			set[DateTaken] = d
		}
	}
	return plan
}

// planLists works out the entries of each list field the change adds to or
// removes from, starting from what the file has; a list that stays as it is
// is not written.
func planLists(set map[Field]string, change Change, cur Current) {
	for _, f := range Fields {
		add, remove := unique(change.Add[f]), unique(change.Remove[f])
		if len(add) == 0 && len(remove) == 0 {
			continue
		}
		have := media.SplitList(cur.Fields[f].Value)
		list := slices.DeleteFunc(slices.Clone(have), func(e string) bool { return slices.Contains(remove, e) })
		for _, e := range add {
			if !slices.Contains(list, e) {
				list = append(list, e)
			}
		}
		if !slices.Equal(list, have) {
			set[f] = strings.Join(list, "; ")
		}
	}
}

func skipReason(err error) string {
	if errors.Is(err, ErrNoDate) {
		return "no date taken"
	}
	return err.Error()
}

// Write writes the plan into target for the file at path. A video always
// goes to its NFO file; own fields always to the sidecar.
func Write(path string, target Target, plan Plan) error {
	if plan.Skipped != "" {
		return nil
	}
	add, remove := plan.KeywordsAdd, plan.KeywordsRemove
	if media.IsVideo(path) {
		return writeNFO(path, plan.Set, add, remove)
	}
	if err := writeOwn(path, plan.Own); err != nil {
		return err
	}
	if target == TargetFile {
		return writeFile(path, plan.Set, add, remove)
	}
	return writeSidecar(path, plan.Set, add, remove)
}

func writeOwn(path string, own map[string]string) error {
	for _, key := range slices.Sorted(maps.Keys(own)) {
		if err := media.WriteField(path, key, own[key]); err != nil {
			return err
		}
	}
	return nil
}

func writeSidecar(path string, set map[Field]string, add, remove []string) error {
	if title, ok := set[Title]; ok {
		if err := media.WriteTitle(path, title); err != nil {
			return err
		}
	}
	props := map[media.Prop]string{}
	for _, spec := range table {
		if v, ok := set[spec.field]; ok && spec.prop != (media.Prop{}) {
			props[spec.prop] = v
		}
	}
	if d, ok := set[DateTaken]; ok {
		props[propDateCreated] = d
	}
	if len(props) > 0 {
		if err := media.WriteSidecarProps(path, sidecarProps, props); err != nil {
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
