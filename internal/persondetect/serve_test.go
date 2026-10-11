package persondetect

import (
	"bytes"
	"encoding/json"
	"errors"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if want := selfTestWarmups + selfTestRuns; gpu.runs != want || cpu.runs != want {
		t.Fatalf("runs gpu=%d cpu=%d, want %d each", gpu.runs, cpu.runs, want)
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
}

func TestProcessCPUTimeAdvances(t *testing.T) {
	start := processCPUTime()
	deadline := time.Now().Add(200 * time.Millisecond)
	value := 0.0
	for time.Now().Before(deadline) {
		value += math.Sqrt(value + 1)
	}
	if processCPUTime() <= start {
		t.Fatalf("process CPU time did not advance (%v, %v)", start, value)
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
