package localllm

import (
	"encoding/json"
	"strings"
)

// BenchmarkSample contains one comparable MTP run.
type BenchmarkSample struct {
	ToolCall          string
	GenerationTokensS float64
	TTFTMilliseconds  float64
	DraftAcceptance   float64
	OOM               bool
	OffloadError      bool
}

// MTPDecision is safe to expose through status APIs.
type MTPDecision struct {
	Selected    bool               `json:"selected"`
	Reason      string             `json:"reason"`
	Runtime     *RuntimeBenchmark  `json:"runtime,omitempty"`
	PromptCache *PromptCacheStatus `json:"prompt_cache,omitempty"`
}

// RuntimeBenchmark exposes only value-free aggregate measurements.
type RuntimeBenchmark struct {
	PerformanceProfile string  `json:"performance_profile"`
	ContextSize        int     `json:"context_size"`
	GenerationTPS      float64 `json:"generation_tps,omitempty"`
	TTFTMilliseconds   float64 `json:"ttft_ms,omitempty"`
	DraftAcceptance    float64 `json:"draft_acceptance,omitempty"`
}

// EvaluateMTP enforces the fixed v1 auto-selection thresholds.
func EvaluateMTP(target, speculative []BenchmarkSample) MTPDecision {
	if len(target) != 3 || len(speculative) != 3 {
		return MTPDecision{Reason: "mtp_requires_three_measured_runs"}
	}
	for i := range target {
		if target[i].OOM || target[i].OffloadError || speculative[i].OOM || speculative[i].OffloadError {
			return MTPDecision{Reason: "mtp_oom_or_offload_error"}
		}
		if normalizeToolCall(target[i].ToolCall) != normalizeToolCall(speculative[i].ToolCall) {
			return MTPDecision{Reason: "mtp_tool_call_mismatch"}
		}
	}
	targetSpeed := median(target, func(v BenchmarkSample) float64 { return v.GenerationTokensS })
	specSpeed := median(speculative, func(v BenchmarkSample) float64 { return v.GenerationTokensS })
	targetTTFT := median(target, func(v BenchmarkSample) float64 { return v.TTFTMilliseconds })
	specTTFT := median(speculative, func(v BenchmarkSample) float64 { return v.TTFTMilliseconds })
	acceptance := median(speculative, func(v BenchmarkSample) float64 { return v.DraftAcceptance })
	if acceptance < 0.80 {
		return MTPDecision{Reason: "mtp_acceptance_below_80_percent"}
	}
	if targetSpeed <= 0 || specSpeed < targetSpeed*1.10 {
		return MTPDecision{Reason: "mtp_speed_gain_below_10_percent"}
	}
	if targetTTFT <= 0 || specTTFT > targetTTFT*1.25 {
		return MTPDecision{Reason: "mtp_ttft_regression_above_25_percent"}
	}
	return MTPDecision{Selected: true, Reason: "mtp_thresholds_passed"}
}

func median(values []BenchmarkSample, selectValue func(BenchmarkSample) float64) float64 {
	numbers := []float64{selectValue(values[0]), selectValue(values[1]), selectValue(values[2])}
	if numbers[0] > numbers[1] {
		numbers[0], numbers[1] = numbers[1], numbers[0]
	}
	if numbers[1] > numbers[2] {
		numbers[1], numbers[2] = numbers[2], numbers[1]
	}
	if numbers[0] > numbers[1] {
		numbers[0], numbers[1] = numbers[1], numbers[0]
	}
	return numbers[1]
}

func normalizeToolCall(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var call struct {
		Name      string `json:"name"`
		Arguments any    `json:"arguments"`
		Function  *struct {
			Name      string `json:"name"`
			Arguments any    `json:"arguments"`
		} `json:"function"`
	}
	if json.Unmarshal([]byte(value), &call) != nil {
		return value
	}
	if call.Function != nil {
		call.Name = call.Function.Name
		call.Arguments = call.Function.Arguments
	}
	if text, ok := call.Arguments.(string); ok {
		var parsed any
		if json.Unmarshal([]byte(text), &parsed) == nil {
			call.Arguments = parsed
		}
	}
	canonical, _ := json.Marshal(struct {
		Name      string `json:"name"`
		Arguments any    `json:"arguments"`
	}{Name: call.Name, Arguments: call.Arguments})
	return string(canonical)
}
