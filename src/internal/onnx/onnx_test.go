package onnx

import (
	"os"
	"testing"
)

// The library and a model are not part of the repository: these tests run
// where UNTERLUMEN_ONNX_LIB names libonnxruntime and UNTERLUMEN_ONNX_MODEL a
// model taking one float32 image tensor of shape [1,3,640,640] (YuNet).
func runtimeForTest(t *testing.T) (*Runtime, []byte) {
	lib, model := os.Getenv("UNTERLUMEN_ONNX_LIB"), os.Getenv("UNTERLUMEN_ONNX_MODEL")
	if lib == "" || model == "" {
		t.Skip("UNTERLUMEN_ONNX_LIB and UNTERLUMEN_ONNX_MODEL are not set")
	}
	rt, err := Load(lib)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(model)
	if err != nil {
		t.Fatal(err)
	}
	return rt, data
}

func TestRunReturnsEveryOutput(t *testing.T) {
	rt, model := runtimeForTest(t)
	s, err := rt.NewSession(model, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	in := Tensor{Shape: []int64{1, 3, 640, 640}, Data: make([]float32, 3*640*640)}
	out, err := s.Run(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(s.OutputNames()) {
		t.Fatalf("got %d outputs for %d names", len(out), len(s.OutputNames()))
	}
	for i, o := range out {
		t.Logf("%s %v", s.OutputNames()[i], o.Shape)
	}
}

func TestRunRejectsAWrongShape(t *testing.T) {
	rt, model := runtimeForTest(t)
	s, err := rt.NewSession(model, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Run(Tensor{Shape: []int64{1, 3, 10}, Data: make([]float32, 30)}); err == nil {
		t.Fatal("a tensor of the wrong shape was accepted")
	}
}

func TestNewTensorRejectsMismatchedData(t *testing.T) {
	r := &Runtime{}
	if _, err := r.newTensor(Tensor{Shape: []int64{2, 2}, Data: make([]float32, 3)}); err == nil {
		t.Fatal("4 values were promised and 3 accepted")
	}
}

func TestNewTensorRejectsMismatchedInts(t *testing.T) {
	r := &Runtime{}
	if _, err := r.newTensor(Tensor{Shape: []int64{1, 77}, Ints: make([]int64, 76)}); err == nil {
		t.Fatal("77 token IDs were promised and 76 accepted")
	}
}
