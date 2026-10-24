package things

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// A thing's ID is its English name (Vocabulary); every other language gives
// it a name and words it is also searched by (ADR-0056). The names, not the
// other words, are what Unterlumen writes as keywords and reads back.

//go:embed lexicon/*.json
var lexiconFiles embed.FS

// lexiconFile is one language's file.
type lexiconFile struct {
	Categories map[string]string   `json:"categories"`
	Things     map[string][]string `json:"things"` // ID → its name, then other words
}

// Language is the words of one language for the things.
type Language struct {
	Code       string
	names      map[string]string   // ID → name
	also       map[string][]string // ID → other words
	categories map[string]string
}

// Languages are the languages things have words in, English first.
var Languages = loadLanguages()

func loadLanguages() map[string]*Language {
	out := map[string]*Language{"en": english()}
	entries, _ := lexiconFiles.ReadDir("lexicon")
	for _, e := range entries {
		code := strings.TrimSuffix(e.Name(), ".json")
		data, err := lexiconFiles.ReadFile("lexicon/" + e.Name())
		if err != nil {
			panic(err)
		}
		var f lexiconFile
		if err := json.Unmarshal(data, &f); err != nil {
			panic(fmt.Sprintf("lexicon/%s: %v", e.Name(), err))
		}
		l := &Language{Code: code, names: map[string]string{}, also: map[string][]string{}, categories: f.Categories}
		for id, words := range f.Things {
			if len(words) > 0 {
				l.names[id], l.also[id] = words[0], words[1:]
			}
		}
		out[code] = l
	}
	return out
}

func english() *Language {
	l := &Language{Code: "en", names: map[string]string{}, also: map[string][]string{}, categories: map[string]string{}}
	for _, t := range Vocabulary {
		l.names[t.Name] = t.Name
	}
	return l
}

// LanguageOf is the language code names, English when there are no words
// for it.
func LanguageOf(code string) *Language {
	if l, ok := Languages[code]; ok {
		return l
	}
	return Languages["en"]
}

// Name is the thing's name in the language; its ID when the language lacks
// one.
func (l *Language) Name(id string) string {
	if n, ok := l.names[id]; ok {
		return n
	}
	return id
}

// Also are the other words the thing is searched by in the language.
func (l *Language) Also(id string) []string { return l.also[id] }

// Category is the category's name in the language.
func (l *Language) Category(c Category) string {
	if n, ok := l.categories[string(c)]; ok {
		return n
	}
	return string(c)
}

// Keyword is what the thing is written as in a sidecar of a library in this
// language.
func (l *Language) Keyword(id string) string { return l.Name(id) }

// Keywords are the thing's names in every language: what may stand in a
// sidecar for it, written by any installation in any language.
func (l *Language) Keywords(id string) []string {
	seen := map[string]bool{}
	var out []string
	for _, code := range languageCodes() {
		n := Languages[code].Name(id)
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// Codes lists the language codes, English first, the others in order.
func languageCodes() []string {
	codes := []string{"en"}
	for c := range Languages {
		if c != "en" {
			codes = append(codes, c)
		}
	}
	sort.Strings(codes[1:])
	return codes
}

// LanguageCodes lists the languages things have words in, English first.
func LanguageCodes() []string { return languageCodes() }
