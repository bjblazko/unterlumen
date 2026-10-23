package things

import "strings"

// Describe is CLIP's vector of a phrase, for searching photos by words.
func (e *Engine) Describe(phrase string) ([]float32, error) {
	return e.describeText(searchPhrase(phrase))
}

// searchPhrase puts words typed into a search the way CLIP learned them,
// as the caption of a photo: "dog on a beach" is "a photo of dog on a beach".
func searchPhrase(words string) string {
	words = strings.TrimSpace(words)
	if strings.HasPrefix(strings.ToLower(words), "a photo") {
		return words
	}
	return "a photo of " + words
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
