package selftest

import "time"

type Capability struct {
	Name       string              `json:"name"`
	Kind       string              `json:"kind"`
	Instance   string              `json:"instance"`
	Enabled    bool                `json:"enabled"`
	Callable   bool                `json:"callable"`
	Status     string              `json:"status"`
	Operations map[string][]string `json:"operations,omitempty"`
	SchemaHash string              `json:"schema_hash"`
}

type Request struct {
	Bindings       []Binding `json:"bindings,omitempty"`
	Profile        string    `json:"profile"`
	Targets        []string  `json:"targets,omitempty"`
	CaseIDs        []string  `json:"case_ids,omitempty"`
	TimeoutSeconds int       `json:"timeout_seconds,omitempty"`
	TokenLimit     int       `json:"token_limit,omitempty"`
}

type BoundCall struct {
	Arguments map[string]any `json:"arguments"`
}

// Bindings are human-supplied exact calls, held in memory only. They never
// authorize a disabled tool or override the application's normal action policy.
type Binding struct {
	CaseID       string         `json:"case_id"`
	Tool         string         `json:"tool"`
	Arguments    map[string]any `json:"arguments"`
	AllowEffects bool           `json:"allow_effects"`
	Verify       *BoundCall     `json:"verify,omitempty"`
	Expected     map[string]any `json:"expected"`
	Cleanup      *BoundCall     `json:"cleanup,omitempty"`
}

type Evidence struct {
	Executed      bool   `json:"executed"`
	FixtureID     string `json:"fixture_id,omitempty"`
	ID            string `json:"id"`
	Tool          string `json:"tool"`
	Operation     string `json:"operation,omitempty"`
	Status        string `json:"status"`
	ArgumentsHash string `json:"arguments_hash"`
	OutputHash    string `json:"output_hash"`
	DurationMS    int64  `json:"duration_ms"`
}

type Case struct {
	ID         string     `json:"id"`
	TemplateID string     `json:"template_id"`
	Tool       string     `json:"tool"`
	Instance   string     `json:"instance"`
	Profile    string     `json:"profile"`
	Prompt     string     `json:"prompt"`
	Verdict    string     `json:"verdict"`
	Code       string     `json:"code,omitempty"`
	Operation  string     `json:"operation,omitempty"`
	Assertions []string   `json:"assertions,omitempty"`
	Evidence   []Evidence `json:"evidence,omitempty"`
	Tokens     int        `json:"tokens"`
	DurationMS int64      `json:"duration_ms"`
	Cleanup    string     `json:"cleanup"`
}

type Summary struct {
	EnabledIntegrations  int            `json:"enabled_integrations"`
	ExecutedIntegrations int            `json:"executed_integrations"`
	EnabledTools         int            `json:"enabled_tools"`
	ExecutedTools        int            `json:"executed_tools"`
	EnabledOperations    int            `json:"enabled_operations"`
	ExecutedOperations   int            `json:"executed_operations"`
	Cases                int            `json:"cases"`
	Verdicts             map[string]int `json:"verdicts"`
	Tokens               int            `json:"tokens"`
	CostStatus           string         `json:"cost_status"`
}

type Run struct {
	ID        string       `json:"id"`
	Revision  int          `json:"revision"`
	Status    string       `json:"status"`
	Started   time.Time    `json:"started"`
	Finished  *time.Time   `json:"finished,omitempty"`
	Request   Request      `json:"request"`
	Inventory []Capability `json:"inventory"`
	Cases     []Case       `json:"cases"`
	Summary   Summary      `json:"summary"`
}

// Execution never carries model prose or raw tool payloads into persistent reports.
type Execution struct {
	Verdict    string
	Code       string
	Operation  string
	Assertions []string
	Evidence   []Evidence
	Tokens     int
	Cleanup    string
}
