package things

import "strings"

// Describe is SigLIP's vector of words typed into a search, in any
// language SigLIP 2 knows: they are taken as they are.
func (e *Engine) Describe(words string) ([]float32, error) {
	return e.describeText(strings.TrimSpace(words))
}

// VocabularyVectors is the text vector of every thing in Vocabulary, in its
// order: the average of its prompts' vectors, of unit length.
func (e *Engine) VocabularyVectors() ([][]float32, error) {
	out := make([][]float32, len(Vocabulary))
	for i, t := range Vocabulary {
		var sum []float32
		for _, p := range t.prompts() {
			v, err := e.describeText(p)
			if err != nil {
				return nil, err
			}
			if sum == nil {
				sum = make([]float32, len(v))
			}
			for j, x := range v {
				sum[j] += x
			}
		}
		out[i] = normalise(sum)
	}
	return out, nil
}
