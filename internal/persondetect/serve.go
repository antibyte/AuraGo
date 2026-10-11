package persondetect

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"aurago/internal/onnxrt"
)

// Self-test runs per session on ReferenceImage() (overview C3). The warm-up
// is never measured: the first WebGPU run compiles shaders (2.1–2.7 s on
// Windows/D3D12).
const (
	selfTestWarmups = 1
	selfTestRuns    = 3
)

// runServeCommand runs `serve --runtime <dir> --model <file> --provider
// cpu|webgpu --threads N`: it loads the session (spinning disabled; the
// WebGPU provider for webgpu, where --model is the patched model), plus for
// webgpu a CPU comparison session of the same model with cpuThreads(), runs
// the self-test, releases the comparison session, prints the hello line and
// answers JSON-line requests until close or EOF.
//
// Every line on stdout is JSON. Usage errors and load failures are a hello
// line with "error" set (exit 2 / 1). A Go panic anywhere in the command is
// recovered by RunWorker and printed as an id-less {"type":"error","error":…}
// line (exit 1), possibly after the hello line and some results: clients treat
// an id-less error line as a fatal worker error. A native crash in ONNX
// Runtime ends the process without a line.
func runServeCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("person-detect-worker serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	runtimeRoot := flags.String("runtime", "", "ONNX Runtime directory")
	modelPath := flags.String("model", "", "D-FINE model (the patched model for webgpu)")
	provider := flags.String("provider", "cpu", "cpu | webgpu")
	threads := flags.Int("threads", 1, "intra-op threads")
	if err := flags.Parse(args); err != nil {
		return serveUsage(stdout, err.Error())
	}
	switch {
	case *runtimeRoot == "" || *modelPath == "":
		return serveUsage(stdout, "--runtime and --model are required")
	case *provider != "cpu" && *provider != "webgpu":
		return serveUsage(stdout, "--provider must be cpu or webgpu")
	case *threads < 1 || *threads > 64:
		return serveUsage(stdout, "--threads must be between 1 and 64")
	case flags.NArg() > 0:
		return serveUsage(stdout, fmt.Sprintf("unexpected argument(s) %q", flags.Args()))
	}
	model, err := os.ReadFile(*modelPath)
	if err != nil {
		return serveFailure(stdout, fmt.Errorf("read model: %w", err))
	}
	rt, err := onnxrt.Load(*runtimeRoot, workerLogID)
	if err != nil {
		return serveFailure(stdout, err)
	}
	// Runs after primary.Close (deferred later): Close refuses to unload the
	// library while a session is open.
	defer rt.Close()
	providers, err := rt.Providers()
	if err != nil {
		return serveFailure(stdout, err)
	}
	options := onnxrt.SessionOptions{Threads: *threads, DisableSpinning: true}
	if *provider == "webgpu" {
		options.Provider = onnxrt.ProviderWebGPU
	}
	session, err := rt.NewSession(model, options)
	if err != nil {
		return serveFailure(stdout, err)
	}
	primary := &ortEngine{session: session}
	defer primary.Close()
	var compare engine
	if *provider == "webgpu" {
		// The comparison session measures what the CPU path would cost, so it
		// uses the CPU path's thread count (overview C2).
		cpuSession, err := rt.NewSession(model, onnxrt.SessionOptions{Threads: cpuThreads(), DisableSpinning: true})
		if err != nil {
			return serveFailure(stdout, fmt.Errorf("load CPU comparison session: %w", err))
		}
		compare = &ortEngine{session: cpuSession}
	}
	selfTest := runSelfTest(primary, compare)
	if compare != nil {
		_ = compare.Close()
	}
	hello := helloMessage{Type: "hello", Version: rt.Version(), Providers: providers, Provider: *provider, SelfTest: &selfTest}
	if err := writeJSONLine(stdout, hello); err != nil {
		fmt.Fprintf(stderr, "write worker hello: %v\n", err)
		return 1
	}
	if err := serveLoop(stdin, stdout, primary); err != nil {
		fmt.Fprintf(stderr, "person detection worker: %v\n", err)
		return 1
	}
	return 0
}

func serveUsage(stdout io.Writer, message string) int {
	_ = writeJSONLine(stdout, helloMessage{Type: "hello", Error: message})
	return 2
}

func serveFailure(stdout io.Writer, err error) int {
	_ = writeJSONLine(stdout, helloMessage{Type: "hello", Error: err.Error()})
	return 1
}

