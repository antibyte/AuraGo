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
//   - native_crash: every detect prints syntheticNativeCrash on stderr and
//     exits 2, like the Go runtime after an access violation in native code
//   - recover_then_exit: the first detect panics inside the real handleDetect
//     (answered as a frame error), the second one exits 0
//   - exit_after_answer: exits 0 right after answering a detect
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

// syntheticNativeCrash mirrors what the Go runtime prints on Windows when
// native code (ONNX Runtime) hits an access violation: a log line, the
// "Exception 0x…" header, ≈5 KB of goroutine traces and a register dump that
// ends in "gs 0x2b". The header is outside the last 4 KiB.
func syntheticNativeCrash() string {
	var text strings.Builder
	text.WriteString("[W:onnxruntime:aurago-persondetect] Some nodes were not assigned to the preferred execution providers\n")
	text.WriteString("Exception 0xc0000005 0x8 0x10 0x10\nPC=0x10\n\n")
	text.WriteString("runtime.cgocall(0x7ff6d864e980, 0x7ff6d87ce0d0)\n\tC:/Program Files/Go/src/runtime/cgocall.go:167 +0x3e fp=0x7d954455e18 sp=0x7d954455db8 pc=0x7ff6d86b6d7e\n")
	for i := 2; i < 40; i++ {
		fmt.Fprintf(&text, "\ngoroutine %d gp=0x7d95444e780 m=nil [GC sweep wait]:\nruntime.gopark(0x0?, 0x0?, 0x0?, 0x0?, 0x0?)\n\tC:/Program Files/Go/src/runtime/proc.go:474 +0xca fp=0x7d954451fa8 sp=0x7d954451f88 pc=0x7ff6d86b88ea\n", i)
	}
	for _, register := range []string{"rax", "rbx", "rcx", "rdx", "rdi", "rsi", "rbp", "rsp", "r8", "r9", "r10", "r11", "r12", "r13", "r14", "r15", "rip", "rflags", "cs", "fs"} {
		fmt.Fprintf(&text, "%-8s0x10\n", register)
	}
	text.WriteString("gs      0x2b\n")
	return text.String()
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
				if settings.has("native_crash") {
					fmt.Fprint(stderr, syntheticNativeCrash())
					return 2
				}
				if settings.has("recover_then_exit") {
					if detects > 1 {
						return 0
					}
					eng := &panickingEngine{inner: newFakeEngine([]float64{0.9}, [][4]float32{{0.3, 0.5, 0.1, 0.4}})}
					_ = writeJSONLine(stdout, handleDetect(eng, request, stderr))
					continue
				}
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
				if settings.has("exit_after_answer") {
					return 0
				}
			case "close":
				_ = writeJSONLine(stdout, closeResponse{ID: request.ID, Type: "result"})
				return 0
			}
		}
	}
	return 2
}
