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

// resemblanceOf is how many of a person's faces a face is compared with:
// the nearest few, so a profile is measured against their profiles rather
// than against an average no photo shows. Measured on a library of 12 554
// named faces of 143 people, the mean of the nearest three suggested the
// right person for 98.1 % of faces and a wrong one for 1.2 %, where the
// average of all their faces managed 92.3 % and 4.8 % (2026-10-23).
const resemblanceOf = 3

// Resemblance is how much e looks like a person, given the prints of their
// faces: the mean similarity of the nearest resemblanceOf of them; -1
// without any.
func Resemblance(e []float32, examples [][]float32) float32 {
	var top [resemblanceOf]float32
	n := 0
	for _, x := range examples {
		s := Similarity(e, x)
		if n < resemblanceOf {
			top[n] = s
			n++
		} else if low := lowest(top[:]); s > top[low] {
			top[low] = s
		}
	}
	if n == 0 {
		return -1
	}
	var sum float32
	for _, s := range top[:n] {
		sum += s
	}
	return sum / float32(n)
}

func lowest(v []float32) int {
	at := 0
	for i := range v {
		if v[i] < v[at] {
			at = i
		}
	}
	return at
}
