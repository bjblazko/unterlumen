package things

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The tokenizer is part of the download, not the repository: this test runs
// where UNTERLUMEN_THINGS_DIR names a folder holding it.
func tokenizerForTest(t *testing.T) *tokenizer {
	dir := os.Getenv("UNTERLUMEN_THINGS_DIR")
	if dir == "" {
		t.Skip("UNTERLUMEN_THINGS_DIR is not set")
	}
	f, err := os.Open(filepath.Join(dir, TokenizerFile))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tok, err := newTokenizer(f)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// The IDs are those of Hugging Face's tokenizer for SigLIP 2, for the same
// text in lower case: English, German, Spanish, Japanese, double spaces and
// a character the vocabulary has.
func TestEncodeMatchesSigLIP2(t *testing.T) {
	tok := tokenizerForTest(t)
	for text, want := range map[string][]int64{
		"a photo of a dog":             {235250, 2686, 576, 476, 5929, 1},
		"Hund am Strand":               {91738, 1144, 40672, 1},
		"perro en la playa":            {142038, 659, 683, 33729, 1},
		"浜辺の犬":                         {238022, 237932, 235372, 237689, 1},
		"A Helicopter over the Rhine!": {235250, 44364, 1163, 573, 46625, 622, 235341, 1},
		"naïve café  two  spaces":      {556, 236370, 524, 18688, 139, 9721, 139, 46633, 1},
		"emoji 🚁 test":                 {62400, 235248, 248988, 2121, 1},
	} {
		got := tok.encode(text)
		if len(got) != contextLength {
			t.Fatalf("%q: %d IDs, not %d", text, len(got), contextLength)
		}
		if !slices.Equal(got[:len(want)], want) || got[len(want)] != 0 {
			t.Errorf("%q: got %v, want %v", text, got[:len(want)+1], want)
		}
	}
}

func TestEncodeCutsLongText(t *testing.T) {
	tok := tokenizerForTest(t)
	long := ""
	for range 100 {
		long += "dog "
	}
	got := tok.encode(long)
	if len(got) != contextLength || got[contextLength-1] != tok.end {
		t.Fatalf("got %d IDs ending in %d", len(got), got[len(got)-1])
	}
}
