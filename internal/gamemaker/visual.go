package gamemaker

// Capture data is transient and never included in events, revisions or exports.
type VisualCapture struct {
	Controlled bool   `json:"controlled"`
	Image      string `json:"image"`
	Scenario   string `json:"scenario"`
	At         string `json:"at"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	HUD        string `json:"hud,omitempty"`
}
type VisualFinding struct {
	Image       int     `json:"image"`
	Observation string  `json:"observation"`
	Region      string  `json:"region"`
	Severity    string  `json:"severity"`
	Confidence  float64 `json:"confidence"`
	Suggestion  string  `json:"suggestion"`
}
type VisualReview struct {
	Status   string          `json:"status"`
	Reason   string          `json:"reason,omitempty"`
	Provider string          `json:"provider,omitempty"`
	Model    string          `json:"model,omitempty"`
	BuildID  string          `json:"build_id,omitempty"`
	Findings []VisualFinding `json:"findings,omitempty"`
	// Precheck is local Decisions evidence; it is never a finding.
	Precheck *VisualPrecheck `json:"precheck,omitempty"`
}

// VisualPrecheck is local evidence about a capture. It never becomes a finding
// and never triggers a repair or changes readiness.
type VisualPrecheck struct {
	Canvas     string  `json:"canvas"` // rendered | blank | loading | error_text | unknown
	Confidence float64 `json:"confidence"`
	HUDVisible *bool   `json:"hud_visible,omitempty"`
}