// runSelfTest measures the primary engine (and, for WebGPU, the CPU comparison
// engine) on the reference image and decides whether the worker is usable
// (overview C3). Detections are decoded at decodeMinScore. CPU worker
// (compare == nil): at least one person with score ≥ referenceMinScore.
// WebGPU worker: SameConfidentDetections(gpu, cpu, compareMinScore,
// sameMinIoU, sameMaxScoreDiff) and at least one person ≥ referenceMinScore
// found by the CPU session, so two empty sets never pass. ReferencePersons
// counts the persons ≥ referenceMinScore of the reference side (the CPU
// session for WebGPU, the session itself for CPU).
func runSelfTest(primary, compare engine) SelfTest {
	var result SelfTest
	pixels, _, _, err := Preprocess(ReferenceImage(), dfineInputSize)
	if err != nil {
		result.Reason = "preprocess reference image: " + err.Error()
		return result
	}
	primaryBoxes, wallMS, cpuMS, err := measureEngine(primary, pixels)
	if err != nil {
		result.Reason = "inference failed: " + err.Error()
		return result
	}
	referenceBoxes := primaryBoxes
	if compare == nil {
		result.CPUWallMS, result.CPUCPUMS = wallMS, cpuMS
	} else {
		result.GPUWallMS, result.GPUCPUMS = wallMS, cpuMS
		compareBoxes, compareWallMS, compareCPUMS, err := measureEngine(compare, pixels)
		if err != nil {
			result.Reason = "CPU comparison failed: " + err.Error()
			return result
		}
		result.CPUWallMS, result.CPUCPUMS = compareWallMS, compareCPUMS
		referenceBoxes = compareBoxes
	}
	result.ReferencePersons = countAtLeast(referenceBoxes, referenceMinScore)
	if compare != nil {
		if same, reason := SameConfidentDetections(primaryBoxes, referenceBoxes, compareMinScore, sameMinIoU, sameMaxScoreDiff); !same {
			result.Reason = "GPU/CPU detections differ (set a = GPU, set b = CPU): " + reason
			return result
		}
	}
	if result.ReferencePersons < 1 {
		result.Reason = "no person with score >= 0.5 on the reference image"
		return result
	}
	result.Passed = true
	return result
}

// measureEngine runs selfTestWarmups + selfTestRuns inferences and returns the
// detections of the last run (decoded at decodeMinScore) with the wall and
// process CPU milliseconds per measured run. The warm-ups are not measured,
// and the measured runs are timed as one interval divided by selfTestRuns:
// Windows process CPU time advances in ≈15.6 ms ticks, so per-run samples
// would mostly read 0 or one tick.
func measureEngine(eng engine, pixels []float32) ([]Box, float64, float64, error) {
	for i := 0; i < selfTestWarmups; i++ {
		if _, err := detectPixels(eng, pixels, dfineInputSize, decodeMinScore); err != nil {
			return nil, 0, 0, err
		}
	}
	var boxes []Box
	cpuStart, wallStart := processCPUTime(), time.Now()
	for i := 0; i < selfTestRuns; i++ {
		var err error
		boxes, err = detectPixels(eng, pixels, dfineInputSize, decodeMinScore)
		if err != nil {
			return nil, 0, 0, err
		}
	}
	wall, cpu := time.Since(wallStart), processCPUTime()-cpuStart
	return boxes, durationMS(wall) / selfTestRuns, durationMS(cpu) / selfTestRuns, nil
}

func durationMS(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

// serveLoop answers JSON-line requests until close or EOF. It returns an
// error only for an unreadable request stream or a failed write; a bad
// request gets an error result with its id.
func serveLoop(input io.Reader, output io.Writer, eng engine) error {
	decoder := json.NewDecoder(bufio.NewReaderSize(input, 1<<20))
	encoder := json.NewEncoder(output)
	for {
		var request workerRequest
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("decode request: %w", err)
		}
		var response any
		switch request.Command {
		case "detect":
			response = handleDetect(eng, request)
		case "close":
			return encoder.Encode(closeResponse{ID: request.ID, Type: "result"})
		default:
			response = errorResponse{ID: request.ID, Type: "result", Error: "unsupported worker command"}
		}
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("write response: %w", err)
		}
	}
}

// handleDetect answers one detect request. wall_ms and cpu_ms cover decoding,
// preprocessing and inference; on Windows cpu_ms has ≈15.6 ms resolution.
func handleDetect(eng engine, request workerRequest) any {
	if len(request.JPEG) == 0 {
		return errorResponse{ID: request.ID, Type: "result", Error: "jpeg is required"}
	}
	if !(request.MinScore > 0 && request.MinScore <= 1) {
		return errorResponse{ID: request.ID, Type: "result", Error: "min_score must be in (0, 1]"}
	}
	cpuStart, wallStart := processCPUTime(), time.Now()
	pixels, width, height, err := Preprocess(request.JPEG, dfineInputSize)
	if err != nil {
		return errorResponse{ID: request.ID, Type: "result", Error: err.Error()}
	}
	boxes, err := detectPixels(eng, pixels, dfineInputSize, request.MinScore)
	if err != nil {
		return errorResponse{ID: request.ID, Type: "result", Error: "inference failed: " + err.Error()}
	}
	if boxes == nil {
		boxes = []Box{}
	}
	return detectResponse{
		ID:     request.ID,
		Type:   "result",
		Boxes:  boxes,
		Width:  width,
		Height: height,
		WallMS: durationMS(time.Since(wallStart)),
		CPUMS:  durationMS(processCPUTime() - cpuStart),
	}
}
