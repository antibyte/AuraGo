package onnxrt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"unicode/utf16"
	"unsafe"

	"github.com/ebitengine/purego"
)

// APIVersion is the ONNX Runtime C API version AuraGo binds. The ORT 1.26
// CPU bundles and the 1.27 WebGPU wheels both serve it.
const APIVersion = 26

// Indices into struct OrtApi (include/onnxruntime/core/session/
// onnxruntime_c_api.h, v1.26.0), counted from 0. Verify any new entry against
// the header of the pinned release before adding it.
const (
	indexGetErrorMessage                       = 2
	indexCreateEnv                             = 3
	indexCreateSessionFromArray                = 8
	indexRun                                   = 9
	indexCreateSessionOptions                  = 10
	indexSetOptimizedModelFilePath             = 11
	indexEnableProfiling                       = 14
	indexSetSessionGraphOptimizationLevel      = 23
	indexSetIntraOpNumThreads                  = 24
	indexCreateTensorWithDataAsOrtValue        = 49
	indexGetTensorMutableData                  = 51
	indexGetTensorElementType                  = 60
	indexGetDimensionsCount                    = 61
	indexGetDimensions                         = 62
	indexGetTensorShapeElementCount            = 64
	indexGetTensorTypeAndShape                 = 65
	indexCreateCpuMemoryInfo                   = 69
	indexAllocatorFree                         = 76
	indexGetAllocatorWithDefaultOptions        = 78
	indexReleaseEnv                            = 92
	indexReleaseStatus                         = 93
	indexReleaseMemoryInfo                     = 94
	indexReleaseSession                        = 95
	indexReleaseValue                          = 96
	indexReleaseTensorTypeAndShapeInfo         = 99
	indexReleaseSessionOptions                 = 100
	indexSessionEndProfiling                   = 110
	indexGetAvailableProviders                 = 125
	indexReleaseAvailableProviders             = 126
	indexAddSessionConfigEntry                 = 130
	indexSessionOptionsAppendExecutionProvider = 216
)

const (
	tensorElementFloat = 1
	tensorElementInt64 = 7

	logSeverityWarning = 2
	// CreateCpuMemoryInfo arguments embeddings has always used
	// (OrtDeviceAllocator, OrtMemTypeCPUInput).
	cpuAllocatorType = 0
	cpuMemoryType    = -2
)

type apiBase struct {
	GetAPI           uintptr
	GetVersionString uintptr
}

type functions struct {
	getErrorMessage func(uintptr) *byte
	releaseStatus   func(uintptr)

	createEnv  func(int32, *byte, *uintptr) uintptr
	releaseEnv func(uintptr)

	createSessionOptions             func(*uintptr) uintptr
	setOptimizedModelFilePath        func(uintptr, unsafe.Pointer) uintptr
	enableProfiling                  func(uintptr, unsafe.Pointer) uintptr
	setSessionGraphOptimizationLevel func(uintptr, int32) uintptr
	setIntraOpNumThreads             func(uintptr, int32) uintptr
	addSessionConfigEntry            func(uintptr, *byte, *byte) uintptr
	appendExecutionProvider          func(uintptr, *byte, **byte, **byte, uintptr) uintptr
	releaseSessionOptions            func(uintptr)

	createSessionFromArray func(uintptr, unsafe.Pointer, uintptr, uintptr, *uintptr) uintptr
	run                    func(uintptr, uintptr, **byte, *uintptr, uintptr, **byte, uintptr, *uintptr) uintptr
	releaseSession         func(uintptr)
	sessionEndProfiling    func(uintptr, uintptr, **byte) uintptr

	createCpuMemoryInfo            func(int32, int32, *uintptr) uintptr
	releaseMemoryInfo              func(uintptr)
	allocatorFree                  func(uintptr, unsafe.Pointer) uintptr
	getAllocatorWithDefaultOptions func(*uintptr) uintptr
	createTensorWithData           func(uintptr, unsafe.Pointer, uintptr, *int64, uintptr, int32, *uintptr) uintptr
	getTensorMutableData           func(uintptr, *unsafe.Pointer) uintptr
	getTensorTypeAndShape          func(uintptr, *uintptr) uintptr
	getTensorElementType           func(uintptr, *int32) uintptr
	getDimensionsCount             func(uintptr, *uintptr) uintptr
	getDimensions                  func(uintptr, *int64, uintptr) uintptr
	getTensorShapeElementCount     func(uintptr, *uintptr) uintptr
	releaseTensorTypeAndShapeInfo  func(uintptr)
	releaseValue                   func(uintptr)

	getAvailableProviders     func(***byte, *int32) uintptr
	releaseAvailableProviders func(**byte, int32) uintptr
}

