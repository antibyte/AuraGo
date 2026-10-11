package persondetect

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"aurago/internal/fileutil"
	"aurago/internal/onnxrt"
)

const (
	workerFlag  = "--person-detect-worker"
	workerLogID = "aurago-persondetect"

	// Detection-set comparison on the reference image (overview C3): raw
	// GPU/CPU (or original/patched) tensors differ on low-confidence queries,
	// detections do not. Both sets are decoded at decodeMinScore and compared
	// with SameConfidentDetections(…, compareMinScore, sameMinIoU,
	// sameMaxScoreDiff), so a box right at 0.3 (D-FINE-N scores one reference
	// person at 0.303) cannot flip the verdict; the reference side must also
	// find at least one person with score ≥ referenceMinScore, so two empty
	// sets never pass.
	compareMinScore   = 0.3
	referenceMinScore = 0.5
	sameMinIoU        = 0.9
	sameMaxScoreDiff  = 0.02
	decodeMinScore    = compareMinScore - sameMaxScoreDiff // 0.28
)

// RunWorker runs `aurago --person-detect-worker <prepare|serve> [flags]`. It
// must be dispatched before normal AuraGo initialization so native runtime
// failures stay inside the worker process. Exit code 2 is a usage error, 1 a
// failure (including a recovered panic), 0 success.
func RunWorker(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	defer recoverWorkerPanic(stdout, stderr, &code)
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: aurago --person-detect-worker prepare|serve [flags]")
		return 2
	}
	switch args[0] {
	case "prepare":
		return runPrepareCommand(args[1:], stdout, stderr)
	case "serve":
		return runServeCommand(args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown person detection worker command %q\n", args[0])
		return 2
	}
}

// recoverWorkerPanic, deferred by RunWorker, turns a panic into a JSON error
// line on stdout (stack on stderr) and exit code 1.
func recoverWorkerPanic(stdout, stderr io.Writer, code *int) {
	value := recover()
	if value == nil {
		return
	}
	fmt.Fprintf(stderr, "person detection worker panic: %v\n%s", value, debug.Stack())
	_ = writeJSONLine(stdout, preparedMessage{Type: "error", Error: fmt.Sprintf("person detection worker panic: %v", value)})
	*code = 1
}

