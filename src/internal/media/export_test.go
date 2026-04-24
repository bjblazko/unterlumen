package media

import "testing"

func TestComputeTargetDims_None(t *testing.T) {
	w, h := computeTargetDims(3000, 2000, ScaleOptions{Mode: ScaleModeNone})
	if w != 3000 || h != 2000 {
		t.Errorf("ScaleModeNone changed dims: got %dx%d", w, h)
	}
}

func TestComputeTargetDims_Percent(t *testing.T) {
	w, h := computeTargetDims(3000, 2000, ScaleOptions{Mode: ScaleModePercent, Percent: 50})
	if w != 1500 || h != 1000 {
		t.Errorf("50%% scale: got %dx%d, want 1500x1000", w, h)
	}
}

func TestComputeTargetDims_PercentZero(t *testing.T) {
	// Zero percent should fall back to 100%
	w, h := computeTargetDims(3000, 2000, ScaleOptions{Mode: ScaleModePercent, Percent: 0})
	if w != 3000 || h != 2000 {
		t.Errorf("0%% should be treated as 100%%, got %dx%d", w, h)
	}
}

func TestComputeTargetDims_PixelsExact(t *testing.T) {
	w, h := computeTargetDims(3000, 2000, ScaleOptions{
		Mode: ScaleModePixels, Width: 1200, Height: 800,
	})
	if w != 1200 || h != 800 {
		t.Errorf("exact pixels: got %dx%d, want 1200x800", w, h)
	}
}

func TestComputeTargetDims_PixelsMaintainAR(t *testing.T) {
	// 3:2 image, fit in 900x900 box — should produce 900x600
	w, h := computeTargetDims(3000, 2000, ScaleOptions{
		Mode: ScaleModePixels, Width: 900, Height: 900, MaintainAR: true,
	})
	if w != 900 || h != 600 {
		t.Errorf("MaintainAR in 900x900: got %dx%d, want 900x600", w, h)
	}
}

func TestComputeTargetDims_MaxDimWidth(t *testing.T) {
	w, h := computeTargetDims(3000, 2000, ScaleOptions{
		Mode: ScaleModeMaxDim, MaxDimension: "width", MaxValue: 1500,
	})
	if w != 1500 || h != 1000 {
		t.Errorf("max_dim width=1500: got %dx%d, want 1500x1000", w, h)
	}
}

func TestComputeTargetDims_MaxDimHeight(t *testing.T) {
	w, h := computeTargetDims(3000, 2000, ScaleOptions{
		Mode: ScaleModeMaxDim, MaxDimension: "height", MaxValue: 1000,
	})
	if w != 1500 || h != 1000 {
		t.Errorf("max_dim height=1000: got %dx%d, want 1500x1000", w, h)
	}
}

func TestComputeTargetDims_MaxDimZero(t *testing.T) {
	w, h := computeTargetDims(3000, 2000, ScaleOptions{
		Mode: ScaleModeMaxDim, MaxValue: 0,
	})
	if w != 3000 || h != 2000 {
		t.Errorf("MaxValue=0 should be no-op, got %dx%d", w, h)
	}
}
