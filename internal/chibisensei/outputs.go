package chibisensei

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Output file names of the trainer (flat in /kaggle/working).
const (
	OutStatus        = "chibi-status.json"
	OutManifest      = "chibi-manifest.json"
	OutMetrics       = "chibi-metrics.jsonl"
	OutEvalSamples   = "chibi-eval_samples.jsonl"
	OutModelGGUF     = "chibi-model-q8_0.gguf"
	OutBaseGGUF      = "chibi-base-q8_0.gguf"
	OutAdapter       = "chibi-adapter-model.safetensors"
	OutAdapterConfig = "chibi-adapter-config.json"
)

// ManifestKindFile maps each manifest kind to its only allowed file name.
var ManifestKindFile = map[string]string{"gguf_tuned": OutModelGGUF, "gguf_base": OutBaseGGUF,
	"adapter": OutAdapter, "adapter_config": OutAdapterConfig, "metrics": OutMetrics,
	"eval_samples": OutEvalSamples, "status": OutStatus}

const (
	maxStatusBytes      = 1 << 20
	maxManifestBytes    = 1 << 20
	maxArtifactBytes    = 8 << 30
	maxMetricsFileBytes = 64 << 20
	maxMetricLineBytes  = 4 << 10
	maxMetricPoints     = 20000
	maxEvalFileBytes    = 256 << 20
	maxEvalLineBytes    = 1 << 20
	maxEvalSamplesCap   = 1000
	maxEvalTextRunes    = 8000
)

// FlexString decodes a JSON string or number (null gives "").
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case bytes.Equal(b, []byte("null")):
		*f = ""
	case len(b) > 0 && b[0] == '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexString(s)
	default:
		var n json.Number
		if err := json.Unmarshal(b, &n); err != nil {
			return errors.New("expected a string or a number")
		}
		*f = FlexString(n.String())
	}
	return nil
}

type ManifestFile struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	V          int            `json:"v"`
	RunID      string         `json:"run_id"`
	ProfileID  string         `json:"profile_id"`
	ProfileSHA string         `json:"profile_sha"`
	Files      []ManifestFile `json:"files"`
}

// ValidManifestPath accepts a flat "chibi-" file name: no separator, no "..", no colon or NUL, trimmed.
func ValidManifestPath(p string) bool {
	return len(p) > len(OutputPrefix) && len(p) <= 128 && strings.HasPrefix(p, OutputPrefix) &&
		!strings.ContainsAny(p, "/\\:\x00") && !strings.Contains(p, "..") && strings.TrimSpace(p) == p
}

// ParseManifest decodes and validates chibi-manifest.json. Errors never quote file content.
func ParseManifest(b []byte) (Manifest, error) {
	if len(b) > maxManifestBytes {
		return Manifest{}, errors.New("manifest: file too large")
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("manifest: %w", err)
	}
	switch {
	case m.V != 1:
		return Manifest{}, errors.New("manifest: unsupported version")
	case !ValidID(m.RunID):
		return Manifest{}, errors.New("manifest: invalid run id")
	case !profileIDPattern.MatchString(m.ProfileID):
		return Manifest{}, errors.New("manifest: invalid profile id")
	case !sha256Hex.MatchString(m.ProfileSHA):
		return Manifest{}, errors.New("manifest: invalid profile sha")
	case len(m.Files) == 0 || len(m.Files) > len(ManifestKindFile):
		return Manifest{}, errors.New("manifest: wrong number of files")
	}
	seenPath, seenKind := map[string]bool{}, map[string]bool{}
	for i, f := range m.Files {
		if !ValidManifestPath(f.Path) {
			return Manifest{}, fmt.Errorf("manifest: file %d: invalid path", i)
		}
		want, ok := ManifestKindFile[f.Kind]
		if !ok {
			return Manifest{}, fmt.Errorf("manifest: file %d: unknown kind", i)
		}
		if f.Path != want {
			return Manifest{}, fmt.Errorf("manifest: file %d: kind %s must be named %s", i, f.Kind, want)
		}
		if seenPath[f.Path] || seenKind[f.Kind] {
			return Manifest{}, fmt.Errorf("manifest: file %d: listed twice", i)
		}
		seenPath[f.Path], seenKind[f.Kind] = true, true
		if f.Bytes < 0 || f.Bytes > maxArtifactBytes {
			return Manifest{}, fmt.Errorf("manifest: file %d: size out of range", i)
		}
		if !sha256Hex.MatchString(f.SHA256) {
			return Manifest{}, fmt.Errorf("manifest: file %d: invalid sha256", i)
		}
	}
	return m, nil
}

