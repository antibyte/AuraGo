package persondetect

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aurago/internal/onnxrt"
)

func decodeLines(t *testing.T, output []byte) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var message map[string]any
		if err := json.Unmarshal(line, &message); err != nil {
			t.Fatalf("worker line %q: %v", line, err)
		}
		lines = append(lines, message)
	}
	return lines
}

func TestRunWorkerRequiresKnownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := RunWorker(nil, strings.NewReader(""), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "usage: aurago --person-detect-worker") {
		t.Fatalf("no subcommand: code %d, stderr %q", code, stderr.String())
	}
	stderr.Reset()
	if code := RunWorker([]string{"bogus"}, strings.NewReader(""), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), `unknown person detection worker command "bogus"`) {
		t.Fatalf("bogus subcommand: code %d, stderr %q", code, stderr.String())
	}
}

func TestPrepareRequiresFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWorker([]string{"prepare", "--in", "model.onnx"}, strings.NewReader(""), &stdout, &stderr)
	lines := decodeLines(t, stdout.Bytes())
	if code != 2 || len(lines) != 1 || lines[0]["type"] != "error" || lines[0]["error"] != "--runtime, --in and --out are required" {
		t.Fatalf("code %d, lines %v", code, lines)
	}
}

func TestPrepareReportsMissingModel(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := RunWorker([]string{"prepare", "--runtime", dir, "--in", filepath.Join(dir, "missing.onnx"), "--out", filepath.Join(dir, "out.onnx")}, strings.NewReader(""), &stdout, &stderr)
	lines := decodeLines(t, stdout.Bytes())
	if code != 1 || len(lines) != 1 || lines[0]["type"] != "error" || !strings.Contains(lines[0]["error"].(string), "read model") {
		t.Fatalf("code %d, lines %v", code, lines)
	}
}

func TestPrepareReportsMissingRuntime(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.onnx")
	if err := os.WriteFile(model, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "out.onnx")
	var stdout, stderr bytes.Buffer
	code := RunWorker([]string{"prepare", "--runtime", runtimeDir, "--in", model, "--out", output}, strings.NewReader(""), &stdout, &stderr)
	lines := decodeLines(t, stdout.Bytes())
	if code != 1 || len(lines) != 1 || !strings.Contains(lines[0]["error"].(string), "not found below") {
		t.Fatalf("code %d, lines %v", code, lines)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output written despite failure: %v", err)
	}
}

type fakeEngine struct {
	logits, boxes onnxrt.Tensor
	err           error
	runs          int
	closed        bool
}

func (f *fakeEngine) Run([]float32, int) (onnxrt.Tensor, onnxrt.Tensor, error) {
	f.runs++
	return f.logits, f.boxes, f.err
}

func (f *fakeEngine) Close() error {
	f.closed = true
	return nil
}

// newFakeEngine returns D-FINE-shaped outputs (logits [1,300,80], pred_boxes
// [1,300,4]): the first queries carry the given person probabilities and
// cx/cy/w/h boxes, every other query and class scores 0.001 with an empty box.
func newFakeEngine(scores []float64, boxes [][4]float32) *fakeEngine {
	logits := make([]float32, dfineQueries*dfineClasses)
	for i := range logits {
		logits[i] = logit(0.001)
	}
	flat := make([]float32, dfineQueries*4)
	for i, score := range scores {
		logits[i*dfineClasses+dfinePersonClass] = logit(score)
		copy(flat[i*4:], boxes[i][:])
	}
	return &fakeEngine{
		logits: onnxrt.Tensor{Shape: []int64{1, dfineQueries, dfineClasses}, Float32: logits},
		boxes:  onnxrt.Tensor{Shape: []int64{1, dfineQueries, 4}, Float32: flat},
	}
}

func TestDetectPixelsDecodesEngineOutputs(t *testing.T) {
	eng := newFakeEngine([]float64{0.9, 0.2}, [][4]float32{{0.5, 0.5, 0.2, 0.4}, {0.2, 0.2, 0.1, 0.1}})
	boxes, err := detectPixels(eng, nil, dfineInputSize, 0.5)
	if err != nil || len(boxes) != 1 || math.Abs(boxes[0].Score-0.9) > 1e-4 {
		t.Fatalf("detectPixels = %+v, %v", boxes, err)
	}
}

