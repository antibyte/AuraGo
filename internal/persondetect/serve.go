package persondetect

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/onnxrt"
)

// Self-test runs per session on ReferenceImage() (overview C3). The warm-up
// is never measured: the first WebGPU run compiles shaders (2.1–2.7 s on
// Windows/D3D12).
const (
	selfTestWarmups = 1
	selfTestRuns    = 3
)

// maxRequestLineBytes bounds one request line: a maxFrameBytes JPEG in base64
// plus the JSON envelope.
var maxRequestLineBytes = base64.StdEncoding.EncodedLen(maxFrameBytes) + 1024

// processCPUClock reads the process CPU time; tests replace it.
var processCPUClock = processCPUTime

var errCPUTimeUnavailable = errors.New("process CPU time unavailable")

// runServeCommand runs `serve --runtime <dir> --model <file> --provider
// cpu|webgpu --threads N`: it loads the session (spinning disabled; the
// WebGPU provider for webgpu, where --model is the patched model), plus for
// webgpu a CPU comparison session of the same model with cpuThreads(), runs
// the self-test (runSelfTest), releases the comparison session, prints the
// hello line and answers JSON-line requests (serveLoop) until close or EOF.
//
// Every line on stdout is JSON. Usage errors and load failures are a hello
// line with "error" set (exit 2 / 1). A Go panic anywhere in the command is
// recovered by RunWorker and printed as an id-less {"type":"error","error":…}
// line (exit 1), possibly after the hello line and some results: clients treat
// an id-less error line as a fatal worker error. A native crash in ONNX
// Runtime ends the process without a line.
//
// In the hello's self_test, gpu_cpu_ms may legitimately be 0 (less than one
// Windows scheduler tick over selfTestRuns runs) and is then omitted
// (omitempty): clients read a missing GPU CPU time as 0.
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
	// Runs after the sessions' deferred closes: Close refuses to unload the
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
	var comparison *ortEngine
	// Deferred as well, so a panic inside the self-test cannot leave the
	// comparison session open (rt.Close would then refuse to unload).
	releaseComparison := func() {
		if comparison != nil {
			_ = comparison.Close()
			comparison = nil
		}
	}
	defer releaseComparison()
	var compare engine
	if *provider == "webgpu" {
		// The comparison session measures what the CPU path would cost, so it
		// uses the CPU path's thread count (overview C2).
		cpuSession, err := rt.NewSession(model, onnxrt.SessionOptions{Threads: cpuThreads(), DisableSpinning: true})
		if err != nil {
			return serveFailure(stdout, fmt.Errorf("load CPU comparison session: %w", err))
		}
		comparison = &ortEngine{session: cpuSession}
		compare = comparison
	}
	selfTest := runSelfTest(primary, compare)
	releaseComparison()
	hello := helloMessage{Type: "hello", Version: rt.Version(), Providers: providers, Provider: *provider, SelfTest: &selfTest}
	if err := writeJSONLine(stdout, hello); err != nil {
		fmt.Fprintf(stderr, "write worker hello: %v\n", err)
		return 1
	}
	if err := serveLoop(stdin, stdout, stderr, primary); err != nil {
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

// measurement is one measureEngine result: the detections of the last run
// and the wall and process CPU milliseconds per measured run.
type measurement struct {
	boxes         []Box
	wallMS, cpuMS float64
}

// runSelfTest measures the primary engine (and, for WebGPU, the CPU comparison
// engine) on the reference image and decides whether the worker is usable
// (overview C3). Detections are decoded at decodeMinScore.
//
// CPU worker (compare == nil): warm-up + measurement; it passes with at least
// one person with score ≥ referenceMinScore.
//
// WebGPU worker: GPU warm-up + measurement, CPU warm-up + measurement, then a
// second GPU measurement without another warm-up. GPUWallMS/GPUCPUMS come from
// the GPU measurement with the lower CPU time (the first one on a tie): on a
// cold start, shader and pipeline work can keep running on background threads
// after the warm-up and inflate the first measurement (a first start on an
// Intel Arc B580 measured exactly 50 % of the CPU path, later starts 5–14 %).
// The comparison uses the first GPU measurement's detections (the runs are
// deterministic). It passes when SameConfidentDetections(gpu, cpu,
// compareMinScore, sameMinIoU, sameMaxScoreDiff) holds and the CPU session
// finds at least one person ≥ referenceMinScore, so two empty sets never
// pass.
//
// ReferencePersons counts the persons ≥ referenceMinScore of the reference
// side (the CPU session for WebGPU, the session itself for CPU). A process
// CPU time that cannot be read fails the self-test.
func runSelfTest(primary, compare engine) SelfTest {
	var result SelfTest
	pixels, _, _, err := Preprocess(ReferenceImage(), dfineInputSize)
	if err != nil {
		result.Reason = "preprocess reference image: " + err.Error()
		return result
	}
	if compare == nil {
		measured, err := measureEngine(primary, pixels, selfTestWarmups)
		if err != nil {
			result.Reason = selfTestFailure("inference failed: ", err)
			return result
		}
		result.CPUWallMS, result.CPUCPUMS = measured.wallMS, measured.cpuMS
		result.ReferencePersons = countAtLeast(measured.boxes, referenceMinScore)
		return referenceVerdict(result)
	}
	gpuFirst, err := measureEngine(primary, pixels, selfTestWarmups)
	if err != nil {
		result.Reason = selfTestFailure("GPU inference failed: ", err)
		return result
	}
	result.GPUWallMS, result.GPUCPUMS = gpuFirst.wallMS, gpuFirst.cpuMS
	cpu, err := measureEngine(compare, pixels, selfTestWarmups)
	if err != nil {
		result.Reason = selfTestFailure("CPU comparison failed: ", err)
		return result
	}
	result.CPUWallMS, result.CPUCPUMS = cpu.wallMS, cpu.cpuMS
	result.ReferencePersons = countAtLeast(cpu.boxes, referenceMinScore)
	gpuSecond, err := measureEngine(primary, pixels, 0)
	if err != nil {
		result.Reason = selfTestFailure("GPU inference failed: ", err)
		return result
	}
	if gpuSecond.cpuMS < gpuFirst.cpuMS {
		result.GPUWallMS, result.GPUCPUMS = gpuSecond.wallMS, gpuSecond.cpuMS
	}
	if same, reason := SameConfidentDetections(gpuFirst.boxes, cpu.boxes, compareMinScore, sameMinIoU, sameMaxScoreDiff); !same {
		result.Reason = "GPU/CPU detections differ (set a = GPU, set b = CPU): " + reason
		return result
	}
	return referenceVerdict(result)
}

// referenceVerdict passes a self-test whose reference side found a person.
func referenceVerdict(result SelfTest) SelfTest {
	if result.ReferencePersons < 1 {
		result.Reason = "no person with score >= 0.5 on the reference image"
		return result
	}
	result.Passed = true
	return result
}

func selfTestFailure(prefix string, err error) string {
	if errors.Is(err, errCPUTimeUnavailable) {
		return errCPUTimeUnavailable.Error()
	}
	return prefix + err.Error()
}

// measureEngine runs warmups unmeasured inferences and then selfTestRuns
// measured ones, timed as one interval divided by selfTestRuns: Windows
// process CPU time advances in ≈15.6 ms ticks, so per-run samples would
// mostly read 0 or one tick. It returns errCPUTimeUnavailable when the
// process CPU time cannot be read or runs backwards.
func measureEngine(eng engine, pixels []float32, warmups int) (measurement, error) {
	for i := 0; i < warmups; i++ {
		if _, err := detectPixels(eng, pixels, dfineInputSize, decodeMinScore); err != nil {
			return measurement{}, err
		}
	}
	cpuStart, ok := processCPUClock()
	if !ok {
		return measurement{}, errCPUTimeUnavailable
	}
	wallStart := time.Now()
	var boxes []Box
	for i := 0; i < selfTestRuns; i++ {
		var err error
		boxes, err = detectPixels(eng, pixels, dfineInputSize, decodeMinScore)
		if err != nil {
			return measurement{}, err
		}
	}
	wall := time.Since(wallStart)
	cpuEnd, ok := processCPUClock()
	if !ok || cpuEnd < cpuStart {
		return measurement{}, errCPUTimeUnavailable
	}
	return measurement{
		boxes:  boxes,
		wallMS: durationMS(wall) / selfTestRuns,
		cpuMS:  durationMS(cpuEnd-cpuStart) / selfTestRuns,
	}, nil
}

func durationMS(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

// serveLoop answers JSON-line requests until close or EOF. Each line may hold
// at most maxRequestLineBytes bytes; blank lines are skipped and a trailing
// "\r" is ignored. A line that is valid JSON with a readable id but a bad
// field (wrong type, invalid base64 jpeg, …) is answered with
// {"id":N,"type":"result","error":"invalid request: …"} (the error text is
// cut to maxRequestErrorBytes) and the loop continues, as for every other bad
// request; a well-formed line without an "id" member is answered with id 0.
// Only an unreadable stream, an over-long line, a line that is not valid
// JSON, a field error in a line without a readable id, and a failed write end
// the loop with an error (the worker exits 1 and reports it on stderr). A
// Go panic while one frame is handled is answered as that frame's error
// (handleDetect); diagnostics receives its stack.
func serveLoop(input io.Reader, output, diagnostics io.Writer, eng engine) error {
	return serveLines(input, output, diagnostics, eng, maxRequestLineBytes)
}

func serveLines(input io.Reader, output, diagnostics io.Writer, eng engine, maxLineBytes int) error {
	reader := bufio.NewReaderSize(input, 64<<10)
	encoder := json.NewEncoder(output)
	for {
		line, err := readRequestLine(reader, maxLineBytes)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		request, failure, err := parseRequest(line)
		if err != nil {
			return err
		}
		var response any
		switch {
		case failure != nil:
			response = *failure
		case request.Command == "detect":
			response = handleDetect(eng, request, diagnostics)
		case request.Command == "close":
			if err := encoder.Encode(closeResponse{ID: request.ID, Type: "result"}); err != nil {
				return fmt.Errorf("write response: %w", err)
			}
			return nil
		default:
			response = errorResponse{ID: request.ID, Type: "result", Error: "unsupported worker command"}
		}
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("write response: %w", err)
		}
	}
}

// readRequestLine returns the next line including its "\n" (a last line
// without one at EOF is returned as is; io.EOF follows). A line whose content
// exceeds maxLineBytes is an error, detected after buffering at most
// maxLineBytes plus one reader buffer.
func readRequestLine(reader *bufio.Reader, maxLineBytes int) ([]byte, error) {
	var line []byte
	for {
		chunk, err := reader.ReadSlice('\n')
		line = append(line, chunk...)
		content := len(line)
		if content > 0 && line[content-1] == '\n' {
			content--
		}
		if content > maxLineBytes {
			return nil, fmt.Errorf("request line exceeds %d bytes", maxLineBytes)
		}
		switch {
		case err == nil:
			return line, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(line) > 0:
			return line, nil
		case errors.Is(err, io.EOF):
			return nil, io.EOF
		default:
			return nil, fmt.Errorf("read request: %w", err)
		}
	}
}

// maxRequestErrorBytes bounds the decoder message in an "invalid request: …"
// response: some messages quote the offending value (an overflowing number
// literal, for example).
const maxRequestErrorBytes = 256

// parseRequest decodes one request line. A field error in a line that is
// valid JSON with a readable id returns an error response for that id; any
// other decoding failure is fatal.
func parseRequest(line []byte) (workerRequest, *errorResponse, error) {
	var request workerRequest
	err := json.Unmarshal(line, &request)
	if err == nil {
		return request, nil, nil
	}
	var head struct {
		ID *uint64 `json:"id"`
	}
	// Unmarshal checks the whole line for valid JSON before decoding.
	if json.Unmarshal(line, &head) != nil || head.ID == nil {
		return workerRequest{}, nil, fmt.Errorf("decode request: %w", err)
	}
	message := "invalid request: " + truncateText(err.Error(), maxRequestErrorBytes)
	return workerRequest{}, &errorResponse{ID: *head.ID, Type: "result", Error: message}, nil
}

// truncateText cuts s to at most limit bytes at a UTF-8 boundary and marks
// the cut with "…".
func truncateText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// handleDetect answers one detect request. wall_ms and cpu_ms cover decoding,
// preprocessing and inference; cpu_ms has ≈15.6 ms resolution on Windows and
// is 0 when the process CPU time cannot be read. A Go panic while the frame
// is handled (a decoder or post-processing bug hit by one frame) is answered
// as the frame's error, "internal error: …", with the stack and then the
// line "panic: … [recovered: request N answered with an error]" on diagnostics;
// the worker keeps serving.
func handleDetect(eng engine, request workerRequest, diagnostics io.Writer) (response any) {
	defer func() {
		value := recover()
		if value == nil {
			return
		}
		// The stack first and the one-line message last: the parent reports a
		// worker's last log line, which must not be a stale stack frame.
		message := strings.Join(strings.Fields(fmt.Sprint(value)), " ")
		fmt.Fprintf(diagnostics, "person detection worker: recovered from a panic while handling request %d:\n%s", request.ID, debug.Stack())
		fmt.Fprintf(diagnostics, "panic: %s [recovered: request %d answered with an error]\n", truncateText(message, maxWorkerErrorBytes), request.ID)
		response = errorResponse{ID: request.ID, Type: "result", Error: truncateText("internal error: "+message, maxRequestErrorBytes)}
	}()
	if len(request.JPEG) == 0 {
		return errorResponse{ID: request.ID, Type: "result", Error: "jpeg is required"}
	}
	if len(request.JPEG) > maxFrameBytes {
		return errorResponse{ID: request.ID, Type: "result", Error: fmt.Sprintf("jpeg has %d bytes, limit %d", len(request.JPEG), maxFrameBytes)}
	}
	if !(request.MinScore > 0 && request.MinScore <= 1) {
		return errorResponse{ID: request.ID, Type: "result", Error: "min_score must be in (0, 1]"}
	}
	cpuStart, cpuStartOK := processCPUClock()
	wallStart := time.Now()
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
	wallMS := durationMS(time.Since(wallStart))
	cpuMS := 0.0
	if cpuEnd, ok := processCPUClock(); ok && cpuStartOK && cpuEnd >= cpuStart {
		cpuMS = durationMS(cpuEnd - cpuStart)
	}
	return detectResponse{
		ID:     request.ID,
		Type:   "result",
		Boxes:  boxes,
		Width:  width,
		Height: height,
		WallMS: wallMS,
		CPUMS:  cpuMS,
	}
}
