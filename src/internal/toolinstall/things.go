package toolinstall

import (
	"context"

	"huepattl.de/unterlumen/internal/things"
)

// clipRevision pins Xenova's ONNX export of OpenAI's CLIP ViT-B/16 on
// Hugging Face to one commit. The image encoder is taken at full precision:
// its 8-bit version picked another best word for a quarter of the photos
// tried. The text encoder's 8-bit version agrees with the full one.
const clipRevision = "342fdf2f67aded64d138ff074745fb4a5d2bba5f"

var thingsModels = []model{
	{name: things.ImageFile, download: download{
		url:    clipFile("onnx/vision_model.onnx"),
		sha256: "b5170b47c0ecc667c9af5b42ed460c341377706d1466d177798c65fe49ea1e6b",
		size:   345060583,
	}},
	{name: things.TextFile, download: download{
		url:    clipFile("onnx/text_model_quantized.onnx"),
		sha256: "9106b51e6c663a56b99182ec617c2b3d53577b037e7e24a7717eb78048a0c97a",
		size:   64504507,
	}},
	{name: things.VocabFile, download: download{
		url:    clipFile("vocab.json"),
		sha256: "5047b556ce86ccaf6aa22b3ffccfc52d391ea4accdab9c2f2407da5b742d4363",
		size:   862328,
	}},
	{name: things.MergesFile, download: download{
		url:    clipFile("merges.txt"),
		sha256: "9fd691f7c8039210e0fced15865466c65820d09b63988b0174bfe25de299051a",
		size:   524619,
	}},
	{name: things.DetectorFile, download: download{
		url:    zooModel("object_detection_yolox/" + things.DetectorFile),
		sha256: "c5c2d13e59ae883e6af3b45daea64af4833a4951c92d116ec270d9ddbe998063",
		size:   35858002,
	}},
}

func clipFile(path string) string {
	return "https://huggingface.co/Xenova/clip-vit-base-patch16/resolve/" + clipRevision + "/" + path
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
