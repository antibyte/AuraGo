package chibisensei

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// ErrSamplesMissing means the run has no evaluation examples file.
var ErrSamplesMissing = errors.New("chibisensei: the run has no evaluation examples")

// SamplePageMax is the largest page ReadSamples returns.
const SamplePageMax = 50

// SampleView is one evaluation example as the Test step shows it.
type SampleView struct {
	Index          int       `json:"index"`
	PromptMessages []Message `json:"prompt_messages"` // null for text prompts
	PromptText     *string   `json:"prompt_text"`
	Expected       string    `json:"expected"`
	BaseOutput     *string   `json:"base_output"`
	TunedOutput    *string   `json:"tuned_output"`
	BaseMS         *int64    `json:"base_ms"`
	TunedMS        *int64    `json:"tuned_ms"`
	Truncated      bool      `json:"truncated"`
	BaseMatch      *bool     `json:"base_match"` // null when the recipe has no match rule or the answer is missing
	TunedMatch     *bool     `json:"tuned_match"`
}

// SamplePage is one page of examples after the filter.
type SamplePage struct {
	Samples []SampleView `json:"samples"`
	Total   int          `json:"total"` // after the filter
	Offset  int          `json:"offset"`
	Limit   int          `json:"limit"`
}

// ReadSamples reads the run's evaluation examples through os.Root and applies the report's own match rules.
// filter: all | different (both answers present and different after normalization) | wrong (tuned_match is false).
func ReadSamples(dataDir string, run Run, recipe, taskKind string, labels []string, offset, limit int, filter string) (SamplePage, error) {
	if !ValidID(run.ID) {
		return SamplePage{}, errors.New("read samples: invalid run id")
	}
	switch filter {
	case "all", "different", "wrong":
	default:
		return SamplePage{}, fmt.Errorf("read samples: unknown filter %q", filter)
	}
	limit = min(max(limit, 1), SamplePageMax)
	offset = max(offset, 0)
	root, err := os.OpenRoot(ChibiDir(dataDir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return SamplePage{}, ErrSamplesMissing
		}
		return SamplePage{}, fmt.Errorf("open %s: %w", chibiDirName, err)
	}
	defer root.Close()
	f, err := root.Open(runRel(run.ID, outSubdir, "eval_samples.jsonl"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return SamplePage{}, ErrSamplesMissing
		}
		return SamplePage{}, fmt.Errorf("read samples: %w", err)
	}
	defer f.Close()
	want := 0
	if run.Remote.Plan != nil {
		want = run.Remote.Plan.EvalSamples
	}
	all, err := ParseEvalSamples(f, want)
	if err != nil {
		return SamplePage{}, err
	}
	page := SamplePage{Samples: []SampleView{}, Offset: offset, Limit: limit}
	for i, s := range all {
		v := sampleView(i, s, recipe, taskKind, labels)
		if !sampleKept(v, filter) {
			continue
		}
		if page.Total >= offset && len(page.Samples) < limit {
			page.Samples = append(page.Samples, v)
		}
		page.Total++
	}
	return page, nil
}

func sampleView(i int, s EvalSample, recipe, taskKind string, labels []string) SampleView {
	expected, base, tuned := sampleAnswers(s)
	v := SampleView{Index: i, Expected: expected, BaseOutput: base, TunedOutput: tuned, BaseMS: s.BaseMS,
		TunedMS: s.TunedMS, Truncated: s.Truncated,
		BaseMatch:  sampleMatch(recipe, taskKind, labels, expected, base),
		TunedMatch: sampleMatch(recipe, taskKind, labels, expected, tuned)}
	if len(s.PromptMessages) > 0 {
		v.PromptMessages = append([]Message(nil), s.PromptMessages...)
	} else {
		text := s.PromptText
		v.PromptText = &text
	}
	return v
}

func sampleKept(v SampleView, filter string) bool {
	switch filter {
	case "different":
		return v.BaseOutput != nil && v.TunedOutput != nil &&
			normalizeAnswer(*v.BaseOutput) != normalizeAnswer(*v.TunedOutput)
	case "wrong":
		return v.TunedMatch != nil && !*v.TunedMatch
	}
	return true
}

// sampleMatch applies the report's rule for the recipe; nil when the recipe has no rule or the answer is missing.
func sampleMatch(recipe, taskKind string, labels []string, expected string, answer *string) *bool {
	if answer == nil || recipe != RecipeSpecialist {
		return nil
	}
	var ok bool
	switch taskKind {
	case "extract", "rewrite", "to_json":
		ok = answerMatches(taskKind, expected, *answer)
	case "classify":
		e := matchLabel(labels, expected)
		if e < 0 {
			return nil
		}
		ok = matchLabel(labels, *answer) == e
	default:
		return nil
	}
	return &ok
}
