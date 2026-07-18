package embeddings

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	ortAPIVersion = 26

	ortIndexGetErrorMessage                       = 2
	ortIndexCreateEnv                             = 3
	ortIndexCreateSessionFromArray                = 8
	ortIndexRun                                   = 9
	ortIndexCreateSessionOptions                  = 10
	ortIndexSetSessionExecutionMode               = 13
	ortIndexEnableProfiling                       = 14
	ortIndexDisableMemPattern                     = 17
	ortIndexSetSessionGraphOptimizationLevel      = 23
	ortIndexSetIntraOpNumThreads                  = 24
	ortIndexCreateTensorWithDataAsOrtValue        = 49
	ortIndexGetTensorMutableData                  = 51
	ortIndexGetTensorElementType                  = 60
	ortIndexGetDimensionsCount                    = 61
	ortIndexGetDimensions                         = 62
	ortIndexGetTensorShapeElementCount            = 64
	ortIndexGetTensorTypeAndShape                 = 65
	ortIndexCreateCpuMemoryInfo                   = 69
	ortIndexAllocatorFree                         = 76
	ortIndexGetAllocatorWithDefaultOptions        = 78
	ortIndexReleaseEnv                            = 92
	ortIndexReleaseStatus                         = 93
	ortIndexReleaseMemoryInfo                     = 94
	ortIndexReleaseSession                        = 95
	ortIndexReleaseValue                          = 96
	ortIndexReleaseTensorTypeAndShapeInfo         = 99
	ortIndexReleaseSessionOptions                 = 100
	ortIndexSessionEndProfiling                   = 110
	ortIndexGetAvailableProviders                 = 125
	ortIndexReleaseAvailableProviders             = 126
	ortIndexSessionOptionsAppendExecutionProvider = 216

	ortTensorElementFloat = 1
	ortTensorElementInt64 = 7
)

type ortAPIBase struct {
	GetAPI           uintptr
	GetVersionString uintptr
}

