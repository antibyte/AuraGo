package persondetect

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	fakeModeEnv = "AURAGO_PDFAKE_MODE"
	fakeDirEnv  = "AURAGO_PDFAKE_DIR"
)

// TestMain lets the test binary act as the worker: detector tests point
// DetectorOptions.Executable at os.Executable(). With AURAGO_PDFAKE_MODE set
// it is a scripted fake, otherwise the real worker (integration tests).
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == workerFlag {
		if mode, ok := os.LookupEnv(fakeModeEnv); ok {
			os.Exit(runFakeWorker(mode, os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
		}
		os.Exit(RunWorker(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// fakeSettings holds comma-separated tokens of AURAGO_PDFAKE_MODE:
//
//   - self-test: gpu_cpu_ms=N, gpu_cpu_ms_restart=N (GPU CPU time of every
//     start after the first), cpu_cpu_ms=N, gpu_selftest_fail,
//     cpu_selftest_fail
//   - prepare: prepare_fail, prepare_sleep_ms=N, prepare_hang
//   - start: panic_start, hello_hang, restart_hang (every start after the
//     first hangs before its hello), exit_after_hello (first start), no_read
//     (the first start never reads stdin), ignore_close (never reads stdin,
//     never exits by itself), noise, stderr_flood (100 stderr lines and a
//     1 KB last line before the hello)
//   - detect: crash_detect, crash_first_detect (first start only),
//     crash_second_detect (per start), hang_detect, panic_detect, frame_error,
//     detect_sleep_ms=N, echo_id (answers with width = request id and height =
//     frame size)
//
// Every detect request read is recorded as a "detect" call.
type fakeSettings map[string]string

func parseFakeMode(mode string) fakeSettings {
	settings := fakeSettings{}
	for _, token := range strings.Split(mode, ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(token), "=")
		if key != "" {
			settings[key] = value
		}
	}
	return settings
}

func (s fakeSettings) has(key string) bool {
	_, ok := s[key]
	return ok
}

func (s fakeSettings) float(key string, fallback float64) float64 {
	if value, err := strconv.ParseFloat(s[key], 64); err == nil {
		return value
	}
	return fallback
}

// recordFakeCall appends to <AURAGO_PDFAKE_DIR>/<name>.log and returns the
// number of calls recorded so far (fake workers are separate processes).
func recordFakeCall(name string) int {
	dir := os.Getenv(fakeDirEnv)
	if dir == "" {
		return 0
	}
	file, err := os.OpenFile(filepath.Join(dir, name+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		_, _ = file.WriteString("call\n")
		_ = file.Close()
	}
	return fakeCallCount(dir, name)
}

func fakeCallCount(dir, name string) int {
	raw, err := os.ReadFile(filepath.Join(dir, name+".log"))
	if err != nil {
		return 0
	}
	return strings.Count(string(raw), "\n")
}

// fakeNoise writes what native libraries print around the protocol: an ONNX
// Runtime warning on stderr, a banner and an unknown JSON line on stdout.
func fakeNoise(stdout, stderr io.Writer) {
	fmt.Fprintln(stderr, "[W:onnxruntime:aurago-persondetect] Some nodes were not assigned to the preferred execution providers which may or may not have an negative impact on performance.")
	fmt.Fprintln(stdout, "native library banner: not JSON")
	fmt.Fprintln(stdout, `{"type":"log","message":"unknown line type"}`)
}

func runFakeWorker(mode string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	settings := parseFakeMode(mode)
	if len(args) == 0 {
		return 2
	}
	flags := flag.NewFlagSet("fake-worker", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.String("runtime", "", "")
	flags.String("in", "", "")
	out := flags.String("out", "", "")
	flags.String("model", "", "")
	provider := flags.String("provider", "cpu", "")
	flags.Int("threads", 1, "")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	switch args[0] {
	case "prepare":
		recordFakeCall("prepare")
		if settings.has("prepare_hang") {
			time.Sleep(time.Hour)
		}
		if settings.has("prepare_sleep_ms") {
			time.Sleep(time.Duration(settings.float("prepare_sleep_ms", 0)) * time.Millisecond)
		}
		if settings.has("prepare_fail") {
			_ = writeJSONLine(stdout, preparedMessage{Type: "error", Error: "fake prepare failure"})
			return 1
		}
		if err := os.WriteFile(*out, []byte("patched model"), 0o600); err != nil {
			return 1
		}
		_ = writeJSONLine(stdout, preparedMessage{Type: "prepared", PatchedNodes: 1})
		return 0
	case "serve":
		starts := recordFakeCall("serve-" + *provider)
		if settings.has("panic_start") {
			// What RunWorker's recover prints for a Go panic.
			_ = writeJSONLine(stdout, preparedMessage{Type: "error", Error: "person detection worker panic: fake panic"})
			return 1
		}
		if settings.has("noise") {
			fakeNoise(stdout, stderr)
		}
		if settings.has("stderr_flood") {
			for i := 0; i < 100; i++ {
				fmt.Fprintf(stderr, "flood line %d\n", i)
			}
			fmt.Fprintln(stderr, "last line "+strings.Repeat("x", 1000))
		}
		if settings.has("hello_hang") || (settings.has("restart_hang") && starts > 1) {
			time.Sleep(time.Hour)
		}
		selfTest := SelfTest{Passed: true, ReferencePersons: 4, CPUWallMS: 120, CPUCPUMS: settings.float("cpu_cpu_ms", 100)}
		if *provider == "webgpu" {
			selfTest.GPUWallMS = 100
			selfTest.GPUCPUMS = settings.float("gpu_cpu_ms", 10)
			if starts > 1 && settings.has("gpu_cpu_ms_restart") {
				selfTest.GPUCPUMS = settings.float("gpu_cpu_ms_restart", 10)
			}
			if settings.has("gpu_selftest_fail") {
				selfTest.Passed = false
				selfTest.Reason = "GPU/CPU detections differ: count 3 vs 4"
			}
		} else if settings.has("cpu_selftest_fail") {
			selfTest.Passed = false
			selfTest.Reason = "no person with score >= 0.5 on the reference image"
		}
		_ = writeJSONLine(stdout, helloMessage{
			Type: "hello", Version: "1.27.0", Providers: []string{"CPUExecutionProvider"}, Provider: *provider, SelfTest: &selfTest,
		})
		if settings.has("exit_after_hello") && starts == 1 {
			return 0
		}
		if (settings.has("no_read") && starts == 1) || settings.has("ignore_close") {
			time.Sleep(time.Hour)
		}
		decoder := json.NewDecoder(stdin)
		detects := 0
		for {
			var request workerRequest
			if err := decoder.Decode(&request); err != nil {
				return 0
			}
			switch request.Command {
			case "detect":
				detects++
				recordFakeCall("detect")
				if settings.has("detect_sleep_ms") {
					time.Sleep(time.Duration(settings.float("detect_sleep_ms", 0)) * time.Millisecond)
				}
				if settings.has("crash_detect") || (settings.has("crash_first_detect") && starts == 1) ||
					(settings.has("crash_second_detect") && detects == 2) {
					return 3
				}
				if settings.has("hang_detect") {
					time.Sleep(time.Hour)
				}
				if settings.has("panic_detect") {
					_ = writeJSONLine(stdout, preparedMessage{Type: "error", Error: "person detection worker panic: fake panic"})
					return 1
				}
				if settings.has("noise") {
					fakeNoise(stdout, stderr)
				}
				if settings.has("frame_error") {
					_ = writeJSONLine(stdout, errorResponse{ID: request.ID, Type: "result", Error: "decode JPEG header: fake frame error"})
					continue
				}
				if settings.has("echo_id") {
					_ = writeJSONLine(stdout, detectResponse{ID: request.ID, Type: "result", Boxes: []Box{}, Width: int(request.ID), Height: len(request.JPEG)})
					continue
				}
				_ = writeJSONLine(stdout, detectResponse{
					ID: request.ID, Type: "result", Boxes: []Box{{X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.4, Score: 0.9}},
					Width: 64, Height: 48, WallMS: 5, CPUMS: 4,
				})
			case "close":
				_ = writeJSONLine(stdout, closeResponse{ID: request.ID, Type: "result"})
				return 0
			}
		}
	}
	return 2
}
