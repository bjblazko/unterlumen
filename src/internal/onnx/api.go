package onnx

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// apiBase is OrtApiBase.
type apiBase struct {
	getAPI           uintptr
	getVersionString uintptr
}

// apiTable is the start of OrtApi, a table of function pointers. Entries are
// only ever appended, so an index stays valid in every later version.
type apiTable [apiTableLen]uintptr

// The indexes of the functions used, counted in onnxruntime_c_api.h's struct OrtApi.
const (
	idxGetErrorMessage                = 2
	idxCreateEnv                      = 3
	idxDisableTelemetryEvents         = 6
	idxCreateSessionFromArray         = 8
	idxRun                            = 9
	idxCreateSessionOptions           = 10
	idxSetSessionGraphOptimization    = 23
	idxSetIntraOpNumThreads           = 24
	idxSetInterOpNumThreads           = 25
	idxSessionGetInputCount           = 30
	idxSessionGetOutputCount          = 31
	idxSessionGetInputName            = 36
	idxSessionGetOutputName           = 37
	idxCreateTensorWithDataAsOrtValue = 49
	idxGetTensorMutableData           = 51
	idxGetTensorElementType           = 60
	idxGetDimensionsCount             = 61
	idxGetDimensions                  = 62
	idxGetTensorTypeAndShape          = 65
	idxCreateCPUMemoryInfo            = 69
	idxAllocatorFree                  = 76
	idxGetAllocatorWithDefaultOptions = 78
	idxReleaseStatus                  = 93
	idxReleaseSession                 = 95
	idxReleaseValue                   = 96
	idxReleaseTensorTypeAndShapeInfo  = 99
	idxReleaseSessionOptions          = 100

	apiTableLen = idxReleaseSessionOptions + 1
)

// Values of ONNX Runtime's enums.
const (
	ortLoggingLevelError      = 3
	ortArenaAllocator         = 1
	ortMemTypeDefault         = 0
	ortTensorElementFloat     = 1
	ortTensorElementInt64     = 7
	ortEnableAllOptimizations = 99
)

// api holds the bound functions. Every OrtStatus* is returned as an
// unsafe.Pointer, nil meaning success; out-parameters are pointers to where
// ONNX Runtime writes its result.
type api struct {
	version string

	getErrorMessage           func(status unsafe.Pointer) string
	createEnv                 func(level int32, logID string, out unsafe.Pointer) unsafe.Pointer
	disableTelemetryEvents    func(env unsafe.Pointer) unsafe.Pointer
	createSessionFromArray    func(env unsafe.Pointer, model unsafe.Pointer, size uintptr, opts unsafe.Pointer, out unsafe.Pointer) unsafe.Pointer
	run                       func(sess, runOpts, inNames, inValues unsafe.Pointer, inCount uintptr, outNames unsafe.Pointer, outCount uintptr, outValues unsafe.Pointer) unsafe.Pointer
	createSessionOptions      func(out unsafe.Pointer) unsafe.Pointer
	setGraphOptimization      func(opts unsafe.Pointer, level int32) unsafe.Pointer
	setIntraOpNumThreads      func(opts unsafe.Pointer, n int32) unsafe.Pointer
	setInterOpNumThreads      func(opts unsafe.Pointer, n int32) unsafe.Pointer
	sessionGetInputCount      func(sess unsafe.Pointer, out unsafe.Pointer) unsafe.Pointer
	sessionGetOutputCount     func(sess unsafe.Pointer, out unsafe.Pointer) unsafe.Pointer
	sessionGetInputName       func(sess unsafe.Pointer, i uintptr, alloc unsafe.Pointer, out unsafe.Pointer) unsafe.Pointer
	sessionGetOutputName      func(sess unsafe.Pointer, i uintptr, alloc unsafe.Pointer, out unsafe.Pointer) unsafe.Pointer
	createTensorWithData      func(mem unsafe.Pointer, data unsafe.Pointer, size uintptr, shape unsafe.Pointer, dims uintptr, elemType int32, out unsafe.Pointer) unsafe.Pointer
	getTensorMutableData      func(value unsafe.Pointer, out unsafe.Pointer) unsafe.Pointer
	getTensorElementType      func(info unsafe.Pointer, out unsafe.Pointer) unsafe.Pointer
	getDimensionsCount        func(info unsafe.Pointer, out unsafe.Pointer) unsafe.Pointer
	getDimensions             func(info unsafe.Pointer, out unsafe.Pointer, n uintptr) unsafe.Pointer
	getTensorTypeAndShape     func(value unsafe.Pointer, out unsafe.Pointer) unsafe.Pointer
	createCPUMemoryInfo       func(allocType int32, memType int32, out unsafe.Pointer) unsafe.Pointer
	allocatorFree             func(alloc unsafe.Pointer, p unsafe.Pointer) unsafe.Pointer
	getDefaultAllocator       func(out unsafe.Pointer) unsafe.Pointer
	releaseStatus             func(unsafe.Pointer)
	releaseSession            func(unsafe.Pointer)
	releaseValue              func(unsafe.Pointer)
	releaseTensorTypeAndShape func(unsafe.Pointer)
	releaseSessionOptions     func(unsafe.Pointer)
}

func newAPI(t *apiTable, version string) *api {
	a := &api{version: version}
	bind := map[int]any{
		idxGetErrorMessage:                &a.getErrorMessage,
		idxCreateEnv:                      &a.createEnv,
		idxDisableTelemetryEvents:         &a.disableTelemetryEvents,
		idxCreateSessionFromArray:         &a.createSessionFromArray,
		idxRun:                            &a.run,
		idxCreateSessionOptions:           &a.createSessionOptions,
		idxSetSessionGraphOptimization:    &a.setGraphOptimization,
		idxSetIntraOpNumThreads:           &a.setIntraOpNumThreads,
		idxSetInterOpNumThreads:           &a.setInterOpNumThreads,
		idxSessionGetInputCount:           &a.sessionGetInputCount,
		idxSessionGetOutputCount:          &a.sessionGetOutputCount,
		idxSessionGetInputName:            &a.sessionGetInputName,
		idxSessionGetOutputName:           &a.sessionGetOutputName,
		idxCreateTensorWithDataAsOrtValue: &a.createTensorWithData,
		idxGetTensorMutableData:           &a.getTensorMutableData,
		idxGetTensorElementType:           &a.getTensorElementType,
		idxGetDimensionsCount:             &a.getDimensionsCount,
		idxGetDimensions:                  &a.getDimensions,
		idxGetTensorTypeAndShape:          &a.getTensorTypeAndShape,
		idxCreateCPUMemoryInfo:            &a.createCPUMemoryInfo,
		idxAllocatorFree:                  &a.allocatorFree,
		idxGetAllocatorWithDefaultOptions: &a.getDefaultAllocator,
		idxReleaseStatus:                  &a.releaseStatus,
		idxReleaseSession:                 &a.releaseSession,
		idxReleaseValue:                   &a.releaseValue,
		idxReleaseTensorTypeAndShapeInfo:  &a.releaseTensorTypeAndShape,
		idxReleaseSessionOptions:          &a.releaseSessionOptions,
	}
	for idx, fn := range bind {
		purego.RegisterFunc(fn, t[idx])
	}
	return a
}
