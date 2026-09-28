package media

// Decoding a whole HEIF is the most memory-hungry thing Unterlumen does:
// heif-convert took about 640 MB for one 40-megapixel Fujifilm file. The
// thumbnail work limit counts cores (two on a Raspberry Pi 5), and two full
// decodes at once pushed an 8 GB NAS that also runs other services into swap
// for minutes. So full decodes run one at a time, whatever the number of cores.
var fullDecodeSlot = make(chan struct{}, 1)

func oneFullDecode(decode func() ([]byte, error)) ([]byte, error) {
	fullDecodeSlot <- struct{}{}
	defer func() { <-fullDecodeSlot }()
	return decode()
}

// ffmpegDecodeJPEG decodes a HEIF's HEVC image to a JPEG with ffmpeg. ffmpeg
// bakes no rotation into the pixels; the caller applies the orientation.
func ffmpegDecodeJPEG(path string) ([]byte, error) {
	return oneFullDecode(func() ([]byte, error) {
		return ffmpegRun(path,
			"-f", "image2pipe",
			"-vcodec", "mjpeg",
			"-q:v", "2",
			"-frames:v", "1",
			"pipe:1",
		)
	})
}

// fullJPEGPurpose names the disk cache entry of a HEIF's full-size JPEG.
const fullJPEGPurpose = "full-v5"

// HEIFConverted reports whether the full-size JPEG of a HEIF is already on
// disk, so serving it costs nothing. A prefetch asks this first: converting a
// photo that may never be looked at is the expensive part.
func HEIFConverted(path string) bool {
	return readCache(cacheKey(path, fullJPEGPurpose)) != nil
}
