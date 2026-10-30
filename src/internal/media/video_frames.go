package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// A video is shown in the grids as a still frame and, under the pointer, as
// a short silent clip. ffmpeg makes both; it turns the frames upright on its
// own, so nothing here reads an orientation.

const (
	clipSeconds = 3
	clipWidth   = 320
)

// VideoPoster is a still frame of the video, at most maxDim on its longer
// side, as JPEG: one second in, or the first frame of a shorter clip.
func VideoPoster(path string, maxDim int) ([]byte, error) {
	data, err := videoFrame(path, "1", maxDim)
	if err != nil || len(data) == 0 {
		return videoFrame(path, "0", maxDim)
	}
	return data, nil
}

func videoFrame(path, at string, maxDim int) ([]byte, error) {
	scale := fmt.Sprintf("scale='min(iw,%d)':'min(ih,%d)':force_original_aspect_ratio=decrease", maxDim, maxDim)
	return ffmpegExec("-ss", at, "-i", path, "-frames:v", "1", "-vf", scale,
		"-f", "image2", "-c:v", "mjpeg", "-q:v", "3", "pipe:1")
}

// VideoPosterCached is VideoPoster through the cache and the shared limit on
// thumbnail work.
func VideoPosterCached(ctx context.Context, path string, maxDim int) ([]byte, error) {
	key := cacheKey(path, fmt.Sprintf("thumb-video-v1-%d", maxDim))
	if cached := readCache(key); cached != nil {
		return cached, nil
	}
	result := thumbnailWork.run(ctx, key, func() thumbnailWorkResult {
		if cached := readCache(key); cached != nil {
			return thumbnailWorkResult{data: cached}
		}
		data, err := VideoPoster(path, maxDim)
		if err != nil {
			return thumbnailWorkResult{err: err}
		}
		writeCache(key, data)
		return thumbnailWorkResult{data: data}
	})
	return result.data, result.err
}

// VideoClip writes a three-second silent H.264 clip of the video, 320 pixels
// wide, to dst: from a tenth of the way in, so it does not start on a black
// frame. The user's own ffmpeg encodes it; Unterlumen carries no codec.
func VideoClip(path, dst string) error {
	start := "0"
	if meta, err := ExtractVideoMeta(path); err == nil {
		if d, err := strconv.ParseFloat(meta.Tags["Duration"], 64); err == nil && d > 2*clipSeconds {
			start = strconv.FormatFloat(d/10, 'f', 2, 64)
		}
	}
	tmp := dst + ".part.mp4"
	defer os.Remove(tmp)
	_, err := ffmpegExec("-y", "-ss", start, "-i", path, "-t", strconv.Itoa(clipSeconds), "-an",
		"-vf", fmt.Sprintf("scale='min(iw,%d)':-2,fps=15", clipWidth),
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "30", "-pix_fmt", "yuv420p",
		"-movflags", "+faststart", tmp)
	if err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// VideoClipCached is the clip of a video in the folders, made once and kept
// in the cache beside its thumbnails. It returns the clip's file.
func VideoClipCached(ctx context.Context, path string) (string, error) {
	key := strings.TrimSuffix(cacheKey(path, "thumb-clip-v1"), ".jpg") + ".mp4"
	file := filepath.Join(getCacheDir(), key)
	if _, err := os.Stat(file); err == nil {
		return file, nil
	}
	result := thumbnailWork.run(ctx, key, func() thumbnailWorkResult {
		if _, err := os.Stat(file); err == nil {
			return thumbnailWorkResult{}
		}
		if err := VideoClip(path, file); err != nil {
			return thumbnailWorkResult{err: err}
		}
		noteCacheWrite()
		return thumbnailWorkResult{}
	})
	return file, result.err
}
