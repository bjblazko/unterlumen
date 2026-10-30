package media

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const iPhoneProbe = `Input #0, mov,mp4,m4a,3gp,3g2,mj2, from 'IMG_5287.MOV':
  Metadata:
    major_brand     : qt
    creation_time   : 2026-10-28T13:58:28.000000Z
    com.apple.quicktime.location.ISO6709: +50.9861+006.8993+049.701/
    com.apple.quicktime.make: Apple
    com.apple.quicktime.model: iPhone 16 Pro Max
    com.apple.quicktime.creationdate: 2026-10-28T14:58:27+0100
  Duration: 00:01:02.10, start: 0.000000, bitrate: 14209 kb/s
  Stream #0:0[0x1](und): Video: hevc (Main) (hvc1 / 0x31637668), yuvj420p(pc, smpte170m/smpte432/bt709), 1920x1440, 12892 kb/s, 29.45 fps, 29.92 tbr, 600 tbn (default)
    Metadata:
      creation_time   : 2026-10-28T13:58:28.000000Z
    Side data:
      Display Matrix: rotation of -90.00 degrees
  Stream #0:1[0x2](und): Video: hevc (Rext) (hvc1 / 0x31637668), gray(pc), 256x192, 6 kb/s
`

func TestParseVideoProbeReadsAnIPhoneClip(t *testing.T) {
	d := parseVideoProbe(iPhoneProbe)
	if d.Tags["Duration"] != "62.10" {
		t.Errorf("Duration = %q, want 62.10", d.Tags["Duration"])
	}
	if d.Tags["Make"] != "Apple" || d.Tags["Model"] != "iPhone 16 Pro Max" {
		t.Errorf("camera = %q %q", d.Tags["Make"], d.Tags["Model"])
	}
	if d.DateTaken == nil || *d.DateTaken != "2026-10-28T14:58:27+01:00" {
		t.Errorf("DateTaken = %v, want the local time with its offset", d.DateTaken)
	}
	if d.Latitude == nil || *d.Latitude != 50.9861 || *d.Longitude != 6.8993 {
		t.Errorf("location = %v %v", d.Latitude, d.Longitude)
	}
	if d.Width != 1440 || d.Height != 1920 {
		t.Errorf("size = %dx%d, want 1440x1920 (turned a quarter)", d.Width, d.Height)
	}
}

func TestParseVideoProbeWithoutMetadata(t *testing.T) {
	d := parseVideoProbe("Input #0, avi, from 'x.avi':\n  Duration: N/A, bitrate: N/A\n")
	if d.DateTaken != nil || d.Latitude != nil || d.Tags["Duration"] != "" {
		t.Errorf("an unknown video reads as %+v", d)
	}
}

func TestLiveCompanionsPairsClipsWithTheirPhotos(t *testing.T) {
	got := LiveCompanions([]string{"IMG_1.HEIC", "IMG_1.MOV", "IMG_2.MOV", "img_3.jpg", "IMG_3.mov", "IMG_4.mp4", "IMG_4.jpg"})
	want := map[string]string{"IMG_1.MOV": "IMG_1.HEIC", "IMG_3.mov": "img_3.jpg"}
	if len(got) != len(want) {
		t.Fatalf("pairs = %v, want %v", got, want)
	}
	for clip, photo := range want {
		if got[clip] != photo {
			t.Errorf("%s pairs with %q, want %q", clip, got[clip], photo)
		}
	}
}

func TestVideoTypes(t *testing.T) {
	if !IsVideo("a.MOV") || !IsSupportedMedia("a.mp4") || IsVideo("a.jpg") {
		t.Error("the video gate lets the wrong files through")
	}
	if VideoContentType("a.mov") != "video/quicktime" || VideoContentType("a.avi") != "" {
		t.Error("an unplayable video would be served inline, or a playable one not")
	}
}

func TestVideoPosterAndClip(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "clip.mp4")
	if out, err := exec.Command("ffmpeg", "-f", "lavfi", "-i", "testsrc=d=2:s=640x360:r=25",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", src).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot make a test video: %v %s", err, out)
	}
	poster, err := VideoPoster(src, 300)
	if err != nil || len(poster) < 2 || poster[0] != 0xFF || poster[1] != 0xD8 {
		t.Fatalf("poster is no JPEG: %v", err)
	}
	dst := filepath.Join(dir, "out.mp4")
	if err := VideoClip(src, dst); err != nil {
		t.Fatalf("clip: %v", err)
	}
	if fi, err := os.Stat(dst); err != nil || fi.Size() == 0 {
		t.Fatalf("clip not written: %v", err)
	}
}
