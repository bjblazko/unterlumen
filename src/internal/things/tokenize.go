package things

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// CLIP reads at most contextLength tokens, its start and end marks included.
const (
	contextLength = 77
	startToken    = "<|startoftext|>"
	endToken      = "<|endoftext|>"
)

// wordPattern splits text into words the way CLIP's tokenizer does.
var wordPattern = regexp.MustCompile(`'s|'t|'re|'ve|'m|'ll|'d|\p{L}+|\p{N}|[^\s\p{L}\p{N}]+`)

// tokenizer is CLIP's byte-level BPE: vocab.json maps tokens to IDs,
// merges.txt ranks the pairs that join, best first.
type tokenizer struct {
	ids        map[string]int64
	ranks      map[[2]string]int
	byteSymbol [256]string
	start, end int64
}

func newTokenizer(vocab, merges io.Reader) (*tokenizer, error) {
	t := &tokenizer{ranks: map[[2]string]int{}, byteSymbol: byteSymbols()}
	if err := json.NewDecoder(vocab).Decode(&t.ids); err != nil {
		return nil, fmt.Errorf("vocab.json: %w", err)
	}
	sc := bufio.NewScanner(merges)
	for line := 0; sc.Scan(); line++ {
		a, b, ok := strings.Cut(sc.Text(), " ")
		if line == 0 && strings.HasPrefix(a, "#") || !ok {
			continue
		}
		t.ranks[[2]string{a, b}] = len(t.ranks)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("merges.txt: %w", err)
	}
	var ok1, ok2 bool
	t.start, ok1 = t.ids[startToken]
	t.end, ok2 = t.ids[endToken]
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("vocab.json has no start or end token")
	}
	return t, nil
}

// encode turns text into contextLength token IDs: start, the words, end,
// and end again as padding (CLIP pools at the first end). Text that is
// too long is cut.
func (t *tokenizer) encode(text string) []int64 {
	ids := []int64{t.start}
	for _, w := range wordPattern.FindAllString(strings.ToLower(strings.Join(strings.Fields(text), " ")), -1) {
		for _, tok := range t.bpe(t.symbols(w)) {
			if id, ok := t.ids[tok]; ok {
				ids = append(ids, id)
			}
		}
	}
	ids = append(ids[:min(len(ids), contextLength-1)], t.end)
	for len(ids) < contextLength {
		ids = append(ids, t.end)
	}
	return ids
}

// symbols maps a word's bytes to BPE's printable symbols, the last one
// marked as the end of the word.
func (t *tokenizer) symbols(word string) []string {
	out := make([]string, len(word))
	for i := range len(word) {
		out[i] = t.byteSymbol[word[i]]
	}
	out[len(out)-1] += "</w>"
	return out
}

// bpe joins the best-ranked neighbouring pair until no pair has a rank.
func (t *tokenizer) bpe(parts []string) []string {
	for len(parts) > 1 {
		best, at := -1, -1
		for i := 0; i+1 < len(parts); i++ {
			if r, ok := t.ranks[[2]string{parts[i], parts[i+1]}]; ok && (best < 0 || r < best) {
				best, at = r, i
			}
		}
		if at < 0 {
			break
		}
		pair := [2]string{parts[at], parts[at+1]}
		joined := parts[:0:0]
		for i := 0; i < len(parts); i++ {
			if i+1 < len(parts) && parts[i] == pair[0] && parts[i+1] == pair[1] {
				joined = append(joined, pair[0]+pair[1])
				i++
				continue
			}
			joined = append(joined, parts[i])
		}
		parts = joined
	}
	return parts
}

// byteSymbols is GPT-2's map of bytes to printable characters: printable
// Latin-1 stands for itself, every other byte for a character from 256 up.
func byteSymbols() [256]string {
	var m [256]string
	next := 256
	for b := range 256 {
		if b >= '!' && b <= '~' || b >= 0xA1 && b <= 0xAC || b >= 0xAE {
			m[b] = string(rune(b))
			continue
		}
		m[b] = string(rune(next))
		next++
	}
	return m
}
