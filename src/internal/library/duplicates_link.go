package library

import (
	"sort"
	"strings"

	"huepattl.de/unterlumen/internal/appearance"
)

// How close two pictures' dHashes must be (ADR-0059). Up to sameLook bits
// they are the same picture, scaled or saved again, unless both say when
// they were taken and the times differ; up to editedLook bits and taken in
// the same second they are an edited or cropped copy. A burst, or a photo
// booth's shots in front of one wall, has its own seconds, so it stays apart.
const (
	sameLook   = 2
	editedLook = 10
	// aspectSlack is how far apart two shapes may be and still be one
	// picture scaled: rounding to whole pixels moves a ratio a little.
	aspectSlack = 0.01
	// manyAtOnce is the number of pictures taken in one second beyond which
	// that second is no moment but a date without its time.
	manyAtOnce = 20
	// hashBands splits a dHash into four bands of 16 bits: two hashes that
	// differ in sameLook bits or fewer agree in at least one of them.
	hashBands = 4
)

// picture is one content — one photo ID — with every path it is at.
type picture struct {
	files []*DupFile
	*DupFile
}

// groupDuplicates links the pictures that are versions of each other and
// returns the groups of two or more files.
func groupDuplicates(byPath map[string]*DupFile) *DupReport {
	pics, unanalysed := picturesOf(byPath)
	sets := newDisjoint(len(pics))
	linkSameLook(pics, sets)
	linkSameMoment(pics, sets)
	members := map[int][]*DupFile{}
	for i, p := range pics {
		root := sets.find(i)
		members[root] = append(members[root], p.files...)
	}
	report := &DupReport{Unanalysed: unanalysed, Groups: []DupGroup{}}
	for _, files := range members {
		if len(files) > 1 {
			report.Groups = append(report.Groups, DupGroup{Files: files})
		}
	}
	return report
}

// picturesOf gathers the files by content, in a fixed order, and counts the
// pictures whose look is not measured yet.
func picturesOf(byPath map[string]*DupFile) ([]picture, int) {
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	index := map[string]int{}
	var pics []picture
	unanalysed := 0
	for _, path := range paths {
		f := byPath[path]
		i, ok := index[f.PhotoID]
		if !ok {
			i = len(pics)
			index[f.PhotoID] = i
			pics = append(pics, picture{DupFile: f})
			if !f.hashed {
				unanalysed++
			}
		}
		pics[i].files = append(pics[i].files, f)
	}
	return pics, unanalysed
}

// linkSameLook links the pictures whose hashes are nearly equal and whose
// shapes agree, finding them through the bands of their hashes. A picture
// without a time is linked only to the closest one that has one: a scaled
// copy of one shot of a burst must not join the burst's shots together.
func linkSameLook(pics []picture, sets *disjoint) {
	closest := map[int]datedMatch{}
	for band := 0; band < hashBands; band++ {
		for _, bucket := range bandBuckets(pics, band) {
			eachPair(bucket, func(x, y int) {
				a, b := pics[x].DupFile, pics[y].DupFile
				d := appearance.HashDistance(a.hash, b.hash)
				if d > sameLook || !sameShape(a, b) || !sameMoment(a, b) {
					return
				}
				switch {
				case (a.taken == "") == (b.taken == ""):
					sets.union(x, y)
				case a.taken == "":
					closest[x] = closer(closest[x], datedMatch{y, d})
				default:
					closest[y] = closer(closest[y], datedMatch{x, d})
				}
			})
		}
	}
	for undated, m := range closest {
		sets.union(undated, m.to)
	}
}

// datedMatch is the closest dated picture found for an undated one so far.
type datedMatch struct{ to, dist int }

func closer(cur, next datedMatch) datedMatch {
	if cur == (datedMatch{}) || next.dist < cur.dist || next.dist == cur.dist && next.to < cur.to {
		return next
	}
	return cur
}

