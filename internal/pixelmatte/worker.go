package pixelmatte

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"aurago/internal/onnxrt"
)

// WorkerFlag is the first command-line argument that selects the matting
// worker (`aurago --pixel-matte-worker run ...`). cmd/aurago dispatches on it
// before any initialisation and the Manager starts workers with it.
const WorkerFlag = "--pixel-matte-worker"

const (
	workerLogID = "aurago-pixelmatte"
	// maxInputBytes bounds the image the worker reads from stdin.
	maxInputBytes = 64 << 20
)

const workerUsage = "usage: aurago " + WorkerFlag + " run --runtime <dir> --model <file> --kind standard|precise --threads N"

// RunWorker runs `aurago --pixel-matte-worker run [flags]`: it reads one PNG
// or JPEG from stdin, runs the model on the CPU and writes the matte, an 8-bit
// grayscale PNG of the same size, to stdout. Exit code 2 is a usage error, 1 a
// failure (including a recovered panic), 0 success. A failure prints one line
// on stderr (after the stack, for a panic). ONNX Runtime is loaded only here,
// in the subprocess, so a native crash never takes the server down.
func RunWorker(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	defer func() {
		if value := recover(); value != nil {
			fmt.Fprintf(stderr, "%s", debug.Stack())
			fmt.Fprintf(stderr, "pixel matte worker panic: %s\n", oneLine(fmt.Sprint(value)))
			code = 1
		}
	}()
	if len(args) == 0 {
		fmt.Fprintln(stderr, workerUsage)
		return 2
	}
	if args[0] != "run" {
		fmt.Fprintf(stderr, "unknown pixel matte worker command %q\n", args[0])
		return 2
	}
	return runCommand(args[1:], stdin, stdout, stderr)
}

type runOptions struct {
	runtimeRoot string
	modelPath   string
	spec        ModelSpec
	threads     int
}

func parseRunOptions(args []string, stderr io.Writer) (runOptions, bool) {
	flags := flag.NewFlagSet("pixel-matte-worker run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	runtimeRoot := flags.String("runtime", "", "ONNX Runtime directory")
	modelPath := flags.String("model", "", "matting model (.onnx)")
	kind := flags.String("kind", "", "standard | precise")
	threads := flags.Int("threads", DefaultThreads(), "intra-op threads")
	fail := func(message string) (runOptions, bool) {
		fmt.Fprintln(stderr, oneLine(message))
		return runOptions{}, false
	}
	if err := flags.Parse(args); err != nil {
		return fail(err.Error())
	}
	switch {
	case *runtimeRoot == "" || *modelPath == "":
		return fail("--runtime and --model are required")
	case !ValidKind(*kind):
		return fail("--kind must be standard or precise")
	case *threads < 1 || *threads > 64:
		return fail("--threads must be between 1 and 64")
	case flags.NArg() > 0:
		return fail(fmt.Sprintf("unexpected argument(s) %q", flags.Args()))
	}
	return runOptions{runtimeRoot: *runtimeRoot, modelPath: *modelPath, spec: models[Kind(*kind)], threads: *threads}, true
}

func runCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	options, ok := parseRunOptions(args, stderr)
	if !ok {
		return 2
	}
	matte, err := runMatte(options, stdin)
	if err != nil {
		fmt.Fprintln(stderr, oneLine(err.Error()))
		return 1
	}
	if _, err := stdout.Write(matte); err != nil {
		fmt.Fprintf(stderr, "write matte: %s\n", oneLine(err.Error()))
		return 1
	}
	return 0
}

// engine runs the model on preprocessed pixels; tests replace openEngine.
type engine interface {
	Run(pixels []float32) (output []float32, shape []int64, err error)
	Close() error
}

// openEngine loads the runtime and the model. It is a variable so tests can
// run the pipeline without native code.
var openEngine = openORTEngine

// runMatte is the whole worker job: input, preprocessing, inference,
// post-processing and PNG encoding.
func runMatte(options runOptions, stdin io.Reader) ([]byte, error) {
	input, err := io.ReadAll(io.LimitReader(stdin, maxInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	if len(input) > maxInputBytes {
		return nil, fmt.Errorf("image exceeds %d bytes", maxInputBytes)
	}
	// Decode before loading the runtime: a bad image fails in milliseconds.
	pixels, width, height, err := Preprocess(input, options.spec)
	if err != nil {
		return nil, err
	}
	input = nil

	eng, err := openEngine(options)
	if err != nil {
		return nil, err
	}
	defer eng.Close()
	output, shape, err := eng.Run(pixels)
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}
	pixels = nil
	matte, err := Postprocess(output, shape, options.spec, width, height)
	if err != nil {
		return nil, err
	}
	return encodeMatte(matte)
}

// ortEngine is the ONNX Runtime backed engine.
type ortEngine struct {
	runtime    *onnxrt.Runtime
	session    *onnxrt.Session
	inputName  string
	outputName string
}

func openORTEngine(options runOptions) (engine, error) {
	model, err := os.ReadFile(options.modelPath)
	if err != nil {
		return nil, fmt.Errorf("read model: %w", err)
	}
	inputName, outputName, err := ParseIONames(model)
	if err != nil {
		return nil, fmt.Errorf("read model tensor names: %w", err)
	}
	rt, err := onnxrt.Load(options.runtimeRoot, workerLogID)
	if err != nil {
		return nil, err
	}
	session, err := rt.NewSession(model, onnxrt.SessionOptions{Threads: options.threads, DisableSpinning: true})
	if err != nil {
		return nil, errors.Join(err, rt.Close())
	}
	return &ortEngine{runtime: rt, session: session, inputName: inputName, outputName: outputName}, nil
}

func (e *ortEngine) Run(pixels []float32) ([]float32, []int64, error) {
	outputs, err := e.session.Run(map[string]onnxrt.Tensor{
		e.inputName: {Shape: []int64{1, 3, InputSize, InputSize}, Float32: pixels},
	}, []string{e.outputName})
	if err != nil {
		return nil, nil, err
	}
	output := outputs[e.outputName]
	if output.Float32 == nil {
		return nil, nil, fmt.Errorf("model output %s is not float32", e.outputName)
	}
	return output.Float32, output.Shape, nil
}

// Close releases the session before the runtime: the runtime refuses to
// unload while a session is open.
func (e *ortEngine) Close() error {
	sessionErr := e.session.Close()
	return errors.Join(sessionErr, e.runtime.Close())
}

// oneLine collapses all whitespace, so a message is a single stderr line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
