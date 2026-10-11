package onnxrt

import (
	"fmt"
	"math"
	"os"
	goruntime "runtime"
	"sort"
	"strings"
	"unsafe"
)

// GraphOptimization selects ORT's graph optimization level.
type GraphOptimization int

const (
	OptimizationAll      GraphOptimization = iota // zero value = ORT level 99
	OptimizationBasic                             // ORT level 1 (constant folding)
	OptimizationExtended                          // ORT level 2
	OptimizationDisabled                          // ORT level 0
)

func (g GraphOptimization) ortLevel() (int32, error) {
	switch g {
	case OptimizationAll:
		return 99, nil
	case OptimizationBasic:
		return 1, nil
	case OptimizationExtended:
		return 2, nil
	case OptimizationDisabled:
		return 0, nil
	default:
		return 0, fmt.Errorf("unknown graph optimization %d", int(g))
	}
}

// Execution provider names as listed by Runtime.Providers.
const (
	ProviderCPU    = ""
	ProviderCUDA   = "CUDAExecutionProvider"
	ProviderCoreML = "CoreMLExecutionProvider"
	ProviderWebGPU = "WebGpuExecutionProvider"
)

const allowSpinningConfigKey = "session.intra_op.allow_spinning"

// appendProviderName is the name passed to SessionOptionsAppendExecutionProvider.
// ORT 1.26/1.27 accept the short or the canonical name for WebGPU and CoreML.
// CUDA keeps the canonical name embeddings always passed; ORT's generic append
// does not register CUDA under either spelling.
func appendProviderName(provider string) string {
	if provider == ProviderWebGPU {
		return "WebGPU"
	}
	return provider
}

// SessionOptions configures NewSession.
type SessionOptions struct {
	Threads            int               // intra-op threads; 0 = ORT default
	DisableSpinning    bool              // session config "session.intra_op.allow_spinning" = "0"
	Optimization       GraphOptimization // zero value = OptimizationAll
	OptimizedModelPath string            // non-empty: ORT writes the optimized model there
	Provider           string            // Provider* constant; "" = CPU only
	ProviderOptions    map[string]string // keys/values for SessionOptionsAppendExecutionProvider
	ProfilePrefix      string            // non-empty: EnableProfiling with this prefix
}

// Tensor is a dense tensor; exactly one of Float32 and Int64 is set.
type Tensor struct {
	Shape   []int64
	Float32 []float32
	Int64   []int64
}

// Session is one ORT inference session.
type Session struct {
	runtime      *Runtime
	handle       uintptr
	profiling    bool
	profileEnded bool
}