// eachLine calls fn for every non-blank line of at most maxLine bytes (CR/LF stripped). Longer lines are
// skipped whole. fn returns false to stop.
func eachLine(r io.Reader, maxLine int, fn func([]byte) bool) error {
	br := bufio.NewReaderSize(r, maxLine+2)
	for {
		line, err := br.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			for errors.Is(err, bufio.ErrBufferFull) {
				_, err = br.ReadSlice('\n')
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			continue
		}
		if t := bytes.TrimRight(line, "\r\n"); len(t) <= maxLine && len(bytes.TrimSpace(t)) > 0 {
			if !fn(t) {
				return nil
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

type StatusGPU struct {
	Name       string     `json:"name"`
	Capability FlexString `json:"capability"`
	VRAMGB     float64    `json:"vram_gb"`
}

type StatusVersions struct {
	Python       string `json:"python"`
	Torch        string `json:"torch"`
	Transformers string `json:"transformers"`
	PEFT         string `json:"peft"`
	LlamaCpp     string `json:"llama_cpp"`
}

type StatusTrain struct {
	Steps         int      `json:"steps"`
	Epochs        float64  `json:"epochs"`
	TrainLoss     *float64 `json:"train_loss"`
	EvalLossBase  *float64 `json:"eval_loss_base"`
	EvalLossTuned *float64 `json:"eval_loss_tuned"`
	StopReason    string   `json:"stop_reason"`
	TruncatedRows int      `json:"truncated_rows"`
}

type Status struct {
	V           int            `json:"v"`
	RunID       string         `json:"run_id"`
	State       string         `json:"state"`
	Stage       string         `json:"stage"`
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	WarningCode string         `json:"warning_code"`
	Started     string         `json:"started"`
	Finished    string         `json:"finished"`
	GPU         StatusGPU      `json:"gpu"`
	Versions    StatusVersions `json:"versions"`
	Train       StatusTrain    `json:"train"`
	Discovery   map[string]any `json:"discovery"`
}

// Succeeded reports whether the trainer finished the whole run.
func (s Status) Succeeded() bool { return s.State == "succeeded" }

// ParseStatus decodes and validates chibi-status.json. Errors never quote file content.
func ParseStatus(b []byte) (Status, error) {
	if len(b) > maxStatusBytes {
		return Status{}, errors.New("status: file too large")
	}
	var s Status
	if err := json.Unmarshal(b, &s); err != nil {
		return Status{}, fmt.Errorf("status: %w", err)
	}
	switch {
	case s.V != 1:
		return Status{}, errors.New("status: unsupported version")
	case !ValidID(s.RunID):
		return Status{}, errors.New("status: invalid run id")
	case s.State != "succeeded" && s.State != "failed":
		return Status{}, errors.New("status: unknown state")
	case s.Stage != "" && !isTrainerStage(s.Stage):
		return Status{}, errors.New("status: unknown stage")
	case s.Code != "" && !codePattern.MatchString(s.Code):
		return Status{}, errors.New("status: invalid code")
	case s.WarningCode != "" && !codePattern.MatchString(s.WarningCode):
		return Status{}, errors.New("status: invalid warning code")
	case !containsString([]string{"", "done", "time_budget", "train_diverged", "oom"}, s.Train.StopReason):
		return Status{}, errors.New("status: unknown stop reason")
	case s.Train.Steps < 0 || s.Train.TruncatedRows < 0:
		return Status{}, errors.New("status: negative counter")
	}
	s.Message = cutRunes(s.Message, maxLogMessage)
	s.GPU.Name = cutRunes(s.GPU.Name, 128)
	s.GPU.Capability = FlexString(cutRunes(string(s.GPU.Capability), 16))
	v := &s.Versions
	v.Python, v.Torch, v.Transformers = cutRunes(v.Python, 64), cutRunes(v.Torch, 64), cutRunes(v.Transformers, 64)
	v.PEFT, v.LlamaCpp = cutRunes(v.PEFT, 64), cutRunes(v.LlamaCpp, 64)
	return s, nil
}

// ParseMetricsFile reads chibi-metrics.jsonl: invalid lines are skipped, (step, kind) pairs dedupe with the
// first line winning, and the points come back sorted by step, then kind.
func ParseMetricsFile(r io.Reader) ([]Metric, error) {
	type key struct {
		step int
		kind string
	}
	seen := map[key]bool{}
	out := []Metric{}
	err := eachLine(io.LimitReader(r, maxMetricsFileBytes), maxMetricLineBytes, func(line []byte) bool {
		m, ok := parseMetric(line)
		if !ok || seen[key{m.Step, m.Kind}] {
			return true
		}
		seen[key{m.Step, m.Kind}] = true
		out = append(out, m)
		return len(out) < maxMetricPoints
	})
	if err != nil {
		return nil, fmt.Errorf("metrics: %w", err)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Step != out[j].Step {
			return out[i].Step < out[j].Step
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

// EvalSample is one line of chibi-eval_samples.jsonl.
type EvalSample struct {
	ID             FlexString `json:"id"` // 0-based line index of the eval file
	PromptMessages []Message  `json:"prompt_messages,omitempty"`
	PromptText     string     `json:"prompt_text,omitempty"`
	Expected       string     `json:"expected"`
	BaseOutput     string     `json:"base_output"`
	TunedOutput    string     `json:"tuned_output"`
	BaseMS         int64      `json:"base_ms"`
	TunedMS        int64      `json:"tuned_ms"`
	Truncated      bool       `json:"truncated"`
}

// ParseEvalSamples reads chibi-eval_samples.jsonl, skipping invalid lines, and returns at most limit samples.
func ParseEvalSamples(r io.Reader, limit int) ([]EvalSample, error) {
	if limit <= 0 || limit > maxEvalSamplesCap {
		limit = maxEvalSamplesCap
	}
	out := []EvalSample{}
	err := eachLine(io.LimitReader(r, maxEvalFileBytes), maxEvalLineBytes, func(line []byte) bool {
		var s EvalSample
		if json.Unmarshal(line, &s) != nil || len(s.PromptMessages) > MaxMessages ||
			(len(s.PromptMessages) == 0 && strings.TrimSpace(s.PromptText) == "") {
			return true
		}
		for i := range s.PromptMessages {
			m := &s.PromptMessages[i]
			if m.Role != "system" && m.Role != "user" && m.Role != "assistant" {
				return true
			}
			m.Content = cutRunes(m.Content, maxEvalTextRunes)
		}
		s.ID = FlexString(cutRunes(string(s.ID), 40))
		s.PromptText = cutRunes(s.PromptText, maxEvalTextRunes)
		s.Expected = cutRunes(s.Expected, maxEvalTextRunes)
		s.BaseOutput = cutRunes(s.BaseOutput, maxEvalTextRunes)
		s.TunedOutput = cutRunes(s.TunedOutput, maxEvalTextRunes)
		s.BaseMS, s.TunedMS = max(s.BaseMS, 0), max(s.TunedMS, 0)
		out = append(out, s)
		return len(out) < limit
	})
	if err != nil {
		return nil, fmt.Errorf("eval samples: %w", err)
	}
	return out, nil
}
