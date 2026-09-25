package channels

import (
	"os"
	"path/filepath"
	"strings"
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

func TestAlbumRegisterDirLivesBesideChannelsJSON(t *testing.T) {
	shared, lib := t.TempDir(), t.TempDir()
	s := NewStore(shared, lib)
	want := filepath.Join(shared, "albums", "website")
	if got := s.AlbumRegisterDir("website"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if s.ConfigDir() != shared {
		t.Errorf("ConfigDir = %q, want %q", s.ConfigDir(), shared)
	}
}

// --- The output folder belongs to one installation ---

func writeSharedChannels(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "channels.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// channels.json is shared, and a folder is a path on one machine, so a saved
// output folder must not end up in it.
func TestSave_KeepsOutputPathOutOfTheSharedFile(t *testing.T) {
	shared := t.TempDir()
	s := NewStore(shared, t.TempDir())
	if err := s.Save(&Channel{Slug: "site", Name: "Site", OutputPath: "/Users/someone/out"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(shared, "channels.json"))
	if strings.Contains(string(raw), "/Users/someone/out") || strings.Contains(string(raw), "outputPath") {
		t.Errorf("channels.json holds a machine-local path:\n%s", raw)
	}
	if got := s.OutputDir("site"); got != "/Users/someone/out" {
		t.Errorf("OutputDir = %q", got)
	}
}

func TestOutputPath_IsPerInstallation(t *testing.T) {
	shared := t.TempDir()
	mac := NewStore(shared, t.TempDir())
	nas := NewStore(shared, t.TempDir())
	if err := mac.Save(&Channel{Slug: "site", Name: "Site", OutputPath: "/Users/someone/out"}); err != nil {
		t.Fatal(err)
	}
	ch, err := nas.Get("site")
	if err != nil {
		t.Fatal(err)
	}
	if ch.OutputPath != "" {
		t.Errorf("the other installation sees %q, want nothing", ch.OutputPath)
	}
	if got, want := nas.OutputDir("site"), filepath.Join(nas.outputBase, "channels", "site"); got != want {
		t.Errorf("OutputDir = %q, want the default %q", got, want)
	}
}

// Destinations saved before this change carry their folder in the shared file.
// Where that folder exists it is this installation's; where it does not, it is
// someone else's and the default applies.
func TestOutputDir_UsesALegacySharedPathOnlyWhereItExists(t *testing.T) {
	shared, here := t.TempDir(), t.TempDir()
	writeSharedChannels(t, shared, `[
	  {"slug":"local","name":"Local","outputPath":"`+here+`"},
	  {"slug":"foreign","name":"Foreign","outputPath":"/Users/nobody/there/out"}
	]`)
	s := NewStore(shared, t.TempDir())
	if got := s.OutputDir("local"); got != here {
		t.Errorf("existing legacy path: OutputDir = %q, want %q", got, here)
	}
	if got, want := s.OutputDir("foreign"), filepath.Join(s.outputBase, "channels", "foreign"); got != want {
		t.Errorf("foreign legacy path: OutputDir = %q, want %q", got, want)
	}
}

// Saving on one installation must not take away the folder another installation
// still reads from the shared file.
func TestSave_LeavesALegacySharedValueAlone(t *testing.T) {
	shared, here := t.TempDir(), t.TempDir()
	writeSharedChannels(t, shared, `[{"slug":"local","name":"Local","outputPath":"`+here+`"}]`)
	s := NewStore(shared, t.TempDir())
	ch, _ := s.Get("local")
	ch.Name = "Renamed"
	if err := s.Save(ch); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(shared, "channels.json"))
	if !strings.Contains(string(raw), here) {
		t.Errorf("the legacy value was removed from the shared file:\n%s", raw)
	}
	if got := s.OutputDir("local"); got != here {
		t.Errorf("OutputDir = %q, want %q kept locally", got, here)
	}
}

func TestSave_ClearingThePathBeatsALegacyValue(t *testing.T) {
	shared, here := t.TempDir(), t.TempDir()
	writeSharedChannels(t, shared, `[{"slug":"local","name":"Local","outputPath":"`+here+`"}]`)
	s := NewStore(shared, t.TempDir())
	ch, _ := s.Get("local")
	ch.OutputPath = ""
	if err := s.Save(ch); err != nil {
		t.Fatal(err)
	}
	if got, want := s.OutputDir("local"), filepath.Join(s.outputBase, "channels", "local"); got != want {
		t.Errorf("OutputDir = %q, want the default %q", got, want)
	}
}

func TestDelete_RemovesTheLocalPath(t *testing.T) {
	s := NewStore(t.TempDir(), t.TempDir())
	_ = s.Save(&Channel{Slug: "site", Name: "Site", OutputPath: "/Users/someone/out"})
	if err := s.Delete("site"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(s.outputBase, "output-paths.json"))
	if strings.Contains(string(raw), "site") {
		t.Errorf("output-paths.json still names the deleted channel:\n%s", raw)
	}
}