func runPrepareCommand(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("person-detect-worker prepare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	runtimeRoot := flags.String("runtime", "", "ONNX Runtime directory")
	input := flags.String("in", "", "pinned D-FINE model")
	output := flags.String("out", "", "WebGPU-compatible model to write")
	if err := flags.Parse(args); err != nil {
		return prepareUsageError(stdout, err.Error())
	}
	if *runtimeRoot == "" || *input == "" || *output == "" {
		return prepareUsageError(stdout, "--runtime, --in and --out are required")
	}
	if flags.NArg() > 0 {
		return prepareUsageError(stdout, fmt.Sprintf("unexpected argument(s) %q", flags.Args()))
	}
	if _, ok := outputFileName(*output); !ok {
		return prepareUsageError(stdout, "--out must name a file")
	}
	if sameFile(*input, *output) {
		return prepareUsageError(stdout, "--in and --out must name different files")
	}
	count, summary, err := prepareModel(*runtimeRoot, *input, *output)
	if err != nil {
		_ = writeJSONLine(stdout, preparedMessage{Type: "error", Error: err.Error()})
		return 1
	}
	fmt.Fprintf(stderr, "prepared %s: %d MatMul node(s) patched; reference check: %s\n", filepath.Base(*output), count, summary)
	if err := writeJSONLine(stdout, preparedMessage{Type: "prepared", PatchedNodes: count}); err != nil {
		fmt.Fprintf(stderr, "write prepare result: %v\n", err)
		return 1
	}
	return 0
}

func prepareUsageError(stdout io.Writer, message string) int {
	_ = writeJSONLine(stdout, preparedMessage{Type: "error", Error: message})
	return 2
}

// sameFile reports whether a and b name the same file: equal absolute
// (cleaned) paths, or two existing paths that os.SameFile matches (case
// variants on case-insensitive file systems, links).
func sameFile(a, b string) bool {
	absoluteA, errA := filepath.Abs(a)
	absoluteB, errB := filepath.Abs(b)
	if errA == nil && errB == nil && absoluteA == absoluteB {
		return true
	}
	infoA, errA := os.Stat(a)
	infoB, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(infoA, infoB)
}

// prepareModel writes the WebGPU-compatible variant of inputPath to
// outputPath. It folds constants with ORT's basic (hardware-independent)
// optimization level using the given runtime, patches the folded graph and
// verifies it against the original on the CPU. It returns the number of
// patched MatMul nodes and the reference check summary.
func prepareModel(runtimeRoot, inputPath, outputPath string) (int, string, error) {
	original, err := os.ReadFile(inputPath)
	if err != nil {
		return 0, "", fmt.Errorf("read model: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return 0, "", fmt.Errorf("create model directory: %w", err)
	}
	sweepStalePrepareFiles(outputPath, inputPath)
	rt, err := onnxrt.Load(runtimeRoot, workerLogID)
	if err != nil {
		return 0, "", fmt.Errorf("load ONNX Runtime: %w", err)
	}
	defer rt.Close()
	suffix, err := randomHex()
	if err != nil {
		return 0, "", err
	}
	foldedPath := outputPath + foldedInfix + suffix + ".onnx"
	defer os.Remove(foldedPath)
	// Never OptimizationAll here: higher levels bake hardware-specific
	// kernels and layouts into the saved model.
	folder, err := rt.NewSession(original, onnxrt.SessionOptions{
		Threads:            1,
		DisableSpinning:    true,
		Optimization:       onnxrt.OptimizationBasic,
		OptimizedModelPath: foldedPath,
	})
	if err != nil {
		return 0, "", fmt.Errorf("fold model: %w", err)
	}
	if err := folder.Close(); err != nil {
		return 0, "", fmt.Errorf("close fold session: %w", err)
	}
	folded, err := os.ReadFile(foldedPath)
	if err != nil {
		return 0, "", fmt.Errorf("read folded model: %w", err)
	}
	patched, count, err := patchFoldedModel(folded)
	if err != nil {
		return 0, "", err
	}
	summary, err := verifyPatchedModel(rt, original, patched)
	if err != nil {
		return 0, "", err
	}
	if err := writeFileAtomic(outputPath, patched, 0o640); err != nil {
		return 0, "", err
	}
	return count, summary, nil
}

// Temp file name parts next to an output: <out>.folded-<hex>.onnx (ORT's
// optimized model) and <out>.tmp-<hex> (writeFileAtomic).
const (
	foldedInfix = ".folded-"
	tempInfix   = ".tmp-"
)

// outputFileName is the file name of an --out path; ok is false when the path
// names no file ("dir/", "dir/.", "dir/..", a bare volume such as "C:").
func outputFileName(outputPath string) (string, bool) {
	_, base := filepath.Split(outputPath)
	return base, base != "" && base != "." && base != ".."
}

// sweepStalePrepareFiles removes the temp files a killed or crashed prepare
// of outputPath left behind (15–41 MB each). It is best effort, matches
// names literally (data paths may contain Glob pattern characters), skips
// directories and inputPath, and does nothing when outputPath names no file
// (the prefixes would match unrelated dot-files).
//
// Overlapping prepares of one output cannot corrupt it: temp names are
// random and only a verified file is published, by rename. A sweep can,
// however, delete the other prepare's temp files and make it fail, so
// callers serialize prepares per output (the detector holds a per-model-file
// mutex).
func sweepStalePrepareFiles(outputPath, inputPath string) {
	base, ok := outputFileName(outputPath)
	if !ok {
		return
	}
	dir := filepath.Dir(outputPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !(strings.HasPrefix(name, base+foldedInfix) || strings.HasPrefix(name, base+tempInfix)) {
			continue
		}
		path := filepath.Join(dir, name)
		if inputPath != "" && sameFile(path, inputPath) {
			continue
		}
		_ = os.Remove(path)
	}
}

// patchFoldedModel applies PatchVectorMatMuls and requires at least one
// patched node: PatchVectorMatMuls refuses unsafe rewrites by returning the
// model unchanged with count 0, and that model cannot run on WebGPU.
func patchFoldedModel(folded []byte) ([]byte, int, error) {
	patched, count, err := PatchVectorMatMuls(folded)
	if err != nil {
		return nil, 0, fmt.Errorf("patch folded model: %w", err)
	}
	if count == 0 {
		return nil, 0, fmt.Errorf("no patchable MatMul in the folded model (a 1-D float weight whose MatMul feeds only Reshape nodes); the WebGPU execution provider cannot run it")
	}
	return patched, count, nil
}

// verifyPatchedModel runs the original and the patched model on two CPU
// sessions on the reference image and applies compareReferenceDetections.
func verifyPatchedModel(rt *onnxrt.Runtime, original, patched []byte) (string, error) {
	pixels, _, _, err := Preprocess(ReferenceImage(), dfineInputSize)
	if err != nil {
		return "", fmt.Errorf("preprocess reference image: %w", err)
	}
	labels := [2]string{"original", "patched"}
	var detections [2][]Box
	for i, model := range [][]byte{original, patched} {
		session, err := rt.NewSession(model, onnxrt.SessionOptions{Threads: cpuThreads(), DisableSpinning: true})
		if err != nil {
			return "", fmt.Errorf("load %s model for verification: %w", labels[i], err)
		}
		boxes, runErr := detectPixels(&ortEngine{session: session}, pixels, dfineInputSize, decodeMinScore)
		closeErr := session.Close()
		if runErr != nil {
			return "", fmt.Errorf("run %s model: %w", labels[i], runErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close %s session: %w", labels[i], closeErr)
		}
		detections[i] = boxes
	}
	return compareReferenceDetections(detections[0], detections[1])
}

// compareReferenceDetections is prepare's acceptance rule (overview C3) for
// the reference detections of the original and the patched model, both
// decoded at decodeMinScore: SameConfidentDetections(original, patched, 0.3,
// 0.9, 0.02), and the original model must find at least one person ≥ 0.5 (a
// preprocessing regression must not let any patched model through). It
// returns a summary of the check.
func compareReferenceDetections(original, patched []Box) (string, error) {
	same, reason := SameConfidentDetections(original, patched, compareMinScore, sameMinIoU, sameMaxScoreDiff)
	if !same {
		return "", fmt.Errorf("patched model changes the reference detections (set a = original, set b = patched): %s", reason)
	}
	persons := countAtLeast(original, referenceMinScore)
	if persons < 1 {
		return "", fmt.Errorf("original model finds no person with score >= %.1f on the reference image (%s)", referenceMinScore, reason)
	}
	return fmt.Sprintf("%s; original model: %d person(s) >= %.1f", reason, persons, referenceMinScore), nil
}

func countAtLeast(boxes []Box, minScore float64) int {
	count := 0
	for _, box := range boxes {
		if box.Score >= minScore {
			count++
		}
	}
	return count
}

func writeJSONLine(output io.Writer, value any) error {
	return json.NewEncoder(output).Encode(value)
}

// writeFileAtomic writes data to a new temp file next to path (<path>.tmp-<hex>,
// so sweepStalePrepareFiles finds it after a crash), flushes it to disk and
// renames it over path with fileutil.Rename, which retries transient Windows
// sharing violations (a reader holding the old file). Readers see the old or
// the new content; on failure the temp file is removed and path is
// untouched. On Unix the directory is synced after the rename (best effort).
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create directory for %s: %w", filepath.Base(path), err)
	}
	suffix, err := randomHex()
	if err != nil {
		return err
	}
	temp := path + tempInfix + suffix
	file, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("write %s: %w", filepath.Base(path), writeErr)
	}
	if err := fileutil.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("activate %s: %w", filepath.Base(path), err)
	}
	if runtime.GOOS != "windows" {
		if handle, err := os.Open(dir); err == nil {
			_ = handle.Sync()
			_ = handle.Close()
		}
	}
	return nil
}

func randomHex() (string, error) {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("generate random suffix: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}
