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

	"aurago/internal/onnxrt"
)

const (
	workerFlag  = "--person-detect-worker"
	workerLogID = "aurago-persondetect"

	// Detection-set comparison on the reference image (overview C3): raw
	// GPU/CPU (or original/patched) tensors differ on low-confidence queries,
	// detections do not. Both sets are decoded at decodeMinScore =
	// compareMinScore - sameMaxScoreDiff and compared with
	// SameConfidentDetections(…, compareMinScore, sameMinIoU,
	// sameMaxScoreDiff), so a box right at 0.3 (D-FINE-N scores one reference
	// person at 0.303) cannot flip the verdict; the reference side must also
	// find at least one person with score ≥ referenceMinScore, so two empty
	// sets never pass.
	decodeMinScore    = 0.28
	compareMinScore   = 0.3
	referenceMinScore = 0.5
	sameMinIoU        = 0.9
	sameMaxScoreDiff  = 0.02
)

// RunWorker runs `aurago --person-detect-worker <prepare|serve> [flags]`. It
// must be dispatched before normal AuraGo initialization so native runtime
// failures stay inside the worker process.
func RunWorker(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: aurago --person-detect-worker prepare|serve [flags]")
		return 2
	}
	switch args[0] {
	case "prepare":
		return runPrepareCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown person detection worker command %q\n", args[0])
		return 2
	}
}

func runPrepareCommand(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("person-detect-worker prepare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	runtimeRoot := flags.String("runtime", "", "ONNX Runtime directory")
	input := flags.String("in", "", "pinned D-FINE model")
	output := flags.String("out", "", "WebGPU-compatible model to write")
	if err := flags.Parse(args); err != nil {
		_ = writeJSONLine(stdout, preparedMessage{Type: "error", Error: err.Error()})
		return 2
	}
	if *runtimeRoot == "" || *input == "" || *output == "" {
		_ = writeJSONLine(stdout, preparedMessage{Type: "error", Error: "--runtime, --in and --out are required"})
		return 2
	}
	count, _, err := prepareModel(*runtimeRoot, *input, *output)
	if err != nil {
		_ = writeJSONLine(stdout, preparedMessage{Type: "error", Error: err.Error()})
		return 1
	}
	if err := writeJSONLine(stdout, preparedMessage{Type: "prepared", PatchedNodes: count}); err != nil {
		fmt.Fprintf(stderr, "write prepare result: %v\n", err)
		return 1
	}
	return 0
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
	rt, err := onnxrt.Load(runtimeRoot, workerLogID)
	if err != nil {
		return 0, "", err
	}
	defer rt.Close()
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return 0, "", fmt.Errorf("create model directory: %w", err)
	}
	suffix, err := randomHex()
	if err != nil {
		return 0, "", err
	}
	foldedPath := outputPath + ".folded-" + suffix + ".onnx"
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

// writeFileAtomic writes data to a new temp file next to path, flushes it to
// disk and renames it over path, so readers see the old or the new content.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create directory for %s: %w", filepath.Base(path), err)
	}
	suffix, err := randomHex()
	if err != nil {
		return err
	}
	temp := path + ".tmp-" + suffix
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
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("activate %s: %w", filepath.Base(path), err)
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
