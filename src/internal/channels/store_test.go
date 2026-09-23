package channels

import (
	"path/filepath"
	"testing"
)

func TestNewStoreSeparatesConfigAndOutputDirs(t *testing.T) {
	configDir := t.TempDir()
	outputDir := t.TempDir()

	s := NewStore(configDir, outputDir)

	ch := &Channel{Slug: "website", Name: "Website", Format: "jpeg", Quality: 85}
	if err := s.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}

	wantConfigPath := filepath.Join(configDir, "channels.json")
	if s.path != wantConfigPath {
		t.Fatalf("config path = %q, want %q", s.path, wantConfigPath)
	}

	got, err := s.Get("website")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Website" {
		t.Fatalf("Get returned %+v, want Name=Website", got)
	}

	wantOutputDir := filepath.Join(outputDir, "channels", "website")
	if gotOutputDir := s.OutputDir("website"); gotOutputDir != wantOutputDir {
		t.Fatalf("OutputDir = %q, want %q", gotOutputDir, wantOutputDir)
	}
}

func TestOutputDirRespectsChannelOverride(t *testing.T) {
	configDir := t.TempDir()
	outputDir := t.TempDir()
	customPath := filepath.Join(t.TempDir(), "custom-output")

	s := NewStore(configDir, outputDir)
	ch := &Channel{Slug: "website", Name: "Website", OutputPath: customPath}
	if err := s.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got := s.OutputDir("website"); got != customPath {
		t.Fatalf("OutputDir = %q, want override %q", got, customPath)
	}
}

func TestOutputDirFallsBackToBuiltinChannel(t *testing.T) {
	configDir := t.TempDir()
	outputDir := t.TempDir()

	s := NewStore(configDir, outputDir)

	wantOutputDir := filepath.Join(outputDir, "channels", "mastodon")
	if got := s.OutputDir("mastodon"); got != wantOutputDir {
		t.Fatalf("OutputDir = %q, want %q", got, wantOutputDir)
	}
}

// A channel's OutputPath comes from the folder picker, which returns paths
// relative to the browse root. Resolving them against the process's working
// directory instead makes the same configuration behave differently depending
// on where the server was started from — the app finds its galleries when
// launched from "/" and finds nothing when launched from a project folder.
func TestOutputDir_ResolvesRelativePathAgainstBoundary(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, dir).WithBoundary("/srv/photos")
	if err := store.Save(&Channel{Slug: "site", Name: "Site", OutputPath: "Users/someone/Pictures/out"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := store.OutputDir("site")
	if want := "/srv/photos/Users/someone/Pictures/out"; got != want {
		t.Errorf("OutputDir = %q, want %q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("OutputDir returned a relative path (%q); every caller treats it as a filesystem path", got)
	}
}

func TestOutputDir_KeepsAbsolutePathAndDefault(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, dir).WithBoundary("/srv/photos")
	if err := store.Save(&Channel{Slug: "abs", Name: "Abs", OutputPath: "/var/www/out"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Save(&Channel{Slug: "plain", Name: "Plain"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got := store.OutputDir("abs"); got != "/var/www/out" {
		t.Errorf("absolute OutputPath = %q, want it unchanged", got)
	}
	if got, want := store.OutputDir("plain"), filepath.Join(dir, "channels", "plain"); got != want {
		t.Errorf("default OutputDir = %q, want %q", got, want)
	}
}

// Saving normalises a folder-picker path to an absolute one, so what the
// destinations list shows is the path that is actually used.
func TestSave_StoresOutputPathAbsolute(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, dir).WithBoundary("/srv/photos")
	if err := store.Save(&Channel{Slug: "site", Name: "Site", OutputPath: "Users/someone/out"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	ch, err := store.Get("site")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if want := "/srv/photos/Users/someone/out"; ch.OutputPath != want {
		t.Errorf("stored OutputPath = %q, want %q", ch.OutputPath, want)
	}
}

// A destination with no custom folder keeps none: an empty path means "the
// default under the library directory", not the browse root.
func TestSave_LeavesEmptyOutputPathEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, dir).WithBoundary("/srv/photos")
	if err := store.Save(&Channel{Slug: "plain", Name: "Plain"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	ch, _ := store.Get("plain")
	if ch.OutputPath != "" {
		t.Errorf("OutputPath = %q, want it left empty", ch.OutputPath)
	}
	if got, want := store.OutputDir("plain"), filepath.Join(dir, "channels", "plain"); got != want {
		t.Errorf("OutputDir = %q, want the default %q", got, want)
	}
}