// Runtime is one loaded ONNX Runtime library with its environment and CPU
// memory info; every Session created from it shares both. Close all sessions
// before closing the runtime. Native code must only be loaded in worker
// processes, never in the AuraGo server process.
type Runtime struct {
	library    *Library
	functions  functions
	env        uintptr
	memoryInfo uintptr
	version    string
}

// Load finds the ONNX Runtime library below runtimeRoot, binds C API
// APIVersion and creates an environment that logs at warning level.
func Load(runtimeRoot, logID string) (_ *Runtime, resultErr error) {
	libraryPath, err := FindLibrary(runtimeRoot)
	if err != nil {
		return nil, err
	}
	library, err := OpenLibrary(libraryPath)
	if err != nil {
		return nil, err
	}
	r := &Runtime{library: library}
	defer func() {
		if resultErr != nil {
			_ = r.Close()
		}
	}()

	entry, err := library.Lookup("OrtGetApiBase")
	if err != nil {
		return nil, err
	}
	var getAPIBase func() *apiBase
	purego.RegisterFunc(&getAPIBase, entry)
	base := getAPIBase()
	if base == nil || base.GetAPI == 0 {
		return nil, fmt.Errorf("OrtGetApiBase returned no API")
	}
	if base.GetVersionString != 0 {
		var getVersionString func() *byte
		purego.RegisterFunc(&getVersionString, base.GetVersionString)
		r.version = cString(getVersionString())
	}
	var getAPI func(uint32) unsafe.Pointer
	purego.RegisterFunc(&getAPI, base.GetAPI)
	apiPointer := getAPI(APIVersion)
	if apiPointer == nil {
		return nil, fmt.Errorf("ONNX Runtime does not expose C API %d", APIVersion)
	}
	r.functions = bindFunctions(apiPointer)

	if strings.TrimSpace(logID) == "" {
		logID = "aurago"
	}
	logName := cBytes(logID)
	status := r.functions.createEnv(logSeverityWarning, &logName[0], &r.env)
	goruntime.KeepAlive(logName)
	if err := r.statusError(status); err != nil {
		return nil, fmt.Errorf("create ONNX Runtime environment: %w", err)
	}
	if err := r.statusError(r.functions.createCpuMemoryInfo(cpuAllocatorType, cpuMemoryType, &r.memoryInfo)); err != nil {
		return nil, fmt.Errorf("create ONNX CPU memory info: %w", err)
	}
	return r, nil
}

func bindFunctions(apiPointer unsafe.Pointer) functions {
	table := unsafe.Slice((*uintptr)(apiPointer), indexSessionOptionsAppendExecutionProvider+1)
	var f functions
	purego.RegisterFunc(&f.getErrorMessage, table[indexGetErrorMessage])
	purego.RegisterFunc(&f.releaseStatus, table[indexReleaseStatus])
	purego.RegisterFunc(&f.createEnv, table[indexCreateEnv])
	purego.RegisterFunc(&f.releaseEnv, table[indexReleaseEnv])
	purego.RegisterFunc(&f.createSessionOptions, table[indexCreateSessionOptions])
	purego.RegisterFunc(&f.setOptimizedModelFilePath, table[indexSetOptimizedModelFilePath])
	purego.RegisterFunc(&f.enableProfiling, table[indexEnableProfiling])
	purego.RegisterFunc(&f.setSessionGraphOptimizationLevel, table[indexSetSessionGraphOptimizationLevel])
	purego.RegisterFunc(&f.setIntraOpNumThreads, table[indexSetIntraOpNumThreads])
	purego.RegisterFunc(&f.addSessionConfigEntry, table[indexAddSessionConfigEntry])
	purego.RegisterFunc(&f.appendExecutionProvider, table[indexSessionOptionsAppendExecutionProvider])
	purego.RegisterFunc(&f.releaseSessionOptions, table[indexReleaseSessionOptions])
	purego.RegisterFunc(&f.createSessionFromArray, table[indexCreateSessionFromArray])
	purego.RegisterFunc(&f.run, table[indexRun])
	purego.RegisterFunc(&f.releaseSession, table[indexReleaseSession])
	purego.RegisterFunc(&f.sessionEndProfiling, table[indexSessionEndProfiling])
	purego.RegisterFunc(&f.createCpuMemoryInfo, table[indexCreateCpuMemoryInfo])
	purego.RegisterFunc(&f.releaseMemoryInfo, table[indexReleaseMemoryInfo])
	purego.RegisterFunc(&f.allocatorFree, table[indexAllocatorFree])
	purego.RegisterFunc(&f.getAllocatorWithDefaultOptions, table[indexGetAllocatorWithDefaultOptions])
	purego.RegisterFunc(&f.createTensorWithData, table[indexCreateTensorWithDataAsOrtValue])
	purego.RegisterFunc(&f.getTensorMutableData, table[indexGetTensorMutableData])
	purego.RegisterFunc(&f.getTensorTypeAndShape, table[indexGetTensorTypeAndShape])
	purego.RegisterFunc(&f.getTensorElementType, table[indexGetTensorElementType])
	purego.RegisterFunc(&f.getDimensionsCount, table[indexGetDimensionsCount])
	purego.RegisterFunc(&f.getDimensions, table[indexGetDimensions])
	purego.RegisterFunc(&f.getTensorShapeElementCount, table[indexGetTensorShapeElementCount])
	purego.RegisterFunc(&f.releaseTensorTypeAndShapeInfo, table[indexReleaseTensorTypeAndShapeInfo])
	purego.RegisterFunc(&f.releaseValue, table[indexReleaseValue])
	purego.RegisterFunc(&f.getAvailableProviders, table[indexGetAvailableProviders])
	purego.RegisterFunc(&f.releaseAvailableProviders, table[indexReleaseAvailableProviders])
	return f
}

