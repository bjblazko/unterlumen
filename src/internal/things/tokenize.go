package things

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// SigLIP 2 reads contextLength tokens: the text's, an end mark, padding.
// Longer text is cut.
const contextLength = 64

// tokenizer is Gemma's byte-level BPE, as SigLIP 2 uses it: text in lower
// case, spaces as "▁", no split into words, the best-ranked pair of
// neighbours joined until none is left; a character the vocabulary lacks
// falls back to its bytes, <0xXX>. Symbols are kept as IDs, so the half a
// million merges take little memory.
type tokenizer struct {
	ids     map[string]int32
	merges  map[[2]int32]int32 // pair → its rank
	joined  map[[2]int32]int32 // pair → the token it becomes
	bytes   [256]int32         // <0xXX>
	end     int64
	padding int64
}

// tokenizerFile is the part of a Hugging Face tokenizer.json that is read.
type tokenizerFile struct {
	Model struct {
		Vocab  map[string]int32 `json:"vocab"`
		Merges [][2]string      `json:"merges"`
	} `json:"model"`
	// Added are tokens kept apart from the vocabulary — the special marks,
	// and some of the byte tokens.
	Added []struct {
		ID      int32  `json:"id"`
		Content string `json:"content"`
	} `json:"added_tokens"`
}

func newTokenizer(r io.Reader) (*tokenizer, error) {
	var f tokenizerFile
	if err := json.NewDecoder(r).Decode(&f); err != nil {
		return nil, fmt.Errorf("tokenizer.json: %w", err)
	}
	t := &tokenizer{ids: f.Model.Vocab, merges: make(map[[2]int32]int32, len(f.Model.Merges)),
		joined: make(map[[2]int32]int32, len(f.Model.Merges))}
	for _, a := range f.Added {
		if _, ok := t.ids[a.Content]; !ok {
			t.ids[a.Content] = a.ID
		}
	}
	for rank, m := range f.Model.Merges {
		a, okA := t.ids[m[0]]
		b, okB := t.ids[m[1]]
		ab, okAB := t.ids[m[0]+m[1]]
		if !okA || !okB || !okAB {
			continue
		}
		pair := [2]int32{a, b}
		if _, seen := t.merges[pair]; !seen {
			t.merges[pair], t.joined[pair] = int32(rank), ab
		}
	}
	unknown := t.ids["<unk>"]
	for b := range 256 {
		// A byte has its token <0xXX>, unless it is a character of its own
		// (the tab); one without either is unknown.
		id, ok := t.ids[fmt.Sprintf("<0x%02X>", b)]
		if !ok {
			id, ok = t.ids[string(rune(b))]
		}
		if !ok {
			id = unknown
		}
		t.bytes[b] = id
	}
	end, okEnd := t.ids["<eos>"]
	pad, okPad := t.ids["<pad>"]
	if !okEnd || !okPad {
		return nil, fmt.Errorf("tokenizer.json has no <eos> or <pad>")
	}
	t.end, t.padding = int64(end), int64(pad)
	return t, nil
}

// encode turns text into contextLength token IDs, as SigLIP 2 was trained:
// the text's tokens, the end mark, padding.
func (t *tokenizer) encode(text string) []int64 {
	text = strings.ReplaceAll(strings.ToLower(text), " ", "▁")
	parts := t.bpe(t.symbols(text))
	ids := make([]int64, 0, contextLength)
	for _, p := range parts[:min(len(parts), contextLength-1)] {
		ids = append(ids, int64(p))
	}
	ids = append(ids, t.end)
	for len(ids) < contextLength {
		ids = append(ids, t.padding)
	}
	return ids
}

// symbols is each character's token, or its bytes' when the vocabulary
// has no token for it.
func (t *tokenizer) symbols(text string) []int32 {
	out := make([]int32, 0, len(text))
	for _, r := range text {
		if id, ok := t.ids[string(r)]; ok {
			out = append(out, id)
			continue
		}
		var buf [utf8.UTFMax]byte
		for _, b := range buf[:utf8.EncodeRune(buf[:], r)] {
			out = append(out, t.bytes[b])
		}
	}
	return out
}

// bpe joins the best-ranked neighbouring pair until no pair has a rank.
func (t *tokenizer) bpe(parts []int32) []int32 {
	for len(parts) > 1 {
		best, at := int32(-1), -1
		for i := 0; i+1 < len(parts); i++ {
			if r, ok := t.merges[[2]int32{parts[i], parts[i+1]}]; ok && (best < 0 || r < best) {
				best, at = r, i
			}
		}
		if at < 0 {
			break
		}
		pair := [2]int32{parts[at], parts[at+1]}
		joined := t.joined[pair]
		out := parts[:0:0]
		for i := 0; i < len(parts); i++ {
			if i+1 < len(parts) && parts[i] == pair[0] && parts[i+1] == pair[1] {
				out = append(out, joined)
				i++
				continue
			}
			out = append(out, parts[i])
		}
		parts = out
	}
	return parts
}