func TestOutputLayoutRejectsBadShapes(t *testing.T) {
	good := newFakeEngine([]float64{0.9}, [][4]float32{{0.5, 0.5, 0.2, 0.2}})
	if queries, classes, err := outputLayout(good.logits, good.boxes); err != nil || queries != dfineQueries || classes != dfineClasses {
		t.Fatalf("outputLayout = %d, %d, %v", queries, classes, err)
	}
	zeros := func(shape ...int64) onnxrt.Tensor {
		return onnxrt.Tensor{Shape: shape, Float32: make([]float32, shape[0]*shape[1]*shape[2])}
	}
	cases := map[string][2]onnxrt.Tensor{
		"int64 logits":    {{Shape: []int64{1, dfineQueries, dfineClasses}, Int64: make([]int64, dfineQueries*dfineClasses)}, good.boxes},
		"rank":            {{Shape: []int64{dfineQueries, dfineClasses}, Float32: make([]float32, dfineQueries*dfineClasses)}, good.boxes},
		"box width":       {good.logits, zeros(1, dfineQueries, 3)},
		"query mismatch":  {good.logits, zeros(1, dfineQueries-1, 4)},
		"length":          {{Shape: []int64{1, dfineQueries, dfineClasses}, Float32: make([]float32, dfineQueries*dfineClasses-1)}, good.boxes},
		"box length":      {good.logits, {Shape: []int64{1, dfineQueries, 4}, Float32: make([]float32, dfineQueries*4+1)}},
		"batch 2":         {zeros(2, dfineQueries, dfineClasses), zeros(2, dfineQueries, 4)},
		"zero queries":    {{Shape: []int64{1, 0, dfineClasses}, Float32: []float32{}}, {Shape: []int64{1, 0, 4}, Float32: []float32{}}},
		"zero classes":    {{Shape: []int64{1, dfineQueries, 0}, Float32: []float32{}}, good.boxes},
		"wrong queries":   {zeros(1, 100, dfineClasses), zeros(1, 100, 4)},
		"wrong classes":   {zeros(1, dfineQueries, 2), good.boxes},
		"negative shapes": {{Shape: []int64{1, -dfineQueries, dfineClasses}, Float32: []float32{}}, good.boxes},
	}
	for name, pair := range cases {
		if _, _, err := outputLayout(pair[0], pair[1]); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestCPUThreadsStayInRange(t *testing.T) {
	if threads := cpuThreads(); threads < 1 || threads > 4 {
		t.Fatalf("cpuThreads() = %d", threads)
	}
}

func TestWriteFileAtomicReplacesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "model.onnx")
	for _, content := range []string{"first", "second"} {
		if err := writeFileAtomic(path, []byte(content), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "second" {
		t.Fatalf("content = %q, %v", raw, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("directory holds %d entries (temp files left?): %v", len(entries), err)
	}
}

// PatchVectorMatMuls refuses unsafe rewrites by returning the model unchanged
// with count 0; prepare must turn that into an error, never write the model.
func TestPatchFoldedModelRequiresPatchedNode(t *testing.T) {
	nodes, initializers := integralChain("integral", "w", "Reshape", false)
	patched, count, err := patchFoldedModel(testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil))
	if err != nil || count != 1 || len(patched) == 0 {
		t.Fatalf("patchable model: %d node(s), %d bytes, %v", count, len(patched), err)
	}
	nodes, initializers = integralChain("integral", "w", "Add", false)
	if _, count, err := patchFoldedModel(testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)); err == nil || count != 0 || !strings.Contains(err.Error(), "no patchable MatMul") {
		t.Fatalf("refused model: %d node(s), %v", count, err)
	}
	if _, _, err := patchFoldedModel([]byte{0xff}); err == nil || !strings.Contains(err.Error(), "patch folded model") {
		t.Fatalf("malformed model: %v", err)
	}
}

// compareReferenceDetections is prepare's acceptance rule (overview C3): the
// sets are decoded at 0.28 and compared with SameConfidentDetections(original,
// patched, 0.3, 0.9, 0.02), and the original model must find a person ≥ 0.5.
func TestCompareReferenceDetections(t *testing.T) {
	strong := Box{X1: 0.1, Y1: 0.1, X2: 0.3, Y2: 0.6, Score: 0.88}
	borderline := Box{X1: 0.6, Y1: 0.2, X2: 0.7, Y2: 0.5, Score: 0.303}
	weak := Box{X1: 0.4, Y1: 0.2, X2: 0.5, Y2: 0.5, Score: 0.42}

	// A box right at 0.3 may fall below it in the other set without failing.
	shifted := borderline
	shifted.Score = 0.297
	summary, err := compareReferenceDetections([]Box{strong, borderline}, []Box{strong, shifted})
	if err != nil || !strings.Contains(summary, "matched 1 confident detection") || !strings.Contains(summary, "1 person(s) >= 0.5") {
		t.Fatalf("matching sets: %q, %v", summary, err)
	}

	moved := strong
	moved.X2 = 0.5
	if _, err := compareReferenceDetections([]Box{strong}, []Box{moved}); err == nil || !strings.Contains(err.Error(), "patched model changes the reference detections") {
		t.Fatalf("moved box accepted: %v", err)
	}
	rescored := strong
	rescored.Score = 0.85
	if _, err := compareReferenceDetections([]Box{strong}, []Box{rescored}); err == nil || !strings.Contains(err.Error(), "patched model changes the reference detections") {
		t.Fatalf("score drift accepted: %v", err)
	}
	if _, err := compareReferenceDetections([]Box{weak}, []Box{weak}); err == nil || !strings.Contains(err.Error(), "no person with score >= 0.5") {
		t.Fatalf("weak reference accepted: %v", err)
	}
	if _, err := compareReferenceDetections(nil, nil); err == nil || !strings.Contains(err.Error(), "no person with score >= 0.5") {
		t.Fatalf("two empty sets accepted: %v", err)
	}
}

func TestWriteFileAtomicLeavesNoTempOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.onnx")
	// A non-empty directory at the target path makes the final rename fail
	// on every platform.
	if err := os.MkdirAll(filepath.Join(path, "keep"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("model"), 0o640); err == nil || !strings.Contains(err.Error(), "activate model.onnx") {
		t.Fatalf("writeFileAtomic over a directory: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "model.onnx" || !entries[0].IsDir() {
		t.Fatalf("directory after the failed write: %v, %v", entries, err)
	}
}

// A killed prepare leaves <out>.folded-<hex>.onnx and <out>.tmp-<hex> next to
// the output; the next prepare of the same output removes exactly those.
func TestSweepStalePrepareFiles(t *testing.T) {
	// "[" would be a pattern character for filepath.Glob.
	dir := filepath.Join(t.TempDir(), "data [cam]", "standard")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "model.webgpu-1.27.0.onnx")
	stale := []string{
		"model.webgpu-1.27.0.onnx.folded-0123456789abcdef.onnx",
		"model.webgpu-1.27.0.onnx.tmp-0123456789abcdef",
	}
	kept := []string{
		"model.onnx",
		"model.onnx.tmp-0123456789abcdef",
		"model.webgpu-1.27.0.onnx",
		"model.webgpu-1.27.0.json",
		"model.webgpu-1.27.0.json.tmp-0123456789abcdef",
		"model.webgpu-1.27.0.onnx.part",
	}
	for _, name := range append(append([]string(nil), stale...), kept...) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "model.webgpu-1.27.0.onnx.tmp-dir"), 0o750); err != nil {
		t.Fatal(err)
	}
	kept = append(kept, "model.webgpu-1.27.0.onnx.tmp-dir")

	sweepStalePrepareFiles(output)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	slices.Sort(kept)
	if !slices.Equal(names, kept) {
		t.Fatalf("after the sweep:\n got %q\nwant %q", names, kept)
	}
	// A missing directory is not an error.
	sweepStalePrepareFiles(filepath.Join(dir, "missing", "model.onnx"))
}

func TestPrepareRejectsExtraArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWorker([]string{"prepare", "--runtime", "rt", "--in", "in.onnx", "--out", "out.onnx", "extra"}, strings.NewReader(""), &stdout, &stderr)
	lines := decodeLines(t, stdout.Bytes())
	if code != 2 || len(lines) != 1 || lines[0]["type"] != "error" || !strings.Contains(lines[0]["error"].(string), `unexpected argument(s) ["extra"]`) {
		t.Fatalf("code %d, lines %v", code, lines)
	}
}

func TestPrepareRejectsSameInputAndOutput(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.onnx")
	if err := os.WriteFile(model, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	aliases := []string{model, filepath.Join(dir, ".", "sub", "..", "model.onnx")}
	if runtime.GOOS == "windows" {
		aliases = append(aliases, filepath.Join(dir, "MODEL.ONNX"))
	}
	for _, alias := range aliases {
		var stdout, stderr bytes.Buffer
		code := RunWorker([]string{"prepare", "--runtime", dir, "--in", model, "--out", alias}, strings.NewReader(""), &stdout, &stderr)
		lines := decodeLines(t, stdout.Bytes())
		if code != 2 || len(lines) != 1 || lines[0]["type"] != "error" || lines[0]["error"] != "--in and --out must name different files" {
			t.Fatalf("--out %s: code %d, lines %v", alias, code, lines)
		}
	}
	raw, err := os.ReadFile(model)
	if err != nil || string(raw) != "model" {
		t.Fatalf("input changed: %q, %v", raw, err)
	}
}

// A panic inside a worker command is an error line with exit code 1; exit
// code 2 keeps meaning "usage error".
func TestRecoverWorkerPanic(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := func() (code int) {
		defer recoverWorkerPanic(&stdout, &stderr, &code)
		panic("boom")
	}()
	lines := decodeLines(t, stdout.Bytes())
	if code != 1 || len(lines) != 1 || lines[0]["type"] != "error" || lines[0]["error"] != "person detection worker panic: boom" {
		t.Fatalf("code %d, lines %v", code, lines)
	}
	if !strings.Contains(stderr.String(), "goroutine") {
		t.Fatalf("stderr has no stack: %q", stderr.String())
	}
	stdout.Reset()
	code = func() (code int) {
		defer recoverWorkerPanic(&stdout, &stderr, &code)
		return 0
	}()
	if code != 0 || stdout.Len() != 0 {
		t.Fatalf("no panic: code %d, stdout %q", code, stdout.String())
	}
}
