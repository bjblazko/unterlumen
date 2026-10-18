package faces

// Similarity is the cosine of two unit-length embeddings: 1 for the same
// face, around 0 for strangers. SFace's makers put the line between the same
// person and another at 0.363.
func Similarity(a, b []float32) float32 {
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}