func (r *Runtime) closed() bool {
	return r == nil || r.env == 0
}

// Version is the ORT release string, for example "1.27.0".
func (r *Runtime) Version() string {
	if r == nil {
		return ""
	}
	return r.version
}

// Providers lists the execution providers compiled into the library.
func (r *Runtime) Providers() ([]string, error) {
	if r.closed() {
		return nil, fmt.Errorf("ONNX Runtime is closed")
	}
	var providersPointer **byte
	var count int32
	if err := r.statusError(r.functions.getAvailableProviders(&providersPointer, &count)); err != nil {
		return nil, fmt.Errorf("get ONNX execution providers: %w", err)
	}
	if providersPointer == nil || count <= 0 {
		return nil, fmt.Errorf("ONNX Runtime reported no execution providers")
	}
	defer func() {
		_ = r.statusError(r.functions.releaseAvailableProviders(providersPointer, count))
	}()
	pointers := unsafe.Slice(providersPointer, count)
	providers := make([]string, 0, count)
	for _, pointer := range pointers {
		providers = append(providers, cString(pointer))
	}
	return providers, nil
}

// Close releases the environment and unloads the library.
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	if r.memoryInfo != 0 && r.functions.releaseMemoryInfo != nil {
		r.functions.releaseMemoryInfo(r.memoryInfo)
		r.memoryInfo = 0
	}
	if r.env != 0 && r.functions.releaseEnv != nil {
		r.functions.releaseEnv(r.env)
		r.env = 0
	}
	if r.library == nil {
		return nil
	}
	err := r.library.Close()
	r.library = nil
	return err
}

func (r *Runtime) statusError(status uintptr) error {
	if status == 0 {
		return nil
	}
	message := cString(r.functions.getErrorMessage(status))
	r.functions.releaseStatus(status)
	if message == "" {
		message = "unknown ONNX Runtime error"
	}
	return errors.New(message)
}

// LibraryName is the unversioned ONNX Runtime library name of this platform.
func LibraryName() string {
	switch goruntime.GOOS {
	case "windows":
		return "onnxruntime.dll"
	case "darwin":
		return "libonnxruntime.dylib"
	default:
		return "libonnxruntime.so"
	}
}

// FindLibrary returns the absolute path of the first ONNX Runtime library
// below root, so the loader never falls back to its search path (Windows
// ships its own System32\onnxruntime.dll). Versioned names such as
// libonnxruntime.so.1.27.0 and libonnxruntime.1.27.0.dylib are accepted;
// symlinks are never extracted, so the versioned file is the one present.
func FindLibrary(root string) (string, error) {
	expected := LibraryName()
	expectedLower := strings.ToLower(expected)
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve ONNX runtime directory %s: %w", root, err)
	}
	var matches []string
	err = filepath.WalkDir(absoluteRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if name == expectedLower || strings.HasPrefix(name, expectedLower+".") ||
			(goruntime.GOOS == "darwin" && strings.HasPrefix(name, "libonnxruntime.") && strings.HasSuffix(name, ".dylib")) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan ONNX runtime: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("%s not found below %s", expected, root)
	}
	return matches[0], nil
}

func cString(pointer *byte) string {
	if pointer == nil {
		return ""
	}
	length := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(pointer), length)) != 0 {
		length++
	}
	return string(unsafe.Slice(pointer, length))
}

func cBytes(value string) []byte {
	return append([]byte(value), 0)
}

// ortPath converts a path to ORTCHAR_T: UTF-16 on Windows, UTF-8 elsewhere.
// The second result must stay alive until the C call returns.
func ortPath(path string) (unsafe.Pointer, any) {
	if goruntime.GOOS == "windows" {
		wide := append(utf16.Encode([]rune(path)), 0)
		return unsafe.Pointer(&wide[0]), wide
	}
	narrow := cBytes(path)
	return unsafe.Pointer(&narrow[0]), narrow
}

func containsFold(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(value, expected) {
			return true
		}
	}
	return false
}