// NewSession creates a session from an in-memory model.
func (r *Runtime) NewSession(model []byte, opts SessionOptions) (*Session, error) {
	if r.closed() {
		return nil, fmt.Errorf("ONNX Runtime is closed")
	}
	if len(model) == 0 {
		return nil, fmt.Errorf("ONNX model is empty")
	}
	if opts.Threads < 0 {
		return nil, fmt.Errorf("ONNX intra-op threads must not be negative")
	}
	level, err := opts.Optimization.ortLevel()
	if err != nil {
		return nil, err
	}
	f := &r.functions
	var options uintptr
	if err := r.statusError(f.createSessionOptions(&options)); err != nil {
		return nil, fmt.Errorf("create ONNX session options: %w", err)
	}
	defer f.releaseSessionOptions(options)
	if err := r.statusError(f.setSessionGraphOptimizationLevel(options, level)); err != nil {
		return nil, fmt.Errorf("set ONNX graph optimization level: %w", err)
	}
	if opts.Threads > 0 {
		if err := r.statusError(f.setIntraOpNumThreads(options, int32(opts.Threads))); err != nil {
			return nil, fmt.Errorf("set ONNX intra-op threads: %w", err)
		}
	}
	if opts.DisableSpinning {
		key, value := cBytes(allowSpinningConfigKey), cBytes("0")
		status := f.addSessionConfigEntry(options, &key[0], &value[0])
		goruntime.KeepAlive(key)
		goruntime.KeepAlive(value)
		if err := r.statusError(status); err != nil {
			return nil, fmt.Errorf("disable ONNX intra-op spinning: %w", err)
		}
	}
	if opts.OptimizedModelPath != "" {
		pointer, keepAlive := ortPath(opts.OptimizedModelPath)
		status := f.setOptimizedModelFilePath(options, pointer)
		goruntime.KeepAlive(keepAlive)
		if err := r.statusError(status); err != nil {
			return nil, fmt.Errorf("set optimized model path: %w", err)
		}
	}
	if opts.Provider != ProviderCPU {
		if err := r.appendProvider(options, opts.Provider, opts.ProviderOptions); err != nil {
			return nil, err
		}
	}
	if opts.ProfilePrefix != "" {
		pointer, keepAlive := ortPath(opts.ProfilePrefix)
		status := f.enableProfiling(options, pointer)
		goruntime.KeepAlive(keepAlive)
		if err := r.statusError(status); err != nil {
			name := opts.Provider
			if name == ProviderCPU {
				name = "CPUExecutionProvider"
			}
			return nil, fmt.Errorf("enable %s execution profiling: %w", name, err)
		}
	}
	session := &Session{runtime: r, profiling: opts.ProfilePrefix != ""}
	status := f.createSessionFromArray(r.env, unsafe.Pointer(&model[0]), uintptr(len(model)), options, &session.handle)
	goruntime.KeepAlive(model)
	if err := r.statusError(status); err != nil {
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}
	return session, nil
}

