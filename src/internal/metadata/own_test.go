package metadata

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestMachineTag(t *testing.T) {
	if got := MachineTag("stock", "Portra 400"); got != "ul:stock=Portra 400" {
		t.Error(got)
	}
	if got := MachineTag("film:stock", "Portra"); got != "film:stock=Portra" {
		t.Error(got)
	}
}

func TestOwnFieldIsWrittenAndMirrored(t *testing.T) {
	photo := filepath.Join(t.TempDir(), "a.jpg")
	os.WriteFile(photo, []byte("x"), 0o644) //nolint:errcheck
	read := func() Current {
		got, err := Read([]string{photo})
		if err != nil {
			t.Fatal(err)
		}
		return got[photo]
	}
	write := func(change Change) {
		if err := Write(photo, TargetSidecar, PlanFor(change, read())); err != nil {
			t.Fatal(err)
		}
	}
	write(Change{Own: map[string]OwnChange{"stock": {Value: ptr("Portra"), Mirror: ptr(true)}}})
	c := read()
	if c.Own["stock"] != "Portra" || !slices.Equal(c.Keywords, []string{"ul:stock=Portra"}) {
		t.Fatalf("own %v, keywords %v", c.Own, c.Keywords)
	}
	write(Change{Own: map[string]OwnChange{"stock": {Value: ptr("Ektar")}}})
	if c = read(); c.Own["stock"] != "Ektar" || !slices.Equal(c.Keywords, []string{"ul:stock=Ektar"}) {
		t.Fatalf("the keyword did not follow the value: own %v, keywords %v", c.Own, c.Keywords)
	}
	write(Change{Own: map[string]OwnChange{"stock": {Mirror: ptr(false)}}})
	if c = read(); c.Own["stock"] != "Ektar" || len(c.Keywords) != 0 {
		t.Fatalf("mirroring off: own %v, keywords %v", c.Own, c.Keywords)
	}
	write(Change{Own: map[string]OwnChange{"stock": {Value: ptr("")}}})
	if c = read(); len(c.Own) != 0 {
		t.Fatalf("the field stayed: %v", c.Own)
	}
}

func TestOwnFieldsLeaveAVideo(t *testing.T) {
	plan := PlanFor(Change{Own: map[string]OwnChange{"k": {Value: ptr("v")}}}, Current{Video: true})
	if len(plan.Own) != 0 {
		t.Errorf("an own field was planned for a video: %v", plan.Own)
	}
}