type ortFunctions struct {
	getErrorMessage func(uintptr) *byte
	releaseStatus   func(uintptr)

	createEnv  func(int32, *byte, *uintptr) uintptr
	releaseEnv func(uintptr)

	createSessionOptions             func(*uintptr) uintptr
	setSessionExecutionMode          func(uintptr, int32) uintptr
	enableProfiling                  func(uintptr, unsafe.Pointer) uintptr
	disableMemPattern                func(uintptr) uintptr
	setSessionGraphOptimizationLevel func(uintptr, int32) uintptr
	setIntraOpNumThreads             func(uintptr, int32) uintptr
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

type ortSession struct {
	library        nativeLibrary
	functions      ortFunctions
	env            uintptr
	session        uintptr
	memoryInfo     uintptr
	providers      []string
	backend        string
	verified       bool
	version        string
	profileEnabled bool
	profileEnded   bool
}

func newORTSession(runtimeRoot, modelFile, backend string) (_ *ortSession, resultErr error) {
	libraryPath, err := findONNXRuntimeLibrary(runtimeRoot)
	if err != nil {
		return nil, err
	}
	library, err := openNativeLibrary(libraryPath)
	if err != nil {
		return nil, err
	}
	session := &ortSession{library: library, backend: backend}
	defer func() {
		if resultErr != nil {
			_ = session.Close()
		}
	}()

	entry, err := library.lookup("OrtGetApiBase")
	if err != nil {
		return nil, err
	}
	var getAPIBase func() *ortAPIBase
	purego.RegisterFunc(&getAPIBase, entry)
	apiBase := getAPIBase()
	if apiBase == nil || apiBase.GetAPI == 0 {
		return nil, fmt.Errorf("OrtGetApiBase returned no API")
	}
	if apiBase.GetVersionString != 0 {
		var getVersionString func() *byte
		purego.RegisterFunc(&getVersionString, apiBase.GetVersionString)
		session.version = cString(getVersionString())
	}
	var getAPI func(uint32) unsafe.Pointer
	purego.RegisterFunc(&getAPI, apiBase.GetAPI)
	apiPointer := getAPI(ortAPIVersion)
	if apiPointer == nil {
		return nil, fmt.Errorf("ONNX Runtime does not expose C API %d", ortAPIVersion)
	}
	session.functions = bindORTFunctions(apiPointer)

	logID := append([]byte("aurago-embeddings"), 0)
	if err := session.statusError(session.functions.createEnv(2, &logID[0], &session.env)); err != nil {
		return nil, fmt.Errorf("create ONNX Runtime environment: %w", err)
	}
	if err := session.statusError(session.functions.createCpuMemoryInfo(0, -2, &session.memoryInfo)); err != nil {
		return nil, fmt.Errorf("create ONNX CPU memory info: %w", err)
	}
	providers, err := session.availableProviders()
	if err != nil {
		return nil, err
	}
	session.providers = providers

	var options uintptr
	if err := session.statusError(session.functions.createSessionOptions(&options)); err != nil {
		return nil, fmt.Errorf("create ONNX session options: %w", err)
	}
	defer session.functions.releaseSessionOptions(options)
	_ = session.statusError(session.functions.setSessionGraphOptimizationLevel(options, 99))
	threads := goruntime.NumCPU()
	if threads > 8 {
		threads = 8
	}
	if threads > 0 {
		_ = session.statusError(session.functions.setIntraOpNumThreads(options, int32(threads)))
	}
	if backend != "cpu" {
		provider := onnxProviderName(backend)
		if provider == "" {
			return nil, fmt.Errorf("unsupported ONNX backend %q", backend)
		}
		if !containsStringFold(providers, provider) {
			return nil, fmt.Errorf("%s is not available in this ONNX Runtime bundle (available: %s)", provider, strings.Join(providers, ", "))
		}
		if backend == "directml" {
			if err := session.statusError(session.functions.disableMemPattern(options)); err != nil {
				return nil, fmt.Errorf("disable memory pattern for DirectML: %w", err)
			}
			if err := session.statusError(session.functions.setSessionExecutionMode(options, 0)); err != nil {
				return nil, fmt.Errorf("set sequential execution for DirectML: %w", err)
			}
		}
		providerName := append([]byte(provider), 0)
		if err := session.statusError(session.functions.appendExecutionProvider(options, &providerName[0], nil, nil, 0)); err != nil {
			return nil, fmt.Errorf("enable %s: %w", provider, err)
		}
		profilePrefix := filepath.Join(
			os.TempDir(),
			fmt.Sprintf("aurago-ort-%d-%d", os.Getpid(), time.Now().UnixNano()),
		)
		profilePointer, profileKeepAlive := ortPathPointer(profilePrefix)
		if err := session.statusError(session.functions.enableProfiling(options, profilePointer)); err != nil {
			return nil, fmt.Errorf("enable %s execution profiling: %w", provider, err)
		}
		goruntime.KeepAlive(profileKeepAlive)
		session.profileEnabled = true
	}

	modelData, err := os.ReadFile(modelFile)
	if err != nil {
		return nil, fmt.Errorf("read ONNX model: %w", err)
	}
	if len(modelData) == 0 {
		return nil, fmt.Errorf("ONNX model is empty")
	}
	if err := session.statusError(session.functions.createSessionFromArray(
		session.env,
		unsafe.Pointer(&modelData[0]),
		uintptr(len(modelData)),
		options,
		&session.session,
	)); err != nil {
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}
	goruntime.KeepAlive(modelData)
	return session, nil
}

func bindORTFunctions(apiPointer unsafe.Pointer) ortFunctions {
	table := unsafe.Slice((*uintptr)(apiPointer), ortIndexSessionOptionsAppendExecutionProvider+1)
	var functions ortFunctions
	purego.RegisterFunc(&functions.getErrorMessage, table[ortIndexGetErrorMessage])
	purego.RegisterFunc(&functions.releaseStatus, table[ortIndexReleaseStatus])
	purego.RegisterFunc(&functions.createEnv, table[ortIndexCreateEnv])
	purego.RegisterFunc(&functions.releaseEnv, table[ortIndexReleaseEnv])
	purego.RegisterFunc(&functions.createSessionOptions, table[ortIndexCreateSessionOptions])
	purego.RegisterFunc(&functions.setSessionExecutionMode, table[ortIndexSetSessionExecutionMode])
	purego.RegisterFunc(&functions.enableProfiling, table[ortIndexEnableProfiling])
	purego.RegisterFunc(&functions.disableMemPattern, table[ortIndexDisableMemPattern])
	purego.RegisterFunc(&functions.setSessionGraphOptimizationLevel, table[ortIndexSetSessionGraphOptimizationLevel])
	purego.RegisterFunc(&functions.setIntraOpNumThreads, table[ortIndexSetIntraOpNumThreads])
	purego.RegisterFunc(&functions.appendExecutionProvider, table[ortIndexSessionOptionsAppendExecutionProvider])
	purego.RegisterFunc(&functions.releaseSessionOptions, table[ortIndexReleaseSessionOptions])
	purego.RegisterFunc(&functions.createSessionFromArray, table[ortIndexCreateSessionFromArray])
	purego.RegisterFunc(&functions.run, table[ortIndexRun])
	purego.RegisterFunc(&functions.releaseSession, table[ortIndexReleaseSession])
	purego.RegisterFunc(&functions.sessionEndProfiling, table[ortIndexSessionEndProfiling])
	purego.RegisterFunc(&functions.createCpuMemoryInfo, table[ortIndexCreateCpuMemoryInfo])
	purego.RegisterFunc(&functions.releaseMemoryInfo, table[ortIndexReleaseMemoryInfo])
	purego.RegisterFunc(&functions.allocatorFree, table[ortIndexAllocatorFree])
	purego.RegisterFunc(&functions.getAllocatorWithDefaultOptions, table[ortIndexGetAllocatorWithDefaultOptions])
	purego.RegisterFunc(&functions.createTensorWithData, table[ortIndexCreateTensorWithDataAsOrtValue])
	purego.RegisterFunc(&functions.getTensorMutableData, table[ortIndexGetTensorMutableData])
	purego.RegisterFunc(&functions.getTensorTypeAndShape, table[ortIndexGetTensorTypeAndShape])
	purego.RegisterFunc(&functions.getTensorElementType, table[ortIndexGetTensorElementType])
	purego.RegisterFunc(&functions.getDimensionsCount, table[ortIndexGetDimensionsCount])
	purego.RegisterFunc(&functions.getDimensions, table[ortIndexGetDimensions])
	purego.RegisterFunc(&functions.getTensorShapeElementCount, table[ortIndexGetTensorShapeElementCount])
	purego.RegisterFunc(&functions.releaseTensorTypeAndShapeInfo, table[ortIndexReleaseTensorTypeAndShapeInfo])
	purego.RegisterFunc(&functions.releaseValue, table[ortIndexReleaseValue])
	purego.RegisterFunc(&functions.getAvailableProviders, table[ortIndexGetAvailableProviders])
	purego.RegisterFunc(&functions.releaseAvailableProviders, table[ortIndexReleaseAvailableProviders])
	return functions
}

func (session *ortSession) Embed(batch tokenBatch) ([][]float32, error) {
	if session == nil || session.session == 0 {
		return nil, fmt.Errorf("ONNX session is closed")
	}
	shape := []int64{int64(batch.BatchSize), int64(batch.SequenceSize)}
	inputIDs, err := session.newTensor(batch.InputIDs, shape, ortTensorElementInt64)
	if err != nil {
		return nil, fmt.Errorf("create input_ids tensor: %w", err)
	}
	defer session.functions.releaseValue(inputIDs)
	attentionMask, err := session.newTensor(batch.AttentionMask, shape, ortTensorElementInt64)
	if err != nil {
		return nil, fmt.Errorf("create attention_mask tensor: %w", err)
	}
	defer session.functions.releaseValue(attentionMask)

	inputNameData := [][]byte{append([]byte("input_ids"), 0), append([]byte("attention_mask"), 0)}
	inputNames := []*byte{&inputNameData[0][0], &inputNameData[1][0]}
	inputValues := []uintptr{inputIDs, attentionMask}
	outputNameData := append([]byte("last_hidden_state"), 0)
	outputNames := []*byte{&outputNameData[0]}
	outputValues := []uintptr{0}
	status := session.functions.run(
		session.session,
		0,
		&inputNames[0],
		&inputValues[0],
		uintptr(len(inputValues)),
		&outputNames[0],
		1,
		&outputValues[0],
	)
	goruntime.KeepAlive(batch.InputIDs)
	goruntime.KeepAlive(batch.AttentionMask)
	goruntime.KeepAlive(inputNameData)
	goruntime.KeepAlive(outputNameData)
	if err := session.statusError(status); err != nil {
		return nil, fmt.Errorf("run ONNX inference: %w", err)
	}
	if outputValues[0] == 0 {
		return nil, fmt.Errorf("ONNX inference returned no output")
	}
	defer session.functions.releaseValue(outputValues[0])
	vectors, err := session.readCLSOutput(outputValues[0], batch.BatchSize, batch.SequenceSize)
	if err != nil {
		return nil, err
	}
	if session.profileEnabled && !session.profileEnded {
		session.verified = session.verifyGPUExecution()
	}
	return vectors, nil
}

func (session *ortSession) verifyGPUExecution() bool {
	session.profileEnded = true
	var allocator uintptr
	if err := session.statusError(session.functions.getAllocatorWithDefaultOptions(&allocator)); err != nil || allocator == 0 {
		return false
	}
	var profileName *byte
	if err := session.statusError(session.functions.sessionEndProfiling(session.session, allocator, &profileName)); err != nil || profileName == nil {
		return false
	}
	path := cString(profileName)
	_ = session.statusError(session.functions.allocatorFree(allocator, unsafe.Pointer(profileName)))
	defer os.Remove(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var events []struct {
		Category string         `json:"cat"`
		Args     map[string]any `json:"args"`
	}
	if json.Unmarshal(raw, &events) != nil {
		return false
	}
	expectedProvider := onnxProviderName(session.backend)
	for _, event := range events {
		provider, _ := event.Args["provider"].(string)
		if strings.EqualFold(provider, expectedProvider) &&
			(strings.EqualFold(event.Category, "Node") || event.Category == "") {
			return true
		}
	}
	return false
}

func ortPathPointer(path string) (unsafe.Pointer, any) {
	if goruntime.GOOS == "windows" {
		wide := append(utf16.Encode([]rune(path)), 0)
		return unsafe.Pointer(&wide[0]), wide
	}
	narrow := append([]byte(path), 0)
	return unsafe.Pointer(&narrow[0]), narrow
}

func (session *ortSession) newTensor(data []int64, shape []int64, elementType int32) (uintptr, error) {
	if len(data) == 0 || len(shape) == 0 {
		return 0, fmt.Errorf("tensor data and shape are required")
	}
	var value uintptr
	status := session.functions.createTensorWithData(
		session.memoryInfo,
		unsafe.Pointer(&data[0]),
		uintptr(len(data))*unsafe.Sizeof(data[0]),
		&shape[0],
		uintptr(len(shape)),
		elementType,
		&value,
	)
	if err := session.statusError(status); err != nil {
		return 0, err
	}
	return value, nil
}

func (session *ortSession) readCLSOutput(value uintptr, batchSize, sequenceSize int) ([][]float32, error) {
	var shapeInfo uintptr
	if err := session.statusError(session.functions.getTensorTypeAndShape(value, &shapeInfo)); err != nil {
		return nil, fmt.Errorf("get output shape: %w", err)
	}
	defer session.functions.releaseTensorTypeAndShapeInfo(shapeInfo)
	var elementType int32
	if err := session.statusError(session.functions.getTensorElementType(shapeInfo, &elementType)); err != nil {
		return nil, fmt.Errorf("get output element type: %w", err)
	}
	if elementType != ortTensorElementFloat {
		return nil, fmt.Errorf("ONNX output element type = %d, want float32", elementType)
	}
	var dimensionCount uintptr
	if err := session.statusError(session.functions.getDimensionsCount(shapeInfo, &dimensionCount)); err != nil {
		return nil, fmt.Errorf("get output dimension count: %w", err)
	}
	dimensions := make([]int64, dimensionCount)
	if len(dimensions) > 0 {
		if err := session.statusError(session.functions.getDimensions(shapeInfo, &dimensions[0], dimensionCount)); err != nil {
			return nil, fmt.Errorf("get output dimensions: %w", err)
		}
	}
	var elementCount uintptr
	if err := session.statusError(session.functions.getTensorShapeElementCount(shapeInfo, &elementCount)); err != nil {
		return nil, fmt.Errorf("get output element count: %w", err)
	}
	var dataPointer unsafe.Pointer
	if err := session.statusError(session.functions.getTensorMutableData(value, &dataPointer)); err != nil {
		return nil, fmt.Errorf("get output data: %w", err)
	}
	if dataPointer == nil {
		return nil, fmt.Errorf("ONNX output data is nil")
	}
	data := unsafe.Slice((*float32)(dataPointer), elementCount)
	result := make([][]float32, batchSize)
	switch {
	case len(dimensions) == 3 &&
		int(dimensions[0]) == batchSize &&
		int(dimensions[1]) == sequenceSize &&
		int(dimensions[2]) == GraniteDimensions:
		rowSize := sequenceSize * GraniteDimensions
		for batchIndex := 0; batchIndex < batchSize; batchIndex++ {
			start := batchIndex * rowSize
			result[batchIndex] = append([]float32(nil), data[start:start+GraniteDimensions]...)
		}
	case len(dimensions) == 2 &&
		int(dimensions[0]) == batchSize &&
		int(dimensions[1]) == GraniteDimensions:
		for batchIndex := 0; batchIndex < batchSize; batchIndex++ {
			start := batchIndex * GraniteDimensions
			result[batchIndex] = append([]float32(nil), data[start:start+GraniteDimensions]...)
		}
	default:
		return nil, fmt.Errorf("unexpected ONNX output shape %v", dimensions)
	}
	for i := range result {
		if err := l2Normalize(result[i]); err != nil {
			return nil, fmt.Errorf("normalize output %d: %w", i, err)
		}
	}
	return result, nil
}

func (session *ortSession) statusError(status uintptr) error {
	if status == 0 {
		return nil
	}
	message := cString(session.functions.getErrorMessage(status))
	session.functions.releaseStatus(status)
	if message == "" {
		message = "unknown ONNX Runtime error"
	}
	return fmt.Errorf("%s", message)
}

func (session *ortSession) availableProviders() ([]string, error) {
	var providersPointer **byte
	var count int32
	if err := session.statusError(session.functions.getAvailableProviders(&providersPointer, &count)); err != nil {
		return nil, fmt.Errorf("get ONNX execution providers: %w", err)
	}
	if providersPointer == nil || count <= 0 {
		return nil, fmt.Errorf("ONNX Runtime reported no execution providers")
	}
	defer func() {
		_ = session.statusError(session.functions.releaseAvailableProviders(providersPointer, count))
	}()
	pointers := unsafe.Slice(providersPointer, count)
	providers := make([]string, 0, count)
	for _, pointer := range pointers {
		providers = append(providers, cString(pointer))
	}
	return providers, nil
}

func (session *ortSession) Close() error {
	if session == nil {
		return nil
	}
	if session.session != 0 && session.profileEnabled && !session.profileEnded {
		_ = session.verifyGPUExecution()
	}
	if session.session != 0 && session.functions.releaseSession != nil {
		session.functions.releaseSession(session.session)
		session.session = 0
	}
	if session.memoryInfo != 0 && session.functions.releaseMemoryInfo != nil {
		session.functions.releaseMemoryInfo(session.memoryInfo)
		session.memoryInfo = 0
	}
	if session.env != 0 && session.functions.releaseEnv != nil {
		session.functions.releaseEnv(session.env)
		session.env = 0
	}
	return session.library.close()
}

func findONNXRuntimeLibrary(root string) (string, error) {
	expected := defaultRuntimeLibraryName()
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		expectedLower := strings.ToLower(expected)
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

func onnxProviderName(backend string) string {
	switch backend {
	case "cuda":
		return "CUDAExecutionProvider"
	case "directml":
		return "DmlExecutionProvider"
	case "coreml":
		return "CoreMLExecutionProvider"
	default:
		return ""
	}
}

func containsStringFold(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(value, expected) {
			return true
		}
	}
	return false
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

func validateOutputNorm(vector []float32) bool {
	var norm float64
	for _, value := range vector {
		norm += float64(value) * float64(value)
	}
	return math.Abs(math.Sqrt(norm)-1) <= 0.001
}
