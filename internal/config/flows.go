package config

// FlowsConfig controls EasyDrag flows: visual missions that run on their own engine.
type FlowsConfig struct {
	Enabled                bool             `yaml:"enabled" json:"enabled"`
	MaxParallelRuns        int              `yaml:"max_parallel_runs" json:"max_parallel_runs"`
	MaxParallelNodesPerRun int              `yaml:"max_parallel_nodes_per_run" json:"max_parallel_nodes_per_run"`
	RunRetentionDays       int              `yaml:"run_retention_days" json:"run_retention_days"`
	MaxRunsPerFlow         int              `yaml:"max_runs_per_flow" json:"max_runs_per_flow"`
	AIProvider             string           `yaml:"ai_provider" json:"ai_provider"`
	Agent                  FlowsAgentConfig `yaml:"agent" json:"agent"`
}

// FlowsAgentConfig limits what the agent may do with flows (used from phase 4 on).
type FlowsAgentConfig struct {
	ReadOnly     bool `yaml:"read_only" json:"read_only"`
	AllowPublish bool `yaml:"allow_publish" json:"allow_publish"`
}

// flowsLimit returns def for unset values and clamps the rest to [lo, hi].
func flowsLimit(v, def, lo, hi int) int {
	if v <= 0 {
		return def
	}
	return max(lo, min(v, hi))
}

// EffectiveMaxParallelRuns bounds concurrent flow runs (default 8, 1–32).
func (c FlowsConfig) EffectiveMaxParallelRuns() int { return flowsLimit(c.MaxParallelRuns, 8, 1, 32) }

// EffectiveMaxParallelNodes bounds parallel branches inside one run (default 4, 1–16).
func (c FlowsConfig) EffectiveMaxParallelNodes() int {
	return flowsLimit(c.MaxParallelNodesPerRun, 4, 1, 16)
}

// EffectiveRunRetentionDays keeps finished runs this long (default 30, 1–365).
func (c FlowsConfig) EffectiveRunRetentionDays() int {
	return flowsLimit(c.RunRetentionDays, 30, 1, 365)
}

// EffectiveMaxRunsPerFlow keeps at most this many finished runs per flow (default 200, 10–5000).
func (c FlowsConfig) EffectiveMaxRunsPerFlow() int {
	return flowsLimit(c.MaxRunsPerFlow, 200, 10, 5000)
}
