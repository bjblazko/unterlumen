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
	out := image.NewRGBA(image.Rect(0, 0, cropSize, cropSize))
	draw.CatmullRom.Scale(out, out.Bounds(), src, faceSquare(src.Bounds(), box), draw.Src, nil)
	return out, nil
}

// faceSquare is the square of a face picture in the thumbnail b: the face
// with a margin around it, moved inside the photo where the face is near an
// edge — a little off centre rather than padded with black — and no larger
// than the photo's shorter side.
func faceSquare(b image.Rectangle, box lib.FaceBox) image.Rectangle {
	w, h := float64(b.Dx()), float64(b.Dy())
	side := math.Min(math.Max(box.W*w, box.H*h)*(1+2*cropMargin), math.Min(w, h))
	x := math.Max(0, math.Min((box.X+box.W/2)*w-side/2, w-side))
	y := math.Max(0, math.Min((box.Y+box.H/2)*h-side/2, h-side))
	return image.Rect(int(x), int(y), int(x+side), int(y+side)).Add(b.Min)
}
