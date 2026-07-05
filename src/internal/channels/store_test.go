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
