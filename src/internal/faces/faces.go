// Package faces finds the faces in a photo and describes each by an
// embedding: 128 numbers that lie close together for the same person. It
// runs YuNet to find faces and SFace to describe them, both from OpenCV's
// model zoo, on ONNX Runtime (ADR-0051).
package faces

import (
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"io"
	"math"
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/onnx"
)

// Version names the models and the way they are used. A face found by an
// older version is found again, since its embedding cannot be compared.
const Version = 1

// The files a recognition folder holds.
const (
	DetectorFile   = "face_detection_yunet_2023mar.onnx"
	RecognizerFile = "face_recognition_sface_2021dec.onnx"
)

// Installed reports whether dir holds both models.
func Installed(dir string) bool {
	for _, f := range []string{DetectorFile, RecognizerFile} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return false
		}
	}
	return true
}

// Face is one face in a photo. Box is in fractions of the photo's width and
// height, its origin top left; Embedding has unit length.
type Face struct {
	X, Y, W, H float64
	Score      float32
	Embedding  []float32
}

// Engine finds and describes faces. Its methods may be called from several
// goroutines at once.
type Engine struct {
	detector, recognizer *onnx.Session
}

// Open loads ONNX Runtime from runtimeLib and the models from dir. threads
// limits the threads one model run uses.
func Open(runtimeLib, dir string, threads int) (*Engine, error) {
	rt, err := onnx.Load(runtimeLib)
	if err != nil {
		return nil, err
	}
	det, err := openModel(rt, filepath.Join(dir, DetectorFile), threads)
	if err != nil {
		return nil, err
	}
	rec, err := openModel(rt, filepath.Join(dir, RecognizerFile), threads)
	if err != nil {
		det.Close()
		return nil, err
	}
	return &Engine{detector: det, recognizer: rec}, nil
}

func openModel(rt *onnx.Runtime, path string, threads int) (*onnx.Session, error) {
	model, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s, err := rt.NewSession(model, threads)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return s, nil
}

// Close releases the models.
func (e *Engine) Close() {
	e.detector.Close()
	e.recognizer.Close()
}

// FindJPEG finds the faces in a JPEG, which must be upright already, as
// Unterlumen's thumbnails are.
func (e *Engine) FindJPEG(r io.Reader) ([]Face, error) {
	img, err := jpeg.Decode(r)
	if err != nil {
		return nil, err
	}
	return e.Find(img)
}

// Find finds the faces in an upright image.
func (e *Engine) Find(src image.Image) ([]Face, error) {
	img := toRGBA(src)
	found, err := detect(e.detector, img)
	if err != nil {
		return nil, err
	}
	w, h := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	faces := make([]Face, 0, len(found))
	for _, d := range found {
		emb, err := e.embed(alignFace(img, d.landmarks))
		if err != nil {
			return nil, err
		}
		b := clip(d.box, w, h)
		faces = append(faces, Face{X: b.x / w, Y: b.y / h, W: b.w / w, H: b.h / h, Score: d.score, Embedding: emb})
	}
	return faces, nil
}

// embed describes an aligned face; SFace takes RGB.
func (e *Engine) embed(face *image.RGBA) ([]float32, error) {
	out, err := e.recognizer.Run(planes(face, false))
	if err != nil {
		return nil, err
	}
	return normalise(out[0].Data), nil
}

func sqrt(v float64) float64 { return math.Sqrt(v) }

func normalise(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	n := float32(math.Sqrt(sum))
	if n == 0 {
		return v
	}
	for i := range v {
		v[i] /= n
	}
	return v
}

func clip(r rect, w, h float64) rect {
	x1, y1 := math.Max(0, r.x), math.Max(0, r.y)
	x2, y2 := math.Min(w, r.x+r.w), math.Min(h, r.y+r.h)
	return rect{x1, y1, math.Max(0, x2-x1), math.Max(0, y2-y1)}
}

func toRGBA(src image.Image) *image.RGBA {
	if img, ok := src.(*image.RGBA); ok && img.Bounds().Min == (image.Point{}) {
		return img
	}
	b := src.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	return img
}
