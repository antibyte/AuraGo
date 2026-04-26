package tools

import (
	"fmt"
	"time"
)

// PreparedMission holds the LLM-generated preparation analysis for a mission.
type PreparedMission struct {
	ID                string               `json:"id"`
	MissionID         string               `json:"mission_id"`
	Version           int                  `json:"version"`
	Status            PreparationStatus    `json:"status"`
	PreparedAt        time.Time            `json:"prepared_at"`
	SourceChecksum    string               `json:"source_checksum"`
	Confidence        float64              `json:"confidence"`
	TokenCost         int                  `json:"token_cost"`
	PreparationTimeMS int64                `json:"preparation_time_ms"`
	ErrorMessage      string               `json:"error_message,omitempty"`
	Analysis          *PreparationAnalysis `json:"analysis,omitempty"`
}

// PreparationStatus represents the state of a mission preparation.
type PreparationStatus string

const (
	PrepStatusNone          PreparationStatus = "none"
	PrepStatusPreparing     PreparationStatus = "preparing"
	PrepStatusPrepared      PreparationStatus = "prepared"
	PrepStatusStale         PreparationStatus = "stale"
	PrepStatusError         PreparationStatus = "error"
	PrepStatusLowConfidence PreparationStatus = "low_confidence"
)

// PreparationAnalysis is the structured LLM output from mission analysis.
type PreparationAnalysis struct {
	Summary        string              `json:"summary"`
	EssentialTools []ToolGuide         `json:"essential_tools"`
	StepPlan       []StepGuide         `json:"step_plan"`
	DecisionPoints []DecisionPoint     `json:"decision_points,omitempty"`
	Pitfalls       []Pitfall           `json:"pitfalls,omitempty"`
	Preloads       []PreloadSuggestion `json:"preloads,omitempty"`
	EstimatedSteps int                 `json:"estimated_steps"`
}

// ToolGuide describes a tool the agent should use during mission execution.
type ToolGuide struct {
	ToolName    string `json:"tool_name"`
	Purpose     string `json:"purpose"`
	SampleInput string `json:"sample_input,omitempty"`
	Order       int    `json:"order"`
}

// StepGuide describes a recommended step in the mission execution plan.
type StepGuide struct {
	Step        int    `json:"step"`
	Action      string `json:"action"`
	Tool        string `json:"tool,omitempty"`
	Expectation string `json:"expectation,omitempty"`
}

// DecisionPoint describes a conditional branch the agent may encounter.
type DecisionPoint struct {
	Condition string `json:"condition"`
	IfTrue    string `json:"if_true"`
	IfFalse   string `json:"if_false"`
}

// Pitfall describes a common mistake or risk during mission execution.
type Pitfall struct {
	Risk       string `json:"risk"`
	Mitigation string `json:"mitigation"`
}

// PreloadSuggestion suggests data the agent should fetch early.
type PreloadSuggestion struct {
	Resource string `json:"resource"`
	Reason   string `json:"reason"`
	Tool     string `json:"tool,omitempty"`
}

// RenderPreparedContext formats the preparation analysis as an advisory markdown block
// that can be appended to the mission prompt.
func (pm *PreparedMission) RenderPreparedContext() string {
	if pm == nil || pm.Analysis == nil || pm.Status != PrepStatusPrepared {
		return ""
	}
	a := pm.Analysis

	var buf []byte
	buf = append(buf, "\n\n---\n## Mission Execution Plan (Advisory)\n"...)
	buf = append(buf, "Scheduler-generated guidance for organizing this mission. Base the final result on actual tool outputs, verified observations, or clearly reported limitations.\n\n"...)

	if a.Summary != "" {
		buf = append(buf, "### Summary\n"...)
		buf = append(buf, a.Summary...)
		buf = append(buf, "\n\n"...)
	}

	if len(a.EssentialTools) > 0 {
		buf = append(buf, "### Essential Tools\n"...)
		for _, t := range a.EssentialTools {
			buf = append(buf, "- **"...)
			buf = append(buf, t.ToolName...)
			buf = append(buf, "**: "...)
			buf = append(buf, t.Purpose...)
			buf = append(buf, '\n')
		}
		buf = append(buf, '\n')
	}

	if len(a.StepPlan) > 0 {
		buf = append(buf, "### Recommended Steps\n"...)
		for _, s := range a.StepPlan {
			buf = append(buf, fmt.Sprintf("%d. %s", s.Step, s.Action)...)
			if s.Tool != "" {
				buf = append(buf, " (tool: "...)
				buf = append(buf, s.Tool...)
				buf = append(buf, ")"...)
			}
			buf = append(buf, '\n')
		}
		buf = append(buf, '\n')
	}

	if len(a.DecisionPoints) > 0 {
		buf = append(buf, "### Decision Points\n"...)
		for _, d := range a.DecisionPoints {
			buf = append(buf, "- **If** "...)
			buf = append(buf, d.Condition...)
			buf = append(buf, " → "...)
			buf = append(buf, d.IfTrue...)
			buf = append(buf, " | **Else** → "...)
			buf = append(buf, d.IfFalse...)
			buf = append(buf, '\n')
		}
		buf = append(buf, '\n')
	}

	if len(a.Pitfalls) > 0 {
		buf = append(buf, "### Pitfalls\n"...)
		for _, p := range a.Pitfalls {
			buf = append(buf, "- ⚠ "...)
			buf = append(buf, p.Risk...)
			buf = append(buf, " → "...)
			buf = append(buf, p.Mitigation...)
			buf = append(buf, '\n')
		}
		buf = append(buf, '\n')
	}

	if len(a.Preloads) > 0 {
		buf = append(buf, "### Suggested Preloads\n"...)
		for _, pl := range a.Preloads {
			buf = append(buf, "- "...)
			buf = append(buf, pl.Resource...)
			buf = append(buf, ": "...)
			buf = append(buf, pl.Reason...)
			buf = append(buf, '\n')
		}
		buf = append(buf, '\n')
	}

	buf = append(buf, "---\n"...)
	return string(buf)
}
