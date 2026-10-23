package things

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The vocabulary is part of the download, not the repository: this test
// runs where UNTERLUMEN_THINGS_DIR names a folder holding it.
func tokenizerForTest(t *testing.T) *tokenizer {
	dir := os.Getenv("UNTERLUMEN_THINGS_DIR")
	if dir == "" {
		t.Skip("UNTERLUMEN_THINGS_DIR is not set")
	}
	vocab, err := os.Open(filepath.Join(dir, VocabFile))
	if err != nil {
		t.Fatal(err)
	}
	defer vocab.Close()
	merges, err := os.Open(filepath.Join(dir, MergesFile))
	if err != nil {
		t.Fatal(err)
	}
	defer merges.Close()
	tok, err := newTokenizer(vocab, merges)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// The IDs are those of OpenAI's CLIP tokenizer for the same text.
func TestEncodeMatchesCLIP(t *testing.T) {
	tok := tokenizerForTest(t)
	for text, want := range map[string][]int64{
		"a photo of a cat":   {49406, 320, 1125, 539, 320, 2368, 49407},
		"A  Photo of a DOG.": {49406, 320, 1125, 539, 320, 1929, 269, 49407},
	} {
		got := tok.encode(text)
		if len(got) != contextLength {
			t.Fatalf("%q: %d IDs, not %d", text, len(got), contextLength)
		}
		if !slices.Equal(got[:len(want)], want) || got[len(want)] != 49407 {
			t.Errorf("%q: got %v, want %v", text, got[:len(want)+1], want)
		}
	}
}

func TestEncodeCutsLongText(t *testing.T) {
	tok := tokenizerForTest(t)
	long := ""
	for range 100 {
		long += "cat "
	}
	got := tok.encode(long)
	if len(got) != contextLength || got[contextLength-1] != tok.end {
		t.Fatalf("got %d IDs ending in %d", len(got), got[len(got)-1])
	}
}
