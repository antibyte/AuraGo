package detective

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("research case not found")
	ErrConflict = errors.New("research case already has an active run")
	ErrBudget   = errors.New("research budget exhausted")
	ErrClosed   = errors.New("research service is closed")
	// ErrPermission means the live configuration no longer allows research. The run
	// stays resumable and is not counted as a research failure.
	ErrPermission = errors.New("research permission revoked")
	// Export failures keep a stable identity so the API can name the reason with a
	// code the app translates instead of forwarding an English sentence.
	ErrExportFormat   = errors.New("unsupported export format")
	ErrExportTooLarge = errors.New("export exceeds 32 MiB")
	ErrPDFFont        = errors.New("built-in PDF font cannot render")
)

type Profile struct {
	Seconds    int `json:"seconds" yaml:"seconds"`
	Tools      int `json:"tools" yaml:"tools"`
	Iterations int `json:"iterations" yaml:"iterations"`
	Tokens     int `json:"tokens" yaml:"tokens"`
}

func Profiles() map[string]Profile {
	return map[string]Profile{"quick": {300, 40, 60, 0}, "normal": {900, 100, 140, 0}, "maximum": {3600, 250, 320, 0}}
}

func (p Profile) Validate() error {
	if p.Seconds < 30 || p.Seconds > 7200 || p.Tools < 1 || p.Tools > 500 || p.Iterations < p.Tools || p.Iterations > 750 || p.Tokens < 0 {
		return errors.New("invalid research profile")
	}
	return nil
}

type Request struct {
	Topic          string             `json:"topic"`
	Effort         string             `json:"effort"`
	Language       string             `json:"language"`
	Scope          string             `json:"scope"`
	ProviderID     string             `json:"provider_id"`
	Model          string             `json:"model"`
	SourceURLs     []string           `json:"source_urls"`
	PrivateSources []string           `json:"private_sources"`
	AutoUpdate     AutoUpdateSettings `json:"auto_update"`
}

type Usage struct {
	Tools            int   `json:"tools"`
	Iterations       int   `json:"iterations"`
	Requests         int   `json:"requests"`
	PromptTokens     int   `json:"prompt_tokens"`
	CompletionTokens int   `json:"completion_tokens"`
	CachedTokens     int   `json:"cached_tokens"`
	Pages            int   `json:"pages"`
	Bytes            int   `json:"bytes"`
	ActiveMS         int64 `json:"active_ms"`
}

type Run struct {
	ID               string    `json:"id"`
	Key              string    `json:"-"`
	Status           string    `json:"status"`
	Phase            string    `json:"phase"`
	Reason           string    `json:"reason,omitempty"`
	Profile          Profile   `json:"profile"`
	Usage            Usage     `json:"usage"`
	StartedAt        time.Time `json:"started_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Automatic        bool      `json:"automatic,omitempty"`
	BaseRevision     int       `json:"base_revision,omitempty"`
	ReportStyle      string    `json:"report_style,omitempty"`
	NewFindings      []string  `json:"new_findings,omitempty"`
	CheckedSources   []string  `json:"checked_sources,omitempty"`
	SuccessfulChecks int       `json:"successful_checks,omitempty"`
}

type Source struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Locator     string    `json:"locator,omitempty"`
	Title       string    `json:"title"`
	Method      string    `json:"method"`
	Status      string    `json:"status"`
	Excerpt     string    `json:"excerpt"`
	Hash        string    `json:"sha256"`
	RetrievedAt time.Time `json:"retrieved_at"`
}

type Finding struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	SourceID string `json:"source_id"`
	Quote    string `json:"quote"`
	Locator  string `json:"locator,omitempty"`
}

type Block struct {
	Type     string     `json:"type"`
	Text     string     `json:"text,omitempty"`
	Items    []string   `json:"items,omitempty"`
	Rows     [][]string `json:"rows,omitempty"`
	Evidence []string   `json:"evidence,omitempty"`
}

type Report struct {
	Revision     int     `json:"revision"`
	BaseRevision int     `json:"base_revision,omitempty"`
	Title        string  `json:"title"`
	Summary      string  `json:"summary"`
	Blocks       []Block `json:"blocks"`
	Limitations  string  `json:"limitations"`
	Partial      bool    `json:"partial"`
	// Provisional marks the partial report complete() writes when a continue, deepen or
	// update run ends without a full report. It never becomes the default revision.
	Provisional bool      `json:"provisional,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Sources     []Source  `json:"sources"`
	Findings    []Finding `json:"findings"`
}

type Case struct {
	ID                     string     `json:"id"`
	Request                Request    `json:"request"`
	Run                    Run        `json:"run"`
	Plan                   []string   `json:"plan"`
	Answers                []string   `json:"answers"`
	Sources                []Source   `json:"sources"`
	Findings               []Finding  `json:"findings"`
	Reports                []Report   `json:"reports"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
	AutoUpdate             AutoUpdate `json:"auto_update"`
	LatestRevision         int        `json:"latest_revision"`
	LatestCompleteRevision int        `json:"latest_complete_revision,omitempty"`
	SourceCount            int        `json:"source_count"`
	FindingCount           int        `json:"finding_count"`
	LatestSourceID         string     `json:"latest_source_id"`
	pendingSources         []Source
	pendingFindings        []Finding
	pendingReports         []Report
}

// completeRevision names the newest revision backed by a full report. A case written
// before the field existed falls back to its latest revision.
// completeRevision names the newest revision backed by a full report. A case written
// before the field existed derives it from its working reports, or falls back to its
// latest revision when only archived reports remain.
func (c *Case) completeRevision() int {
	if c.LatestCompleteRevision > 0 {
		return c.LatestCompleteRevision
	}
	for i := len(c.Reports) - 1; i >= 0; i-- {
		if !c.Reports[i].Partial && !c.Reports[i].Provisional {
			return c.Reports[i].Revision
		}
	}
	if len(c.Reports) == 0 {
		return c.LatestRevision
	}
	return 0
}

type AutoUpdateSettings struct {
	Enabled       bool   `json:"enabled"`
	IntervalHours int    `json:"interval_hours"`
	ReportStyle   string `json:"report_style"`
	Effort        string `json:"effort"`
}

type AutoUpdate struct {
	AutoUpdateSettings
	NextRunAt     time.Time `json:"next_run_at"`
	LastCheckedAt time.Time `json:"last_checked_at"`
	LastChangedAt time.Time `json:"last_changed_at"`
	Outcome       string    `json:"outcome"`
	Error         string    `json:"error,omitempty"`
}

type Event struct {
	ID     int64     `json:"id"`
	CaseID string    `json:"case_id"`
	Kind   string    `json:"kind"`
	Text   string    `json:"text"`
	At     time.Time `json:"at"`
}

type Continuation struct {
	Provider string          `json:"provider"`
	Model    string          `json:"model"`
	Messages json.RawMessage `json:"messages"`
}
