package media

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadTitle_NoSidecar(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "shot.jpg")
	title, err := ReadTitle(photo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if title != "" {
		t.Fatalf("expected empty title, got %q", title)
	}
}

func TestReadTitle_WithTitle(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "shot.jpg")
	xmp := `<?xml version="1.0" encoding="UTF-8"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/">
      <dc:title>
        <rdf:Alt>
          <rdf:li xml:lang="x-default">Sunset at the lake</rdf:li>
        </rdf:Alt>
      </dc:title>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>`
	if err := os.WriteFile(SidecarPath(photo), []byte(xmp), 0o644); err != nil {
		t.Fatal(err)
	}
	title, err := ReadTitle(photo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if title != "Sunset at the lake" {
		t.Fatalf("expected %q, got %q", "Sunset at the lake", title)
	}
}

func TestReadTitle_NoTitle(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "shot.jpg")
	pub := Publication{
		Channel:     "instagram",
		PublishedAt: time.Now(),
	}
	if err := AppendPublication(photo, pub); err != nil {
		t.Fatal(err)
	}
	title, err := ReadTitle(photo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if title != "" {
		t.Fatalf("expected empty title, got %q", title)
	}
}

func TestWriteTitle_FreshSidecar(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "shot.jpg")
	if err := WriteTitle(photo, "Golden hour"); err != nil {
		t.Fatalf("WriteTitle error: %v", err)
	}
	title, err := ReadTitle(photo)
	if err != nil {
		t.Fatalf("ReadTitle error: %v", err)
	}
	if title != "Golden hour" {
		t.Fatalf("expected %q, got %q", "Golden hour", title)
	}
}

func TestWriteTitle_PreservesULBlock(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "shot.jpg")
	pub := Publication{
		Channel:     "flickr",
		PostID:      "abc123",
		PublishedAt: time.Now(),
	}
	if err := AppendPublication(photo, pub); err != nil {
		t.Fatal(err)
	}
	if err := WriteTitle(photo, "Mountain view"); err != nil {
		t.Fatalf("WriteTitle error: %v", err)
	}
	title, err := ReadTitle(photo)
	if err != nil {
		t.Fatalf("ReadTitle error: %v", err)
	}
	if title != "Mountain view" {
		t.Fatalf("expected title %q, got %q", "Mountain view", title)
	}
	pubs, err := ReadSidecar(photo)
	if err != nil {
		t.Fatalf("ReadSidecar error: %v", err)
	}
	if len(pubs) != 1 || pubs[0].Channel != "flickr" || pubs[0].PostID != "abc123" {
		t.Fatalf("ul block not preserved: got %+v", pubs)
	}
}

func TestWriteTitle_UpdateTitle(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "shot.jpg")
	if err := WriteTitle(photo, "First title"); err != nil {
		t.Fatalf("first WriteTitle error: %v", err)
	}
	if err := WriteTitle(photo, "Second title"); err != nil {
		t.Fatalf("second WriteTitle error: %v", err)
	}
	title, err := ReadTitle(photo)
	if err != nil {
		t.Fatalf("ReadTitle error: %v", err)
	}
	if title != "Second title" {
		t.Fatalf("expected %q, got %q", "Second title", title)
	}
}

func TestWriteTitle_ClearTitle(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "shot.jpg")
	if err := WriteTitle(photo, "To be cleared"); err != nil {
		t.Fatalf("WriteTitle error: %v", err)
	}
	if err := WriteTitle(photo, ""); err != nil {
		t.Fatalf("clear WriteTitle error: %v", err)
	}
	title, err := ReadTitle(photo)
	if err != nil {
		t.Fatalf("ReadTitle error: %v", err)
	}
	if title != "" {
		t.Fatalf("expected empty title after clear, got %q", title)
	}
}
