// Package onnx runs ONNX models with ONNX Runtime, loaded from its shared
// library at run time through purego. Unterlumen stays a cgo-free single
// binary: without the library nothing here works, and nothing else needs it
// (ADR-0051).
//
// Only what Unterlumen's models need is bound — float32 tensors in, float32
// tensors out, on the CPU.
package onnx

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// apiVersion is the oldest ONNX Runtime C API whose functions are used here:
// 1.17. A newer library serves an older API version unchanged.
const apiVersion = 17

// Runtime is a loaded ONNX Runtime library and its environment.
type Runtime struct {
	api *api
	env unsafe.Pointer
	mem unsafe.Pointer // CPU memory info for input tensors
}

var (
	loadOnce sync.Once
	loaded   *Runtime
	loadErr  error
)

// Load loads the ONNX Runtime library at libPath. A process loads it once:
// later calls return the first result, whatever path they name.
func Load(libPath string) (*Runtime, error) {
	loadOnce.Do(func() { loaded, loadErr = load(libPath) })
	return loaded, loadErr
}

func load(libPath string) (*Runtime, error) {
	lib, err := openLibrary(libPath)
	if err != nil {
		return nil, fmt.Errorf("ONNX Runtime could not be loaded from %s: %w", libPath, err)
	}
	a, err := bindAPI(lib)
	if err != nil {
		return nil, err
	}
	r := &Runtime{api: a}
	if err := r.status(a.createEnv(ortLoggingLevelError, "unterlumen\x00", unsafe.Pointer(&r.env))); err != nil {
		return nil, err
	}
	// Official Windows builds report to Windows' own telemetry unless told not to.
	if err := r.status(a.disableTelemetryEvents(r.env)); err != nil {
		return nil, err
	}
	if err := r.status(a.createCPUMemoryInfo(ortArenaAllocator, ortMemTypeDefault, unsafe.Pointer(&r.mem))); err != nil {
		return nil, err
	}
	return r, nil
}

// bindAPI finds the function table of apiVersion in the library.
func bindAPI(lib uintptr) (*api, error) {
	sym, err := symbol(lib, "OrtGetApiBase")
	if err != nil {
		return nil, fmt.Errorf("this is not an ONNX Runtime library: %w", err)
	}
	var getAPIBase func() *apiBase
	purego.RegisterFunc(&getAPIBase, sym)
	base := getAPIBase()
	var getAPI func(uint32) *apiTable
	var version func() string
	purego.RegisterFunc(&getAPI, base.getAPI)
	purego.RegisterFunc(&version, base.getVersionString)
	table := getAPI(apiVersion)
	if table == nil {
		return nil, fmt.Errorf("ONNX Runtime %s is older than 1.%d, which Unterlumen needs", version(), apiVersion)
	}
	return newAPI(table, version()), nil
}

// Version is the loaded library's version, as it reports it.
func (r *Runtime) Version() string { return r.api.version }

// status turns an OrtStatus into an error and releases it.
func (r *Runtime) status(st unsafe.Pointer) error {
	if st == nil {
		return nil
	}
	msg := r.api.getErrorMessage(st)
	r.api.releaseStatus(st)
	return errors.New("ONNX Runtime: " + msg)
}

// goString copies a NUL-terminated C string.
func goString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

// keepAlive keeps Go memory handed to ONNX Runtime from being collected
// before the call that uses it returns.
func keepAlive(v ...any) { runtime.KeepAlive(v) }
