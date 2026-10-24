package toolinstall

import (
	"context"

	"huepattl.de/unterlumen/internal/things"
)

// siglipRevision pins the ONNX export of Google's SigLIP 2 base (patch 16,
// 224 px) on Hugging Face to one commit (ADR-0055). The image encoder is
// taken at full precision; the text encoder at 8 bits, as CLIP's agreed
// with its full one. The tokenizer is Hugging Face's tokenizer.json, read
// as it is.
const siglipRevision = "ba1f3b0843f24bc5417d38e19c37b287d719b2f4"

var thingsModels = []model{
	{name: things.ImageFile, download: download{
		url:    siglipFile("onnx/vision_model.onnx"),
		sha256: "c0573e3f4140c3a7c4e9cc5912bd6b26a033b46a6a8e8af26cbea262b163bcad",
		size:   371807752,
	}},
	{name: things.TextFile, download: download{
		url:    siglipFile("onnx/text_model_int8.onnx"),
		sha256: "3a0603d3a00c05a80a6ded4743c16aaac7b1e62cdcc7e362e7ce418659b96400",
		size:   283438275,
	}},
	{name: things.TokenizerFile, download: download{
		url:    siglipFile("tokenizer.json"),
		sha256: "cb9140fae3ac5122c972d37adf83e1248471a38147ad76f8215c8872c6fd8322",
		size:   34363039,
	}},
	{name: things.DetectorFile, download: download{
		url:    zooModel("object_detection_yolox/" + things.DetectorFile),
		sha256: "c5c2d13e59ae883e6af3b45daea64af4833a4951c92d116ec270d9ddbe998063",
		size:   35858002,
	}},
}

func siglipFile(path string) string {
	return "https://huggingface.co/onnx-community/siglip2-base-patch16-224-ONNX/resolve/" + siglipRevision + "/" + path
}

// PlanThings says whether things can be found on this system and how much
// it downloads.
func PlanThings(runtimeDir string) ModelPlan {
	return planModels(runtimeDir, thingsModels)
}

// InstallThings downloads ONNX Runtime, unless it is in runtimeDir, and the
// things models into dir.
func InstallThings(ctx context.Context, report Reporter, runtimeDir, dir string) error {
	return installModels(ctx, report, runtimeDir, dir, thingsModels)
}
