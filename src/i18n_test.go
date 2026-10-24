package main

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The interface is written in English and translated by web/i18n/<lang>.js,
// whose keys are the English texts (i18n.js). A text that reaches t() — or
// a helper that passes it there — without a German entry shows in English
// in a German interface.

var (
	jsString = `'((?:[^'\\\n]|\\.)*)'`
	// The forms a text reaches t() in: t('…'), t(cond ? '…' : '…'), and
	// the helpers that translate what they are given.
	translatedTexts = []*regexp.Regexp{
		regexp.MustCompile(`(?:^|[^\w.])t\(\s*` + jsString),
		regexp.MustCompile(`(?:^|[^\w.])t\([^()'\n]*?\?\s*` + jsString + `\s*:\s*` + jsString),
		regexp.MustCompile(`count(?:Text|HTML)\([^,]+,\s*` + jsString + `(?:\s*,\s*` + jsString + `)?`),
		regexp.MustCompile(`(?:\.row\(|badgeRow\(|\.section\(|metaGroup\(|legendStepHTML\(|panelToggleButton\()\s*` + jsString),
		regexp.MustCompile(`addTag\([^,]+,[^,]+,[^,]+,\s*'[^']*',\s*` + jsString),
	}
	staticText   = regexp.MustCompile(`data-i18n(?:-attr="[^"]*")?[^>]*>\s*([^<]+?)\s*<`)
	placeholders = regexp.MustCompile(`\{\w+\}`)
	jsEscape     = regexp.MustCompile(`\\(u[0-9a-fA-F]{4}|.)`)
)

// unescapeJS turns the inside of a single-quoted JavaScript string into the
// text it stands for.
func unescapeJS(s string) string {
	return jsEscape.ReplaceAllStringFunc(s, func(m string) string {
		if len(m) == 6 && m[1] == 'u' {
			var r rune
			for _, c := range m[2:] {
				r = r*16 + rune(strings.IndexRune("0123456789abcdef", c|0x20))
			}
			return string(r)
		}
		return m[1:]
	})
}

func readGermanWords(t *testing.T) map[string]string {
	t.Helper()
	data, err := webFS.ReadFile("web/i18n/de.js")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	start := strings.Index(s, ".de = ")
	if start < 0 {
		t.Fatal("de.js does not assign .de")
	}
	var words map[string]string
	if err := json.Unmarshal([]byte(strings.TrimRight(strings.TrimSpace(s[start+6:]), ";")), &words); err != nil {
		t.Fatalf("de.js: %v", err)
	}
	return words
}

// interfaceTexts is every English text the interface passes to t(), with
// the file it was found in first.
func interfaceTexts(t *testing.T) map[string]string {
	t.Helper()
	texts := map[string]string{}
	add := func(text, file string) {
		if strings.ContainsFunc(text, func(r rune) bool { return r >= 'A' && r <= 'z' }) {
			if _, ok := texts[text]; !ok {
				texts[text] = file
			}
		}
	}
	err := fs.WalkDir(webFS, "web/js", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".js") {
			return err
		}
		data, err := webFS.ReadFile(p)
		if err != nil {
			return err
		}
		for _, re := range translatedTexts {
			for _, m := range re.FindAllStringSubmatch(string(data), -1) {
				for _, g := range m[1:] {
					add(unescapeJS(g), p)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	html, err := webFS.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range staticText.FindAllStringSubmatch(string(html), -1) {
		add(m[1], "web/index.html")
	}
	return texts
}

func TestEveryInterfaceTextHasAGermanWord(t *testing.T) {
	words := readGermanWords(t)
	texts := interfaceTexts(t)
	var missing []string
	for text, file := range texts {
		if _, ok := words[text]; !ok {
			missing = append(missing, file+": "+text)
		}
	}
	slices.Sort(missing)
	for _, m := range missing {
		t.Error("no German for", m)
	}
	t.Logf("%d interface texts, %d German words", len(texts), len(words))
}

func TestGermanWordsKeepTheirPlaceholders(t *testing.T) {
	for en, de := range readGermanWords(t) {
		a, b := placeholders.FindAllString(en, -1), placeholders.FindAllString(de, -1)
		slices.Sort(a)
		slices.Sort(b)
		if !slices.Equal(slices.Compact(a), slices.Compact(b)) {
			t.Errorf("%q → %q: placeholders %v, %v", en, de, a, b)
		}
	}
}
