package detective

// CaseSummary is the bounded projection that the live view and the case list read.
// It lives in detective_cases.summary next to the body, so polling never has to parse
// the evidence body. caseSummary is the single implementation: the schema migration
// backfills every row through it and a test asserts the stored value equals the
// projection of the loaded case.
type CaseSummary struct {
	Status         string     `json:"status"`
	Phase          string     `json:"phase"`
	Reason         string     `json:"reason,omitempty"`
	Effort         string     `json:"effort"`
	Topic          string     `json:"topic"`
	Usage          Usage      `json:"usage"`
	Profile        Profile    `json:"profile"`
	Sources        int        `json:"sources"`
	Findings       int        `json:"findings"`
	LatestRevision int        `json:"latest_revision"`
	LatestSourceID string     `json:"latest_source_id,omitempty"`
	AutoUpdate     AutoUpdate `json:"auto_update"`
}

func caseSummary(c *Case) CaseSummary {
	sources, findings := c.SourceCount, c.FindingCount
	if len(c.Sources) > sources {
		sources = len(c.Sources)
	}
	if len(c.Findings) > findings {
		findings = len(c.Findings)
	}
	latestSource := c.LatestSourceID
	if latestSource == "" && len(c.Sources) > 0 {
		latestSource = c.Sources[len(c.Sources)-1].ID
	}
	return CaseSummary{
		Status:         c.Run.Status,
		Phase:          c.Run.Phase,
		Reason:         c.Run.Reason,
		Effort:         c.Request.Effort,
		Topic:          c.Request.Topic,
		Usage:          c.Run.Usage,
		Profile:        c.Run.Profile,
		Sources:        sources,
		Findings:       findings,
		LatestRevision: c.LatestRevision,
		LatestSourceID: latestSource,
		AutoUpdate:     c.AutoUpdate,
	}
}

// liveView projects a summary row into the live view. The active_ms column still wins
// over the JSON value, so a lost heartbeat cannot shorten the recorded time.
func liveView(id string, activeMS int64, sum CaseSummary) LiveView {
	usage := sum.Usage
	if activeMS > usage.ActiveMS {
		usage.ActiveMS = activeMS
	}
	return LiveView{
		ID:             id,
		Status:         sum.Status,
		Phase:          sum.Phase,
		Reason:         sum.Reason,
		Effort:         sum.Effort,
		Topic:          sum.Topic,
		Usage:          usage,
		Profile:        sum.Profile,
		Sources:        sum.Sources,
		Findings:       sum.Findings,
		LatestRevision: sum.LatestRevision,
		LatestSourceID: sum.LatestSourceID,
		AutoUpdate:     sum.AutoUpdate,
	}
}

// summaryCase rebuilds the bounded case shape the list endpoint returns. Only the
// request fields the Desktop app reads are included; evidence stays in the archive.
func summaryCase(id string, activeMS int64, sum CaseSummary) Case {
	usage := sum.Usage
	if activeMS > usage.ActiveMS {
		usage.ActiveMS = activeMS
	}
	return Case{
		ID:             id,
		Request:        Request{Topic: sum.Topic, Effort: sum.Effort, AutoUpdate: sum.AutoUpdate.AutoUpdateSettings},
		Run:            Run{Status: sum.Status, Phase: sum.Phase, Reason: sum.Reason, Usage: usage, Profile: sum.Profile},
		AutoUpdate:     sum.AutoUpdate,
		LatestRevision: sum.LatestRevision,
		SourceCount:    sum.Sources,
		FindingCount:   sum.Findings,
		LatestSourceID: sum.LatestSourceID,
	}
}
