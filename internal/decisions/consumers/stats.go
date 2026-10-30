package consumers

type Stats struct {
	Attempts      int     `json:"attempts"`
	Successes     int     `json:"successes"`
	Failures      int     `json:"failures"`
	Uncertain     int     `json:"uncertain"`
	Deferred      int     `json:"deferred"`
	Preempted     int     `json:"preempted"`
	Timeouts      int     `json:"timeouts"`
	ImagesSkipped int     `json:"images_skipped"`
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	CostUSD       float64 `json:"cost_usd"`
	CostMissing   int     `json:"cost_missing"`
	LastCode      string  `json:"last_code,omitempty"`
}
