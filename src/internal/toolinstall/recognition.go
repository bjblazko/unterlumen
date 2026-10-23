package toolinstall

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/faces"
)

// zooRevision pins OpenCV's model zoo on Hugging Face to one commit.
const zooRevision = "d4938dfc9d4ec5d098bfa33e98b3f3345a236586"

var faceModels = []model{
	{name: faces.DetectorFile, download: download{
		url:    zooModel("face_detection_yunet/" + faces.DetectorFile),
		sha256: "8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4",
		size:   232589,
	}},
	{name: faces.RecognizerFile, download: download{
		url:    zooModel("face_recognition_sface/" + faces.RecognizerFile),
		sha256: "0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79",
		size:   38696353,
	}},
}

func zooModel(path string) string {
	return "https://huggingface.co/opencv/opencv_zoo/resolve/" + zooRevision + "/models/" + path
}

// PlanRecognition says whether face recognition can be installed on this
// system and how much it downloads.
func PlanRecognition(runtimeDir string) ModelPlan {
	return planModels(runtimeDir, faceModels)
}

// InstallRecognition downloads ONNX Runtime, unless it is in runtimeDir,
// and the face models into dir.
func InstallRecognition(ctx context.Context, report Reporter, runtimeDir, dir string) error {
	return installModels(ctx, report, runtimeDir, dir, faceModels)
}

// RemoveRecognition deletes what InstallRecognition put into dir.
func RemoveRecognition(dir string) error {
	if filepath.Base(dir) != "recognition" {
		return fmt.Errorf("%s is not a recognition folder", dir)
	}
	return os.RemoveAll(dir)
}
