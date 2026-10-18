package apipeople

import (
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"

	lib "huepattl.de/unterlumen/internal/library"
)

const (
	cropSize   = 192  // pixels of a face picture's side; shown at up to 96 CSS px
	cropMargin = 0.35 // of the face's size, added on every side
)

// serveFace serves a square picture of one face, cut from its photo's
// thumbnail. A face found again gets a new box, and with it a new ETag.
func serveFace(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		fid, ok2 := pathInt(w, r, "fid")
		if !ok || !ok2 {
			return
		}
		box, err := store.FaceBox(fid)
		if err != nil {
			http.Error(w, "The face was not found.", http.StatusNotFound)
			return
		}
		etag := fmt.Sprintf(`"%s-%.5f-%.5f-%.5f"`, filepath.Base(box.ThumbPath), box.X, box.Y, box.W)
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-cache")
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		img, err := cropFace(filepath.Join(mgr.LibDir(r.PathValue("id")), box.ThumbPath), box)
		if err != nil {
			http.Error(w, "The face's photo has no thumbnail.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		jpeg.Encode(w, img, &jpeg.Options{Quality: 85}) //nolint:errcheck
	}
}

func cropFace(thumbPath string, box lib.FaceBox) (image.Image, error) {
	f, err := os.Open(thumbPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := jpeg.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	side := math.Max(box.W*w, box.H*h) * (1 + 2*cropMargin)
	cx, cy := (box.X+box.W/2)*w, (box.Y+box.H/2)*h
	r := image.Rect(int(cx-side/2), int(cy-side/2), int(cx+side/2), int(cy+side/2)).Add(b.Min).Intersect(b)
	out := image.NewRGBA(image.Rect(0, 0, cropSize, cropSize))
	draw.CatmullRom.Scale(out, squareIn(out.Bounds(), r), src, r, draw.Src, nil)
	return out, nil
}

// squareIn is where r, cut short at a photo's edge, goes in the square dst
// so the face keeps its proportions.
func squareIn(dst, r image.Rectangle) image.Rectangle {
	if r.Dx() == r.Dy() || r.Dx() == 0 || r.Dy() == 0 {
		return dst
	}
	if r.Dx() > r.Dy() {
		h := dst.Dx() * r.Dy() / r.Dx()
		return image.Rect(0, (dst.Dy()-h)/2, dst.Dx(), (dst.Dy()+h)/2)
	}
	w := dst.Dy() * r.Dx() / r.Dy()
	return image.Rect((dst.Dx()-w)/2, 0, (dst.Dx()+w)/2, dst.Dy())
}
