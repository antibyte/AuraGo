package persondetect

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/onnxrt"
)

var (
	twoTestBoxes = [][4]float32{{0.3, 0.5, 0.1, 0.4}, {0.7, 0.5, 0.1, 0.4}}
	oneTestBox   = [][4]float32{{0.3, 0.5, 0.1, 0.4}}
)

func testFrame(t *testing.T) []byte {
	return encodeTestJPEG(t, 64, 48, func(int, int) color.RGBA { return color.RGBA{R: 90, G: 120, B: 150, A: 255} })
}

func runServeLines(t *testing.T, eng engine, requests []workerRequest) []string {
	t.Helper()
	var input bytes.Buffer
	encoder := json.NewEncoder(&input)
	for _, request := range requests {
		if err := encoder.Encode(request); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := serveLoop(&input, &output, eng); err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(output.String()), "\n")
}

func TestServeLoopAnswersDetectUnknownAndClose(t *testing.T) {
	eng := newFakeEngine([]float64{0.9, 0.2}, twoTestBoxes)
	frame := testFrame(t)
	lines := runServeLines(t, eng, []workerRequest{
		{ID: 1, Command: "detect", JPEG: frame, MinScore: 0.5},
		{ID: 2, Command: "explode"},
		{ID: 3, Command: "close"},
		{ID: 4, Command: "detect", JPEG: frame, MinScore: 0.5},
	})
	if len(lines) != 3 {
		t.Fatalf("lines = %q", lines)
	}
	var result workerMessage
	if err := json.Unmarshal([]byte(lines[0]), &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != 1 || result.Type != "result" || result.Error != "" || result.Width != 64 || result.Height != 48 ||
		len(result.Boxes) != 1 || result.WallMS < 0 || result.CPUMS < 0 {
		t.Fatalf("detect result = %+v", result)
	}
	if !strings.Contains(lines[1], `"id":2`) || !strings.Contains(lines[1], `"error":"unsupported worker command"`) {
		t.Fatalf("unknown command line = %s", lines[1])
	}
	if lines[2] != `{"id":3,"type":"result"}` {
		t.Fatalf("close line = %s", lines[2])
	}
	if eng.runs != 1 {
		t.Fatalf("engine runs = %d, want 1 (no request after close)", eng.runs)
	}
}

func TestServeLoopReportsBadRequests(t *testing.T) {
	eng := newFakeEngine([]float64{0.9}, oneTestBox)
	lines := runServeLines(t, eng, []workerRequest{
		{ID: 1, Command: "detect", JPEG: []byte("not a jpeg"), MinScore: 0.5},
		{ID: 2, Command: "detect", JPEG: testFrame(t), MinScore: 0},
		{ID: 3, Command: "detect", MinScore: 0.5},
	})
	want := []string{"decode JPEG", "min_score must be in (0, 1]", "jpeg is required"}
	if len(lines) != len(want) {
		t.Fatalf("lines = %q", lines)
	}
	for i, fragment := range want {
		if !strings.Contains(lines[i], fragment) {
			t.Fatalf("line %d = %s, want %q", i, lines[i], fragment)
		}
	}
	if eng.runs != 0 {
		t.Fatalf("engine ran %d times for invalid requests", eng.runs)
	}
}

func TestServeLoopSendsEmptyBoxArray(t *testing.T) {
	lines := runServeLines(t, newFakeEngine([]float64{0.1}, oneTestBox), []workerRequest{
		{ID: 7, Command: "detect", JPEG: testFrame(t), MinScore: 0.5},
	})
	if len(lines) != 1 || !strings.Contains(lines[0], `"boxes":[]`) {
		t.Fatalf("lines = %q", lines)
	}
}

func TestServeLoopFailsOnMalformedJSON(t *testing.T) {
	if err := serveLoop(strings.NewReader("{not json\n"), io.Discard, newFakeEngine(nil, nil)); err == nil {
		t.Fatal("malformed request accepted")
	}
}

func TestRunSelfTestCPU(t *testing.T) {
	eng := newFakeEngine([]float64{0.9, 0.4}, twoTestBoxes)
	pass := runSelfTest(eng, nil)
	if !pass.Passed || pass.ReferencePersons != 1 || pass.Reason != "" || pass.CPUWallMS < 0 || pass.GPUWallMS != 0 || pass.GPUCPUMS != 0 {
		t.Fatalf("passing self-test = %+v", pass)
	}
	if want := selfTestWarmups + selfTestRuns; eng.runs != want {
		t.Fatalf("runs = %d, want %d", eng.runs, want)
	}
	none := runSelfTest(newFakeEngine([]float64{0.4}, oneTestBox), nil)
	if none.Passed || none.Reason != "no person with score >= 0.5 on the reference image" {
		t.Fatalf("empty self-test = %+v", none)
	}
	broken := runSelfTest(&fakeEngine{err: errors.New("boom")}, nil)
	if broken.Passed || broken.Reason != "inference failed: boom" {
		t.Fatalf("broken self-test = %+v", broken)
	}
}

func TestRunSelfTestWebGPU(t *testing.T) {
	gpu := newFakeEngine([]float64{0.9, 0.6}, twoTestBoxes)
	cpu := newFakeEngine([]float64{0.9, 0.6}, twoTestBoxes)
	result := runSelfTest(gpu, cpu)
	if !result.Passed || result.Reason != "" || result.ReferencePersons != 2 || result.GPUWallMS < 0 || result.CPUWallMS < 0 {
		t.Fatalf("self-test = %+v", result)
	}
	// GPU: warm-up + measurement, then a second measurement after the CPU
	// comparison (no second warm-up); CPU: warm-up + measurement.
	if wantGPU, wantCPU := selfTestWarmups+2*selfTestRuns, selfTestWarmups+selfTestRuns; gpu.runs != wantGPU || cpu.runs != wantCPU {
		t.Fatalf("runs gpu=%d cpu=%d, want %d and %d", gpu.runs, cpu.runs, wantGPU, wantCPU)
	}
	// A confident CPU detection without a GPU partner fails the comparison.
	differing := runSelfTest(newFakeEngine([]float64{0.9}, oneTestBox), newFakeEngine([]float64{0.9, 0.6}, twoTestBoxes))
	if differing.Passed || differing.ReferencePersons != 2 ||
		!strings.HasPrefix(differing.Reason, "GPU/CPU detections differ (set a = GPU, set b = CPU): ") ||
		!strings.Contains(differing.Reason, "has no match in set a") {
		t.Fatalf("differing self-test = %+v", differing)
	}
	// A box right at 0.3 on one side and just below on the other (D-FINE-N
	// scores one reference person at 0.303) stays in the margin band and
	// cannot flip the verdict; an exact count at 0.3 would.
	margin := runSelfTest(
		newFakeEngine([]float64{0.9, 0.303}, twoTestBoxes),
		newFakeEngine([]float64{0.9, 0.297}, twoTestBoxes))
	if !margin.Passed || margin.ReferencePersons != 1 {
		t.Fatalf("margin self-test = %+v", margin)
	}
	// Equal detections without a person >= 0.5 on the CPU side never pass,
	// including two empty sets.
	weak := runSelfTest(newFakeEngine([]float64{0.4}, oneTestBox), newFakeEngine([]float64{0.4}, oneTestBox))
	empty := runSelfTest(newFakeEngine(nil, nil), newFakeEngine(nil, nil))
	for name, test := range map[string]SelfTest{"weak": weak, "empty": empty} {
		if test.Passed || test.ReferencePersons != 0 || test.Reason != "no person with score >= 0.5 on the reference image" {
			t.Fatalf("%s self-test = %+v", name, test)
		}
	}
	// The CPU side is the reference: a GPU-only person >= 0.5 is no match.
	gpuOnly := runSelfTest(newFakeEngine([]float64{0.9}, oneTestBox), newFakeEngine(nil, nil))
	if gpuOnly.Passed || gpuOnly.ReferencePersons != 0 || !strings.Contains(gpuOnly.Reason, "set b is empty") {
		t.Fatalf("GPU-only self-test = %+v", gpuOnly)
	}
	compareBroken := runSelfTest(newFakeEngine([]float64{0.9}, oneTestBox), &fakeEngine{err: errors.New("boom")})
	if compareBroken.Passed || compareBroken.Reason != "CPU comparison failed: boom" || compareBroken.GPUWallMS < 0 {
		t.Fatalf("broken comparison self-test = %+v", compareBroken)
	}
	gpuBroken := runSelfTest(&fakeEngine{err: errors.New("boom")}, newFakeEngine([]float64{0.9}, oneTestBox))
	if gpuBroken.Passed || gpuBroken.Reason != "GPU inference failed: boom" {
		t.Fatalf("broken GPU self-test = %+v", gpuBroken)
	}
}

// flakyEngine fails from its failAt-th run on (1-based).
type flakyEngine struct {
	*fakeEngine
	failAt int
}

func (f *flakyEngine) Run(pixels []float32, size int) (onnxrt.Tensor, onnxrt.Tensor, error) {
	logits, boxes, err := f.fakeEngine.Run(pixels, size)
	if f.runs >= f.failAt {
		return onnxrt.Tensor{}, onnxrt.Tensor{}, errors.New("device lost")
	}
	return logits, boxes, err
}

// scriptCPUClock replaces processCPUClock with readings of the given
// cumulative process CPU times (ok=false once they run out).
func scriptCPUClock(t *testing.T, readings ...time.Duration) {
	t.Helper()
	previous := processCPUClock
	t.Cleanup(func() { processCPUClock = previous })
	processCPUClock = func() (time.Duration, bool) {
		if len(readings) == 0 {
			return 0, false
		}
		value := readings[0]
		readings = readings[1:]
		return value, true
	}
}

func TestRunSelfTestReportsTheLowerGPUMeasurement(t *testing.T) {
	ms := time.Millisecond
	// Readings per measurement (start, end), each over selfTestRuns runs:
	// GPU first 300 ms (100 per run, cold start), CPU 600 ms (200 per run),
	// GPU again 30 ms (10 per run).
	scriptCPUClock(t, 0, 300*ms, 300*ms, 900*ms, 900*ms, 930*ms)
	result := runSelfTest(newFakeEngine([]float64{0.9}, oneTestBox), newFakeEngine([]float64{0.9}, oneTestBox))
	if !result.Passed || result.GPUCPUMS != 10 || result.CPUCPUMS != 200 {
		t.Fatalf("self-test = %+v, want GPU CPU 10 ms and CPU CPU 200 ms", result)
	}
	// A slower second GPU measurement does not replace the first one.
	scriptCPUClock(t, 0, 60*ms, 60*ms, 660*ms, 660*ms, 960*ms)
	result = runSelfTest(newFakeEngine([]float64{0.9}, oneTestBox), newFakeEngine([]float64{0.9}, oneTestBox))
	if !result.Passed || result.GPUCPUMS != 20 || result.CPUCPUMS != 200 {
		t.Fatalf("self-test = %+v, want GPU CPU 20 ms and CPU CPU 200 ms", result)
	}
	// The second GPU measurement failing fails the self-test.
	scriptCPUClock(t, 0, 30*ms, 30*ms, 630*ms, 630*ms, 660*ms)
	gpu := &flakyEngine{fakeEngine: newFakeEngine([]float64{0.9}, oneTestBox), failAt: selfTestWarmups + selfTestRuns + 1}
	result = runSelfTest(gpu, newFakeEngine([]float64{0.9}, oneTestBox))
	if result.Passed || result.Reason != "GPU inference failed: device lost" {
		t.Fatalf("self-test = %+v", result)
	}
}

func TestRunSelfTestNeedsProcessCPUTime(t *testing.T) {
	scriptCPUClock(t)
	if result := runSelfTest(newFakeEngine([]float64{0.9}, oneTestBox), nil); result.Passed || result.Reason != "process CPU time unavailable" {
		t.Fatalf("CPU self-test = %+v", result)
	}
	// A clock running backwards is unavailable too, never a negative time.
	scriptCPUClock(t, 0, 30*time.Millisecond, 600*time.Millisecond, 300*time.Millisecond)
	result := runSelfTest(newFakeEngine([]float64{0.9}, oneTestBox), newFakeEngine([]float64{0.9}, oneTestBox))
	if result.Passed || result.Reason != "process CPU time unavailable" || result.CPUCPUMS < 0 || result.GPUCPUMS < 0 {
		t.Fatalf("WebGPU self-test = %+v", result)
	}
}

func TestProcessCPUTimeAdvances(t *testing.T) {
	start, ok := processCPUTime()
	if !ok {
		t.Fatal("process CPU time unavailable")
	}
	deadline := time.Now().Add(200 * time.Millisecond)
	value := 0.0
	for time.Now().Before(deadline) {
		value += math.Sqrt(value + 1)
	}
	if end, ok := processCPUTime(); !ok || end <= start {
		t.Fatalf("process CPU time did not advance (%v, %v, %v)", start, end, value)
	}
}

func TestServeLoopAnswersFieldErrorsWithID(t *testing.T) {
	eng := newFakeEngine([]float64{0.9}, oneTestBox)
	frame, err := json.Marshal(testFrame(t))
	if err != nil {
		t.Fatal(err)
	}
	input := strings.Join([]string{
		`{"id":5,"command":"detect","jpeg":"!!!!","min_score":0.5}`,
		`{"id":6,"command":"detect","jpeg":` + string(frame) + `,"min_score":"high"}`,
		``,
		`{"id":7,"command":"detect","jpeg":` + string(frame) + `,"min_score":0.5}`,
		`{"id":8,"command":"close"}`,
	}, "\n")
	var output bytes.Buffer
	if err := serveLoop(strings.NewReader(input), &output, eng); err != nil {
		t.Fatal(err)
	}
	lines := decodeLines(t, output.Bytes())
	if len(lines) != 4 {
		t.Fatalf("lines = %v", lines)
	}
	for i, fragment := range []string{"illegal base64 data", "min_score"} {
		message, _ := lines[i]["error"].(string)
		if lines[i]["id"] != float64(5+i) || lines[i]["type"] != "result" || !strings.HasPrefix(message, "invalid request: ") || !strings.Contains(message, fragment) {
			t.Fatalf("line %d = %v, want an error with %q", i, lines[i], fragment)
		}
	}
	if lines[2]["id"] != float64(7) || lines[2]["error"] != nil || lines[3]["id"] != float64(8) || eng.runs != 1 {
		t.Fatalf("lines = %v, engine runs %d", lines, eng.runs)
	}
}

func TestServeLoopFailsOnUnreadableLines(t *testing.T) {
	for name, line := range map[string]string{
		"not json":   `{"id":1,"command":"detect"`,
		"no id":      `{"command":"detect","jpeg":5}`,
		"string id":  `{"id":"1","command":"close"}`,
		"negative":   `{"id":-1,"command":"close"}`,
		"array":      `[1,2,3]`,
		"bare value": `"close"`,
	} {
		var output bytes.Buffer
		err := serveLoop(strings.NewReader(line+"\n"+`{"id":9,"command":"close"}`+"\n"), &output, newFakeEngine(nil, nil))
		if err == nil || output.Len() != 0 {
			t.Errorf("%s: err %v, output %q", name, err, output.String())
		}
	}
}

func TestServeLoopBoundsRequestLines(t *testing.T) {
	if maxFrameBytes != 16<<20 || maxRequestLineBytes != base64.StdEncoding.EncodedLen(maxFrameBytes)+1024 {
		t.Fatalf("limits changed: frame %d, line %d", maxFrameBytes, maxRequestLineBytes)
	}
	closeLine := `{"id":2,"command":"close"}`
	// A line of exactly the limit is read (and answered); one byte more is
	// fatal before it is parsed.
	exact := `{"id":1,"command":"noop","pad":"` + strings.Repeat("x", 40) + `"}`
	limit := len(exact)
	var output bytes.Buffer
	if err := serveLines(strings.NewReader(exact+"\n"+closeLine+"\n"), &output, newFakeEngine(nil, nil), limit); err != nil {
		t.Fatalf("line at the limit: %v", err)
	}
	if lines := decodeLines(t, output.Bytes()); len(lines) != 2 || lines[0]["error"] != "unsupported worker command" {
		t.Fatalf("lines = %v", lines)
	}
	output.Reset()
	long := `{"id":1,"command":"noop","pad":"` + strings.Repeat("x", 41) + `"}`
	err := serveLines(strings.NewReader(long+"\n"+closeLine+"\n"), &output, newFakeEngine(nil, nil), limit)
	if err == nil || !strings.Contains(err.Error(), "exceeds") || output.Len() != 0 {
		t.Fatalf("over-long line: err %v, output %q", err, output.String())
	}
	// Longer than the reader's buffer, without a final newline.
	output.Reset()
	big := `{"id":3,"command":"noop","pad":"` + strings.Repeat("y", 200<<10) + `"}`
	if err := serveLines(strings.NewReader(big), &output, newFakeEngine(nil, nil), len(big)); err != nil || !strings.Contains(output.String(), `"id":3`) {
		t.Fatalf("big last line: err %v, output %q", err, output.String())
	}
}

func TestHandleDetectRejectsOversizedFrame(t *testing.T) {
	eng := newFakeEngine([]float64{0.9}, oneTestBox)
	response := handleDetect(eng, workerRequest{ID: 4, Command: "detect", JPEG: make([]byte, maxFrameBytes+1), MinScore: 0.5})
	failure, ok := response.(errorResponse)
	if !ok || failure.ID != 4 || failure.Error != fmt.Sprintf("jpeg has %d bytes, limit %d", maxFrameBytes+1, maxFrameBytes) || eng.runs != 0 {
		t.Fatalf("response = %+v, engine runs %d", response, eng.runs)
	}
}

func TestServeLoopReportsEngineErrors(t *testing.T) {
	eng := &fakeEngine{err: errors.New("device lost")}
	lines := runServeLines(t, eng, []workerRequest{
		{ID: 1, Command: "detect", JPEG: testFrame(t), MinScore: 0.5},
		{ID: 2, Command: "close"},
	})
	if len(lines) != 2 || lines[0] != `{"id":1,"type":"result","error":"inference failed: device lost"}` || lines[1] != `{"id":2,"type":"result"}` {
		t.Fatalf("lines = %q", lines)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("pipe closed") }

func TestServeLoopFailsOnWriteError(t *testing.T) {
	for _, input := range []string{
		`{"id":1,"command":"noop"}` + "\n",
		`{"id":1,"command":"close"}` + "\n",
	} {
		if err := serveLoop(strings.NewReader(input), failingWriter{}, newFakeEngine(nil, nil)); err == nil || !strings.Contains(err.Error(), "pipe closed") {
			t.Fatalf("%s: err = %v", strings.TrimSpace(input), err)
		}
	}
}

func TestRunWorkerServeValidatesFlags(t *testing.T) {
	cases := map[string]struct {
		args    []string
		message string
	}{
		"missing":  {[]string{"serve"}, "--runtime and --model are required"},
		"provider": {[]string{"serve", "--runtime", "r", "--model", "m", "--provider", "cuda"}, "--provider must be cpu or webgpu"},
		"threads":  {[]string{"serve", "--runtime", "r", "--model", "m", "--threads", "0"}, "--threads must be between 1 and 64"},
		"extra":    {[]string{"serve", "--runtime", "r", "--model", "m", "extra"}, `unexpected argument(s) ["extra"]`},
	}
	for name, tc := range cases {
		var stdout, stderr bytes.Buffer
		code := RunWorker(tc.args, strings.NewReader(""), &stdout, &stderr)
		lines := decodeLines(t, stdout.Bytes())
		if code != 2 || len(lines) != 1 || lines[0]["type"] != "hello" || lines[0]["error"] != tc.message {
			t.Errorf("%s: code %d, lines %v", name, code, lines)
		}
	}
}

func TestServeReportsLoadErrorInHello(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.onnx")
	if err := os.WriteFile(model, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := RunWorker([]string{"serve", "--runtime", runtimeDir, "--model", model}, strings.NewReader(""), &stdout, &stderr)
	lines := decodeLines(t, stdout.Bytes())
	if code != 1 || len(lines) != 1 || lines[0]["type"] != "hello" || !strings.Contains(lines[0]["error"].(string), "not found below") {
		t.Fatalf("code %d, lines %v", code, lines)
	}
}
