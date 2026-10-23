package things

import (
	"image"
	"image/color"
	"math"
	"sort"

	"golang.org/x/image/draw"

	"huepattl.de/unterlumen/internal/onnx"
)

// YOLOX looks at a 640×640 RGB image of values 0–255, the photo scaled into
// its top left corner and the rest grey, as OpenCV's model zoo feeds it.
const (
	detectSize     = 640
	detectMinScore = 0.35 // the zoo's default
	detectOverlap  = 0.5  // boxes of one class overlapping more are one
)

var detectStrides = []int{8, 16, 32}

// Box is one animal or object in a photo: its COCO class, how sure the
// detector is, and where it is, in fractions of the photo's width and height
// with the origin top left.
type Box struct {
	Class      string
	Score      float32
	X, Y, W, H float64
}

func (b Box) iou(o Box) float64 {
	x1, y1 := max(b.X, o.X), max(b.Y, o.Y)
	x2, y2 := min(b.X+b.W, o.X+o.W), min(b.Y+b.H, o.Y+o.H)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	inter := (x2 - x1) * (y2 - y1)
	return inter / (b.W*b.H + o.W*o.H - inter)
}

func (e *Engine) detect(img image.Image) ([]Box, error) {
	in, scale := letterbox(img)
	out, err := e.detector.Run(in)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	return decodeBoxes(out[0].Data, float64(b.Dx())*scale, float64(b.Dy())*scale), nil
}

// letterbox scales img into the detector's square and returns it with the
// scale used.
func letterbox(img image.Image) (onnx.Tensor, float64) {
	b := img.Bounds()
	scale := min(float64(detectSize)/float64(b.Dx()), float64(detectSize)/float64(b.Dy()))
	sq := image.NewRGBA(image.Rect(0, 0, detectSize, detectSize))
	draw.Draw(sq, sq.Bounds(), image.NewUniform(color.RGBA{114, 114, 114, 255}), image.Point{}, draw.Src)
	fit := image.Rect(0, 0, int(float64(b.Dx())*scale), int(float64(b.Dy())*scale))
	draw.BiLinear.Scale(sq, fit, img, b, draw.Src, nil)
	n := detectSize * detectSize
	data := make([]float32, 3*n)
	for i := range n {
		for c := range 3 {
			data[c*n+i] = float32(sq.Pix[i*4+c])
		}
	}
	return onnx.Tensor{Shape: []int64{1, 3, detectSize, detectSize}, Data: data}, scale
}

// decodeBoxes reads YOLOX's rows — centre, size, objectness and one score
// per class, relative to its grid — into boxes relative to a photo that
// covers w×h of the square.
func decodeBoxes(rows []float32, w, h float64) []Box {
	width := 5 + len(cocoClasses)
	var found []Box
	row := 0
	for _, stride := range detectStrides {
		cells := detectSize / stride
		for gy := range cells {
			for gx := range cells {
				r := rows[row*width : (row+1)*width]
				row++
				class, score := bestClass(r[4], r[5:])
				if score < detectMinScore {
					continue
				}
				s := float64(stride)
				cx, cy := (float64(r[0])+float64(gx))*s, (float64(r[1])+float64(gy))*s
				bw, bh := math.Exp(float64(r[2]))*s, math.Exp(float64(r[3]))*s
				found = append(found, clipBox(Box{Class: cocoClasses[class], Score: score,
					X: (cx - bw/2) / w, Y: (cy - bh/2) / h, W: bw / w, H: bh / h}))
			}
		}
	}
	return suppressOverlaps(found)
}

func bestClass(objectness float32, scores []float32) (int, float32) {
	best := 0
	for i, s := range scores {
		if s > scores[best] {
			best = i
		}
	}
	return best, objectness * scores[best]
}

// clipBox keeps a box inside the photo.
func clipBox(b Box) Box {
	x2, y2 := min(b.X+b.W, 1), min(b.Y+b.H, 1)
	b.X, b.Y = max(b.X, 0), max(b.Y, 0)
	b.W, b.H = max(x2-b.X, 0), max(y2-b.Y, 0)
	return b
}

// suppressOverlaps keeps, of boxes of one class that overlap, the surest.
func suppressOverlaps(found []Box) []Box {
	sort.Slice(found, func(i, j int) bool { return found[i].Score > found[j].Score })
	var kept []Box
	for _, b := range found {
		dup := false
		for _, k := range kept {
			if k.Class == b.Class && k.iou(b) > detectOverlap {
				dup = true
				break
			}
		}
		if !dup {
			kept = append(kept, b)
		}
	}
	return kept
}