func (r *Runtime) appendProvider(options uintptr, provider string, providerOptions map[string]string) error {
	providers, err := r.Providers()
	if err != nil {
		return err
	}
	if !containsFold(providers, provider) {
		return fmt.Errorf("%s is not available in this ONNX Runtime bundle (available: %s)", provider, strings.Join(providers, ", "))
	}
	keys := sortedKeys(providerOptions)
	keyData := make([][]byte, len(keys))
	valueData := make([][]byte, len(keys))
	keyPointers := make([]*byte, len(keys))
	valuePointers := make([]*byte, len(keys))
	for i, key := range keys {
		keyData[i] = cBytes(key)
		valueData[i] = cBytes(providerOptions[key])
		keyPointers[i] = &keyData[i][0]
		valuePointers[i] = &valueData[i][0]
	}
	var keyArray, valueArray **byte
	if len(keys) > 0 {
		keyArray = &keyPointers[0]
		valueArray = &valuePointers[0]
	}
	name := cBytes(appendProviderName(provider))
	status := r.functions.appendExecutionProvider(options, &name[0], keyArray, valueArray, uintptr(len(keys)))
	goruntime.KeepAlive(name)
	goruntime.KeepAlive(keyData)
	goruntime.KeepAlive(valueData)
	goruntime.KeepAlive(keyPointers)
	goruntime.KeepAlive(valuePointers)
	if err := r.statusError(status); err != nil {
		return fmt.Errorf("enable %s: %w", provider, err)
	}
	return nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func validateTensor(name string, tensor Tensor) error {
	hasFloat, hasInt := tensor.Float32 != nil, tensor.Int64 != nil
	if hasFloat == hasInt {
		return fmt.Errorf("input %s must set exactly one of Float32 and Int64", name)
	}
	if len(tensor.Shape) == 0 {
		return fmt.Errorf("input %s has no shape", name)
	}
	count := int64(1)
	for _, dim := range tensor.Shape {
		if dim <= 0 {
			return fmt.Errorf("input %s has invalid dimension %d", name, dim)
		}
		if count > math.MaxInt64/dim {
			return fmt.Errorf("input %s shape %v overflows", name, tensor.Shape)
		}
		count *= dim
	}
	length := len(tensor.Int64)
	if hasFloat {
		length = len(tensor.Float32)
	}
	if int64(length) != count {
		return fmt.Errorf("input %s has %d values for shape %v (want %d)", name, length, tensor.Shape, count)
	}
	return nil
}

// createTensor wraps Go memory without copying; the caller keeps the slices
// alive until the value is released.
func (r *Runtime) createTensor(tensor Tensor) (uintptr, error) {
	var data unsafe.Pointer
	var byteCount uintptr
	elementType := int32(tensorElementInt64)
	if tensor.Float32 != nil {
		data = unsafe.Pointer(&tensor.Float32[0])
		byteCount = uintptr(len(tensor.Float32)) * unsafe.Sizeof(tensor.Float32[0])
		elementType = tensorElementFloat
	} else {
		data = unsafe.Pointer(&tensor.Int64[0])
		byteCount = uintptr(len(tensor.Int64)) * unsafe.Sizeof(tensor.Int64[0])
	}
	var value uintptr
	status := r.functions.createTensorWithData(r.memoryInfo, data, byteCount, &tensor.Shape[0], uintptr(len(tensor.Shape)), elementType, &value)
	goruntime.KeepAlive(tensor.Shape)
	if err := r.statusError(status); err != nil {
		return 0, err
	}
	return value, nil
}

// readTensor copies a float32 or int64 output of any shape into Go memory.
func (r *Runtime) readTensor(value uintptr) (Tensor, error) {
	f := &r.functions
	var shapeInfo uintptr
	if err := r.statusError(f.getTensorTypeAndShape(value, &shapeInfo)); err != nil {
		return Tensor{}, fmt.Errorf("get output shape: %w", err)
	}
	defer f.releaseTensorTypeAndShapeInfo(shapeInfo)
	var elementType int32
	if err := r.statusError(f.getTensorElementType(shapeInfo, &elementType)); err != nil {
		return Tensor{}, fmt.Errorf("get output element type: %w", err)
	}
	var dimensionCount uintptr
	if err := r.statusError(f.getDimensionsCount(shapeInfo, &dimensionCount)); err != nil {
		return Tensor{}, fmt.Errorf("get output dimension count: %w", err)
	}
	shape := make([]int64, dimensionCount)
	if len(shape) > 0 {
		if err := r.statusError(f.getDimensions(shapeInfo, &shape[0], dimensionCount)); err != nil {
			return Tensor{}, fmt.Errorf("get output dimensions: %w", err)
		}
	}
	var elementCount uintptr
	if err := r.statusError(f.getTensorShapeElementCount(shapeInfo, &elementCount)); err != nil {
		return Tensor{}, fmt.Errorf("get output element count: %w", err)
	}
	var data unsafe.Pointer
	if err := r.statusError(f.getTensorMutableData(value, &data)); err != nil {
		return Tensor{}, fmt.Errorf("get output data: %w", err)
	}
	if data == nil && elementCount > 0 {
		return Tensor{}, fmt.Errorf("ONNX output data is nil")
	}
	result := Tensor{Shape: shape}
	switch elementType {
	case tensorElementFloat:
		result.Float32 = make([]float32, elementCount)
		if elementCount > 0 {
			copy(result.Float32, unsafe.Slice((*float32)(data), elementCount))
		}
	case tensorElementInt64:
		result.Int64 = make([]int64, elementCount)
		if elementCount > 0 {
			copy(result.Int64, unsafe.Slice((*int64)(data), elementCount))
		}
	default:
		return Tensor{}, fmt.Errorf("unsupported output element type %d", elementType)
	}
	return result, nil
}

// Run feeds inputs by name and returns the requested outputs copied into Go
// memory.
func (s *Session) Run(inputs map[string]Tensor, outputs []string) (map[string]Tensor, error) {
	if s == nil || s.handle == 0 || s.runtime.closed() {
		return nil, fmt.Errorf("ONNX session is closed")
	}
	if len(inputs) == 0 || len(outputs) == 0 {
		return nil, fmt.Errorf("ONNX run needs at least one input and one output")
	}
	r := s.runtime
	names := make([]string, 0, len(inputs))
	for name := range inputs {
		names = append(names, name)
	}
	sort.Strings(names)
	inputNameData := make([][]byte, len(names))
	inputNames := make([]*byte, len(names))
	inputValues := make([]uintptr, len(names))
	defer func() {
		for _, value := range inputValues {
			if value != 0 {
				r.functions.releaseValue(value)
			}
		}
	}()
	for i, name := range names {
		tensor := inputs[name]
		if err := validateTensor(name, tensor); err != nil {
			return nil, err
		}
		value, err := r.createTensor(tensor)
		if err != nil {
			return nil, fmt.Errorf("create %s tensor: %w", name, err)
		}
		inputValues[i] = value
		inputNameData[i] = cBytes(name)
		inputNames[i] = &inputNameData[i][0]
	}
	outputNameData := make([][]byte, len(outputs))
	outputNames := make([]*byte, len(outputs))
	outputValues := make([]uintptr, len(outputs))
	defer func() {
		for _, value := range outputValues {
			if value != 0 {
				r.functions.releaseValue(value)
			}
		}
	}()
	for i, name := range outputs {
		outputNameData[i] = cBytes(name)
		outputNames[i] = &outputNameData[i][0]
	}
	status := r.functions.run(
		s.handle,
		0,
		&inputNames[0],
		&inputValues[0],
		uintptr(len(inputValues)),
		&outputNames[0],
		uintptr(len(outputNames)),
		&outputValues[0],
	)
	goruntime.KeepAlive(inputs)
	goruntime.KeepAlive(inputNameData)
	goruntime.KeepAlive(outputNameData)
	if err := r.statusError(status); err != nil {
		return nil, err
	}
	result := make(map[string]Tensor, len(outputs))
	for i, name := range outputs {
		if outputValues[i] == 0 {
			return nil, fmt.Errorf("ONNX output %s is missing", name)
		}
		tensor, err := r.readTensor(outputValues[i])
		if err != nil {
			return nil, fmt.Errorf("read output %s: %w", name, err)
		}
		result[name] = tensor
	}
	return result, nil
}

// EndProfiling stops profiling and returns the profile file; the caller
// removes it.
func (s *Session) EndProfiling() (string, error) {
	if s == nil || s.handle == 0 || s.runtime.closed() {
		return "", fmt.Errorf("ONNX session is closed")
	}
	if !s.profiling {
		return "", fmt.Errorf("profiling is not enabled for this session")
	}
	if s.profileEnded {
		return "", fmt.Errorf("profiling already ended")
	}
	s.profileEnded = true
	r := s.runtime
	var allocator uintptr
	if err := r.statusError(r.functions.getAllocatorWithDefaultOptions(&allocator)); err != nil {
		return "", fmt.Errorf("get ONNX allocator: %w", err)
	}
	if allocator == 0 {
		return "", fmt.Errorf("get ONNX allocator: no default allocator")
	}
	var name *byte
	if err := r.statusError(r.functions.sessionEndProfiling(s.handle, allocator, &name)); err != nil {
		return "", fmt.Errorf("end ONNX profiling: %w", err)
	}
	if name == nil {
		return "", fmt.Errorf("end ONNX profiling: no profile file")
	}
	path := cString(name)
	_ = r.statusError(r.functions.allocatorFree(allocator, unsafe.Pointer(name)))
	return path, nil
}

// Close releases the session. A profile that was never ended is ended and
// deleted so profile files cannot accumulate.
func (s *Session) Close() error {
	if s == nil || s.handle == 0 || s.runtime.closed() {
		return nil
	}
	if s.profiling && !s.profileEnded {
		if path, err := s.EndProfiling(); err == nil && path != "" {
			_ = os.Remove(path)
		}
	}
	s.runtime.functions.releaseSession(s.handle)
	s.handle = 0
	return nil
}