// bandBuckets groups the measured pictures by one 16-bit band of their hash.
func bandBuckets(pics []picture, band int) map[uint64][]int {
	buckets := map[uint64][]int{}
	for i, p := range pics {
		if p.hashed {
			key := p.hash >> (16 * band) & 0xffff
			buckets[key] = append(buckets[key], i)
		}
	}
	return buckets
}

// linkSameMoment links the pictures taken in the same second that look
// alike: an edit keeps the time of the photo it was made from.
func linkSameMoment(pics []picture, sets *disjoint) {
	byTime := map[string][]int{}
	for i, p := range pics {
		if p.hashed && p.taken != "" {
			byTime[p.taken] = append(byTime[p.taken], i)
		}
	}
	for _, moment := range byTime {
		if len(moment) > manyAtOnce {
			continue
		}
		eachPair(moment, func(x, y int) {
			if appearance.HashDistance(pics[x].hash, pics[y].hash) <= editedLook && sameFraction(pics[x].subsec, pics[y].subsec) {
				sets.union(x, y)
			}
		})
	}
}

// eachPair calls f with every two of the indexes.
func eachPair(idx []int, f func(x, y int)) {
	for i := 0; i < len(idx); i++ {
		for j := i + 1; j < len(idx); j++ {
			f(idx[i], idx[j])
		}
	}
}

// sameMoment holds unless both say when they were taken and the times differ.
func sameMoment(a, b *DupFile) bool {
	return a.taken == "" || b.taken == "" || a.taken == b.taken && sameFraction(a.subsec, b.subsec)
}

// sameFraction compares two fractions of a second to the digits both have:
// an export often keeps fewer ("64" of "640"), while the shots of a burst in
// one second differ ("400", "650"). Zero or nothing says nothing.
func sameFraction(a, b string) bool {
	if strings.Trim(a, "0 ") == "" || strings.Trim(b, "0 ") == "" {
		return true
	}
	n := min(len(a), len(b))
	return a[:n] == b[:n]
}

// sameShape holds when the long sides relate to the short ones alike, so a
// photo turned by its EXIF orientation still matches; unknown sizes match.
func sameShape(a, b *DupFile) bool {
	ra, rb := shapeOf(a), shapeOf(b)
	if ra == 0 || rb == 0 {
		return true
	}
	diff := ra - rb
	if diff < 0 {
		diff = -diff
	}
	return diff <= aspectSlack*ra
}

func shapeOf(f *DupFile) float64 {
	long, short := f.Width, f.Height
	if short > long {
		long, short = short, long
	}
	if short == 0 {
		return 0
	}
	return float64(long) / float64(short)
}

// relationTo says what f is to the group's largest version.
func relationTo(largest, f *DupFile) string {
	switch {
	case f.PhotoID == largest.PhotoID:
		return DupIdentical
	case !f.hashed || !largest.hashed ||
		appearance.HashDistance(f.hash, largest.hash) > sameLook || !sameShape(f, largest):
		return DupSimilar
	case f.Width*f.Height < largest.Width*largest.Height:
		return DupScaled
	}
	return DupResaved
}

// looser is the group kind that holds both a group of kind and a version
// in relation rel to the largest.
func looser(kind, rel string) string {
	switch {
	case kind == DupSimilar || rel == DupSimilar:
		return DupSimilar
	case kind == DupSame || rel == DupScaled || rel == DupResaved:
		return DupSame
	}
	return DupIdentical
}

// disjoint is a union-find over the pictures' indexes.
type disjoint struct{ parent []int }

func newDisjoint(n int) *disjoint {
	d := &disjoint{parent: make([]int, n)}
	for i := range d.parent {
		d.parent[i] = i
	}
	return d
}

func (d *disjoint) find(i int) int {
	for d.parent[i] != i {
		d.parent[i] = d.parent[d.parent[i]]
		i = d.parent[i]
	}
	return i
}

func (d *disjoint) union(a, b int) { d.parent[d.find(a)] = d.find(b) }
