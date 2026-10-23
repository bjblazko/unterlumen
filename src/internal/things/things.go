// Package things tells what a photo shows — animals, objects, scenes — and
// describes it by a vector that a phrase can be compared with. It runs
// OpenAI's CLIP (an image and a text encoder) and YOLOX from OpenCV's model
// zoo, which puts a box around each animal and object it knows, on ONNX
// Runtime (ADR-0054).
package things

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/onnx"
)

// Version names the models and the way they are used. A photo looked at by
// an older version is looked at again, since its vector cannot be compared.
const Version = 1

// The files a things folder holds.
const (
	ImageFile    = "clip_vit_b16_vision.onnx"
	TextFile     = "clip_vit_b16_text_quantized.onnx"
	VocabFile    = "clip_vocab.json"
	MergesFile   = "clip_merges.txt"
	DetectorFile = "object_detection_yolox_2022nov.onnx"
)

// Files are every file a things folder holds.
var Files = []string{ImageFile, TextFile, VocabFile, MergesFile, DetectorFile}

// Installed reports whether dir holds every model.
func Installed(dir string) bool {
	for _, f := range Files {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return false
		}
	}
	return true
}

// Seen is what one photo shows: its vector, of unit length, and the boxes
// the detector found.
type Seen struct {
	Vector []float32
	Boxes  []Box
}

// Engine looks at photos and phrases. Its methods may be called from
// several goroutines at once.
type Engine struct {
	image, text, detector *onnx.Session
	tok                   *tokenizer
}

// Open loads ONNX Runtime from runtimeLib and the models from dir. threads
// limits the threads one model run uses.
func Open(runtimeLib, dir string, threads int) (*Engine, error) {
	rt, err := onnx.Load(runtimeLib)
	if err != nil {
		return nil, err
	}
	tok, err := openTokenizer(dir)
	if err != nil {
		return nil, err
	}
	e := &Engine{tok: tok}
	for _, m := range []struct {
		file string
		s    **onnx.Session
	}{{ImageFile, &e.image}, {TextFile, &e.text}, {DetectorFile, &e.detector}} {
		if *m.s, err = openModel(rt, filepath.Join(dir, m.file), threads); err != nil {
			e.Close()
			return nil, err
		}
	}
	return e, nil
}

func openTokenizer(dir string) (*tokenizer, error) {
	vocab, err := os.Open(filepath.Join(dir, VocabFile))
	if err != nil {
		return nil, err
	}
	defer vocab.Close()
	merges, err := os.Open(filepath.Join(dir, MergesFile))
	if err != nil {
		return nil, err
	}
	defer merges.Close()
	return newTokenizer(vocab, merges)
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
	for _, s := range []*onnx.Session{e.image, e.text, e.detector} {
		if s != nil {
			s.Close()
		}
	}
}

// LookJPEG looks at a JPEG, which must be upright already, as Unterlumen's
// thumbnails are.
func (e *Engine) LookJPEG(r io.Reader) (Seen, error) {
	img, err := jpeg.Decode(r)
	if err != nil {
		return Seen{}, err
	}
	return e.Look(img)
}

// Look describes an upright image and finds the boxes in it.
func (e *Engine) Look(img image.Image) (Seen, error) {
	vec, err := e.describeImage(img)
	if err != nil {
		return Seen{}, err
	}
	boxes, err := e.detect(img)
	if err != nil {
		return Seen{}, err
	}
	return Seen{Vector: vec, Boxes: boxes}, nil
}
