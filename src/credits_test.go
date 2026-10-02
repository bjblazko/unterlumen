package main

import (
	"encoding/json"
	"io/fs"
	"path"
	"runtime/debug"
	"testing"
)

// credits is web/licenses/credits.json, which the Licenses place lists and
// whose texts travel inside the binary, as the BSD, MIT, ISC and OFL
// licenses of what is built in require.
type credits struct {
	Inside   []credit `json:"inside"`
	Programs []credit `json:"programs"`
	Data     []credit `json:"data"`
}

type credit struct {
	Name    string `json:"name"`
	Module  string `json:"module"`
	License string `json:"license"`
	Home    string `json:"home"`
	Text    string `json:"text"`
}

func readCredits(t *testing.T) credits {
	t.Helper()
	data, err := webFS.ReadFile("web/licenses/credits.json")
	if err != nil {
		t.Fatal(err)
	}
	var c credits
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("credits.json: %v", err)
	}
	return c
}

func TestCreditsListEveryModuleBuiltIn(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Skip("no build info")
	}
	listed := map[string]bool{}
	for _, c := range readCredits(t).Inside {
		if c.Module != "" {
			listed[c.Module] = true
		}
	}
	for _, dep := range info.Deps {
		if !listed[dep.Path] {
			t.Errorf("%s is built in but missing from web/licenses/credits.json", dep.Path)
		}
	}
}

func TestCreditsEveryBuiltInPartHasItsLicenseText(t *testing.T) {
	for _, c := range readCredits(t).Inside {
		if c.License == "" || c.Home == "" || c.Text == "" {
			t.Errorf("%s: license, home and text are required", c.Name)
			continue
		}
		if _, err := fs.Stat(webFS, path.Join("web/licenses", c.Text)); err != nil {
			t.Errorf("%s: %v", c.Name, err)
		}
	}
}
