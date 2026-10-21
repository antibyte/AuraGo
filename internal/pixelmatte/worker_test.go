package pixelmatte

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

const fakeWorkerEnv = "AURAGO_PMFAKE_MODE"

// TestMain lets the test binary act as the worker: runner tests point
// RunRequest.Executable at os.Executable(). With AURAGO_PMFAKE_MODE set it is
// a scripted fake, otherwise the real worker (usage errors only here).
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == WorkerFlag {
		if mode, ok := os.LookupEnv(fakeWorkerEnv); ok {
			os.Exit(runFakeWorker(mode, os.Stdin, os.Stdout, os.Stderr))
		}
		os.Exit(RunWorker(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

func runFakeWorker(mode string, stdin io.Reader, stdout, stderr io.Writer) int {
	input, _ := io.ReadAll(stdin)
	switch mode {
	case "echo":
		_, _ = stdout.Write(input)
		return 0
	case "fail":
		_, _ = io.WriteString(stderr, "first line\nmodel exploded: bad day\n")
		return 1
	case "hang":
		time.Sleep(time.Minute)
		return 0
	case "flood":
		chunk := make([]byte, 1<<20)
		for i := 0; i < maxMatteBytes>>20+4; i++ {
			if _, err := stdout.Write(chunk); err != nil {
				time.Sleep(time.Minute)
				return 1
			}
		}
		return 0
	case "empty":
		return 0
	}
	return 3
}

func TestRunWorkerUsageErrorsExitTwo(t *testing.T) {
	cases := map[string][]string{
		"no arguments":    nil,
		"unknown command": {"serve"},
		"no flags":        {"run"},
		"no model":        {"run", "--runtime", "rt", "--kind", "standard"},
		"bad kind":        {"run", "--runtime", "rt", "--model", "m.onnx", "--kind", "huge"},
		"no kind":         {"run", "--runtime", "rt", "--model", "m.onnx"},
		"zero threads":    {"run", "--runtime", "rt", "--model", "m.onnx", "--kind", "standard", "--threads", "0"},
		"unknown flag":    {"run", "--runtime", "rt", "--model", "m.onnx", "--kind", "standard", "--gpu"},
		"extra argument":  {"run", "--runtime", "rt", "--model", "m.onnx", "--kind", "standard", "extra"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWorker(args, strings.NewReader("never read"), &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("code %d, stdout %d bytes, stderr %q; want 2, none, a message", code, stdout.Len(), stderr.String())
			}
		})
	}
}

type fakeEngine struct {
	pixels []float32
	output []float32
	shape  []int64
	err    error
	panics bool
	closed bool
}

func (f *fakeEngine) Run(pixels []float32) ([]float32, []int64, error) {
	f.pixels = pixels
	if f.panics {
		panic("native boom\nsecond line")
	}
	return f.output, f.shape, f.err
}

func (f *fakeEngine) Close() error { f.closed = true; return nil }

func useEngine(t *testing.T, eng *fakeEngine, opened *runOptions) {
	t.Helper()
	previous := openEngine
	openEngine = func(options runOptions) (engine, error) {
		if opened != nil {
			*opened = options
		}
		return eng, nil
	}
	t.Cleanup(func() { openEngine = previous })
}

func rampOutput() []float32 {
	output := make([]float32, InputSize*InputSize)
	for i := range output {
		output[i] = float32(i % InputSize)
	}
	return output
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 5), G: uint8(y * 9), B: 90, A: 255})
		}
	}
	return encodePNG(t, img)
}

func TestRunWorkerWritesMatteOfInputSize(t *testing.T) {
	eng := &fakeEngine{output: rampOutput(), shape: []int64{1, 1, InputSize, InputSize}}
	var opened runOptions
	useEngine(t, eng, &opened)

	var stdout, stderr bytes.Buffer
	args := []string{"run", "--runtime", "rt", "--model", "m.onnx", "--kind", "precise", "--threads", "3"}
	code := RunWorker(args, bytes.NewReader(testPNG(t, 40, 30)), &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr.String())
	}
	if opened.runtimeRoot != "rt" || opened.modelPath != "m.onnx" || opened.spec.Kind != KindPrecise || opened.threads != 3 {
		t.Fatalf("engine opened with %+v", opened)
	}
	if len(eng.pixels) != 3*InputSize*InputSize || !eng.closed {
		t.Fatalf("engine got %d values, closed %v", len(eng.pixels), eng.closed)
	}
	matte, err := png.Decode(&stdout)
	if err != nil {
		t.Fatalf("stdout is not a PNG: %v", err)
	}
	gray, ok := matte.(*image.Gray)
	if !ok || gray.Bounds().Dx() != 40 || gray.Bounds().Dy() != 30 {
		t.Fatalf("matte is %T %v, want an 8-bit gray 40x30", matte, matte.Bounds())
	}
}

