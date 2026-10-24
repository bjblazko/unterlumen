package things

import (
	"strings"
	"testing"
)

// Every language names every thing and every category: a missing name
// would show the English one in the middle of another language.
func TestEveryLanguageNamesEveryThing(t *testing.T) {
	for _, code := range LanguageCodes() {
		l := Languages[code]
		for _, th := range Vocabulary {
			if _, ok := l.names[th.Name]; !ok {
				t.Errorf("%s has no name for %q", code, th.Name)
			}
			if code != "en" && l.Category(th.Category) == string(th.Category) {
				t.Errorf("%s has no name for the category %q", code, th.Category)
			}
		}
		for id := range l.names {
			if _, ok := indexOf[id]; !ok {
				t.Errorf("%s names %q, which is not a thing", code, id)
			}
		}
	}
}

// A keyword is read back to its thing whatever language wrote it, so no
// name may stand for two things, in any two languages.
func TestEveryNameIsOneThing(t *testing.T) {
	owner := map[string]string{}
	for _, code := range LanguageCodes() {
		for id, name := range Languages[code].names {
			key := strings.ToLower(name)
			if other, ok := owner[key]; ok && other != id {
				t.Errorf("%q (%s) names both %q and %q", name, code, other, id)
			}
			owner[key] = id
		}
	}
}
