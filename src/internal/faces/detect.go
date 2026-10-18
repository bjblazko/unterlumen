package faces

import (
	"image"
	"math"
	"sort"

	"golang.org/x/image/draw"

	"huepattl.de/unterlumen/internal/onnx"
)

// YuNet takes a fixed 640×640 input; a photo is scaled to fit and the rest
// left black. It finds faces of about 10 to 300 pixels at that size.
const (
	detectSize     = 640
	scoreThreshold = 0.9  // OpenCV's default; lower finds more and invents more
	nmsThreshold   = 0.3  // overlap above which the weaker of two boxes goes
	minFaceSide    = 0.03 // of the photo's longer side: smaller faces are too blurred to recognise
)

var detectStrides = [3]int{8, 16, 32}

// detection is one face YuNet found, in the photo's pixels.
type detection struct {
	box       rect
	landmarks [5]point // right eye, left eye, nose tip, right and left mouth corner
	score     float32
}

type point struct{ x, y float64 }

type rect struct{ x, y, w, h float64 }

func (r rect) area() float64 { return r.w * r.h }

func (r rect) iou(o rect) float64 {
	x1, y1 := math.Max(r.x, o.x), math.Max(r.y, o.y)
	x2, y2 := math.Min(r.x+r.w, o.x+o.w), math.Min(r.y+r.h, o.y+o.h)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	inter := (x2 - x1) * (y2 - y1)
	return inter / (r.area() + o.area() - inter)
}

// detect finds the faces in img.
func detect(s *onnx.Session, img *image.RGBA) ([]detection, error) {
	b := img.Bounds()
	scale := float64(detectSize) / float64(max(b.Dx(), b.Dy()))
	out, err := s.Run(detectorInput(img, scale))
	if err != nil {
		return nil, err
	}
	found := decodeDetections(out)
	minSide := minFaceSide * detectSize
	kept := found[:0]
	for _, d := range suppressOverlaps(found) {
		if math.Min(d.box.w, d.box.h) >= minSide {
			kept = append(kept, d.scaled(1/scale))
		}
	}
	return kept, nil
}

// detectorInput scales img by scale into the top left of a black square and
// lays it out as YuNet wants it: BGR planes of 0–255.
func detectorInput(img *image.RGBA, scale float64) onnx.Tensor {
	b := img.Bounds()
	square := image.NewRGBA(image.Rect(0, 0, detectSize, detectSize))
	w := min(detectSize, int(math.Round(float64(b.Dx())*scale)))
	h := min(detectSize, int(math.Round(float64(b.Dy())*scale)))
	draw.BiLinear.Scale(square, image.Rect(0, 0, w, h), img, b, draw.Src, nil)
	return planes(square, true)
}

// planes lays out an RGBA image as a [1,3,h,w] tensor of 0–255, in BGR or RGB order.
func planes(img *image.RGBA, bgr bool) onnx.Tensor {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	n := w * h
	data := make([]float32, 3*n)
	first, third := 0, 2
	if bgr {
		first, third = 2, 0
	}
	for i := range n {
		px := img.Pix[i*4 : i*4+3]
		data[i] = float32(px[first])
		data[n+i] = float32(px[1])
		data[2*n+i] = float32(px[third])
	}
	return onnx.Tensor{Shape: []int64{1, 3, int64(h), int64(w)}, Data: data}
}

// decodeDetections reads YuNet's outputs — cls, obj, bbox, kps for each of
// the three strides, in that order — as OpenCV's FaceDetectorYN does.
func decodeDetections(out []onnx.Tensor) []detection {
	var found []detection
	for si, stride := range detectStrides {
		cls, obj := out[si].Data, out[si+3].Data
		bbox, kps := out[si+6].Data, out[si+9].Data
		cols := detectSize / stride
		for idx := range cls {
			score := float32(math.Sqrt(float64(clamp01(cls[idx]) * clamp01(obj[idx]))))
			if score < scoreThreshold {
				continue
			}
			found = append(found, decodeOne(idx%cols, idx/cols, stride, bbox[idx*4:idx*4+4], kps[idx*10:idx*10+10], score))
		}
	}
	return found
}

func decodeOne(c, r, stride int, bbox, kps []float32, score float32) detection {
	s := float64(stride)
	cx := (float64(c) + float64(bbox[0])) * s
	cy := (float64(r) + float64(bbox[1])) * s
	w := math.Exp(float64(bbox[2])) * s
	h := math.Exp(float64(bbox[3])) * s
	d := detection{box: rect{cx - w/2, cy - h/2, w, h}, score: score}
	for n := range d.landmarks {
		d.landmarks[n] = point{(float64(kps[2*n]) + float64(c)) * s, (float64(kps[2*n+1]) + float64(r)) * s}
	}
	return d
}

func clamp01(v float32) float32 { return min(max(v, 0), 1) }

// suppressOverlaps keeps, of boxes overlapping by more than nmsThreshold,
// only the most certain.
func suppressOverlaps(found []detection) []detection {
	sort.Slice(found, func(i, j int) bool { return found[i].score > found[j].score })
	var kept []detection
	for _, d := range found {
		overlaps := false
		for _, k := range kept {
			if d.box.iou(k.box) > nmsThreshold {
				overlaps = true
				break
			}
		}
		if !overlaps {
			kept = append(kept, d)
		}
	}
	return kept
}

func (d detection) scaled(f float64) detection {
	d.box = rect{d.box.x * f, d.box.y * f, d.box.w * f, d.box.h * f}
	for i, p := range d.landmarks {
		d.landmarks[i] = point{p.x * f, p.y * f}
	}
	return d
}