func TestRunWorkerFailuresAreOneLineAndExitOne(t *testing.T) {
	good := testPNG(t, 8, 8)
	cases := map[string]struct {
		input  []byte
		engine *fakeEngine
		want   string
	}{
		"not an image": {[]byte("nope"), &fakeEngine{}, "valid PNG or JPEG"},
		"inference":    {good, &fakeEngine{err: errors.New("session exploded\nwith detail")}, "inference failed: session exploded with detail"},
		"bad shape":    {good, &fakeEngine{output: []float32{1}, shape: []int64{1, 1, 1, 1}}, "unexpected model output shape"},
		"panic":        {good, &fakeEngine{panics: true}, "pixel matte worker panic: native boom second line"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			useEngine(t, c.engine, nil)
			var stdout, stderr bytes.Buffer
			args := []string{"run", "--runtime", "rt", "--model", "m.onnx", "--kind", "standard"}
			code := RunWorker(args, bytes.NewReader(c.input), &stdout, &stderr)
			lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
			if code != 1 || stdout.Len() != 0 || !strings.Contains(lines[len(lines)-1], c.want) {
				t.Fatalf("code %d, stdout %d bytes, last stderr line %q; want 1, none, %q", code, stdout.Len(), lines[len(lines)-1], c.want)
			}
			if name != "panic" && len(lines) != 1 {
				t.Fatalf("stderr has %d lines: %q", len(lines), stderr.String())
			}
		})
	}
}

func TestRunWorkerRejectsOversizedInput(t *testing.T) {
	useEngine(t, &fakeEngine{}, nil)
	var stdout, stderr bytes.Buffer
	args := []string{"run", "--runtime", "rt", "--model", "m.onnx", "--kind", "standard"}
	code := RunWorker(args, io.LimitReader(zeroReader{}, maxInputBytes+1), &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "image exceeds") {
		t.Fatalf("code %d, stderr %q", code, stderr.String())
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func fakeRequest(t *testing.T, image []byte) RunRequest {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return RunRequest{Executable: executable, RuntimeRoot: t.TempDir(), ModelPath: "m.onnx", Kind: KindStandard, Threads: 2, Image: image}
}

func TestProcessRunnerRunsTheWorkerSubprocess(t *testing.T) {
	t.Run("returns stdout", func(t *testing.T) {
		t.Setenv(fakeWorkerEnv, "echo")
		got, err := processRunner{}.Run(context.Background(), fakeRequest(t, []byte("matte bytes")))
		if err != nil || string(got) != "matte bytes" {
			t.Fatalf("got %q, %v", got, err)
		}
	})
	t.Run("reports the last stderr line", func(t *testing.T) {
		t.Setenv(fakeWorkerEnv, "fail")
		_, err := processRunner{}.Run(context.Background(), fakeRequest(t, nil))
		if err == nil || !strings.Contains(err.Error(), "model exploded: bad day") || strings.Contains(err.Error(), "first line") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("empty output is an error", func(t *testing.T) {
		t.Setenv(fakeWorkerEnv, "empty")
		if _, err := (processRunner{}).Run(context.Background(), fakeRequest(t, nil)); err == nil {
			t.Fatal("expected an error")
		}
	})
	t.Run("kills the worker when the context ends", func(t *testing.T) {
		t.Setenv(fakeWorkerEnv, "hang")
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		started := time.Now()
		_, err := processRunner{}.Run(ctx, fakeRequest(t, nil))
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 20*time.Second {
			t.Fatalf("error %v after %v", err, time.Since(started))
		}
	})
	t.Run("stops a worker that writes too much", func(t *testing.T) {
		t.Setenv(fakeWorkerEnv, "flood")
		_, err := processRunner{}.Run(context.Background(), fakeRequest(t, nil))
		if err == nil || !strings.Contains(err.Error(), "output exceeds") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("real worker usage error", func(t *testing.T) {
		request := fakeRequest(t, nil)
		request.Kind = "bogus"
		_, err := processRunner{}.Run(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "--kind must be standard or precise") {
			t.Fatalf("error = %v", err)
		}
	})
}
