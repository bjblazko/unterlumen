package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const kodiNFO = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<movie>
    <title>Old</title>
    <runtime>3</runtime>
    <tag>beach</tag>
    <thumb aspect="poster">poster.jpg</thumb>
</movie>
`

func TestNFOKeepsWhatItDoesNotKnow(t *testing.T) {
	out, err := withNFO([]byte(kodiNFO), map[Field]string{
		Title: "Harbour", DateTaken: "2024-05-01T12:30:00+02:00", Rating: "4", Rights: "© Ann",
	}, []string{"sea"}, []string{"beach"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"<runtime>3</runtime>", `<thumb aspect="poster">poster.jpg</thumb>`,
		"<title>Harbour</title>", "<premiered>2024-05-01</premiered>", "<userrating>8</userrating>",
		"<tag>sea</tag>", `xmlns:ul="` + ulNFONS + `"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in\n%s", want, s)
		}
	}
	if strings.Contains(s, "beach") || strings.Count(s, "<title>") != 1 {
		t.Errorf("a removed tag or the old title stayed:\n%s", s)
	}
	fields, tags, _ := parseNFO(out)
	if fields[DateTaken] != "2024-05-01T12:30:00+02:00" || fields[Rating] != "4" || fields[Rights] != "© Ann" || len(tags) != 1 {
		t.Errorf("read back %v %v", fields, tags)
	}
}

func TestNFOIsMadeBesideTheVideoOnly(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "clip.mp4")
	if err := writeNFO(video, map[Field]string{Title: "x"}, nil, nil); err == nil {
		t.Error("an NFO was written without its video")
	}
	os.WriteFile(video, []byte("v"), 0o644) //nolint:errcheck
	if err := writeNFO(video, map[Field]string{Title: "Clip"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	fields, _, err := readNFO(video)
	if err != nil || fields[Title] != "Clip" {
		t.Errorf("read %v, %v", fields, err)
	}
}

func TestNFOOfAnotherKindIsLeftAlone(t *testing.T) {
	if _, err := withNFO([]byte("<tvshow><title>x</title></tvshow>"), map[Field]string{Title: "y"}, nil, nil); err == nil {
		t.Error("a tvshow NFO was changed")
	}
}
