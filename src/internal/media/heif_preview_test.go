package media

import "testing"

func TestChooseHEIFJPEGPreviewStreamPrefersSmallestAdequatePreview(t *testing.T) {
	streams := []heifJPEGPreviewStream{
		{Index: 4, Width: 1920, Height: 1280},
		{Index: 5, Width: 640, Height: 480},
		{Index: 6, Width: 160, Height: 120},
	}

	tests := []struct {
		name    string
		maxDim  int
		wantIdx int
	}{
		{name: "largest when size unspecified", maxDim: 0, wantIdx: 4},
		{name: "small thumbnail uses 160 preview", maxDim: 80, wantIdx: 6},
		{name: "medium thumbnail uses 640 preview", maxDim: 300, wantIdx: 5},
		{name: "large thumbnail uses 1920 preview", maxDim: 1000, wantIdx: 4},
		{name: "too-large request falls back to largest preview", maxDim: 3000, wantIdx: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, ok := chooseHEIFJPEGPreviewStream(streams, tt.maxDim)
			if !ok {
				t.Fatalf("chooseHEIFJPEGPreviewStream returned no stream")
			}
			if stream.Index != tt.wantIdx {
				t.Fatalf("chooseHEIFJPEGPreviewStream(...).Index = %d, want %d", stream.Index, tt.wantIdx)
			}
		})
	}
}

func TestParseHEIFJPEGStreams(t *testing.T) {
	probe := `
Input #0, mov,mp4,m4a,3gp,3g2,mj2, from 'sample.HIF':
  Stream #0:0: Video: hevc (Rext), 7728x5152
  Stream #0:4: Video: mjpeg (Baseline), yuvj422p(pc), 1920x1280, 1 fps
  Stream #0:5: Video: mjpeg (Baseline), yuvj422p(pc), 640x480, 1 fps
  Stream #0:6: Video: mjpeg (Baseline), yuvj422p(pc), 160x120, 1 fps
`

	streams := parseHEIFJPEGStreams(probe)
	if len(streams) != 3 {
		t.Fatalf("parseHEIFJPEGStreams() len = %d, want 3", len(streams))
	}
	if streams[0].Index != 4 || streams[0].Width != 1920 || streams[0].Height != 1280 {
		t.Fatalf("first stream = %+v, want index 4 1920x1280", streams[0])
	}
	if streams[2].Index != 6 || streams[2].Width != 160 || streams[2].Height != 120 {
		t.Fatalf("third stream = %+v, want index 6 160x120", streams[2])
	}
}
