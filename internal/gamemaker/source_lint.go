package gamemaker

import (
	"context"
	"fmt"
	"strings"
)

// LintFinding is one deterministic structural observation about generated
// source. Lint is advisory: it never blocks a write and never feeds validation
// or publication.
type LintFinding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
}

const (
	lintMaxFindings = 5
	lintMaxRoleScan = 8
)

// LintSource checks the structural contract of src/main.ts against the accepted
// plan. The rules are intentionally conservative: only facts that are certainly
// wrong for every supported helper.
func LintSource(plan *GamePlan, source string) []LintFinding {
	if plan == nil || strings.TrimSpace(source) == "" {
		return nil
	}
	findings := make([]LintFinding, 0, lintMaxFindings)
	add := func(code, message string, line int) {
		if len(findings) < lintMaxFindings {
			findings = append(findings, LintFinding{Code: code, Message: message, Line: line})
		}
	}
	lines := strings.Split(source, "\n")
	if !hasCodeToken(lines, "setup(") {
		add("missing_setup", "src/main.ts defines no setup(); the installed helper calls it and the validator binds there.", 0)
	}
	if !hasCodeToken(lines, "this.player") {
		add("missing_player", "assign this.player in setup(); the validator reads it.", 0)
	}
	if len(plan.Stages) >= 2 && !hasCodeToken(lines, "configureLevels") && !hasCodeToken(lines, "levels:") {
		add("missing_levels", "the accepted plan declares multiple stages; configureLevels()/config.levels must implement them.", 0)
	}
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		if strings.Contains(trimmed, "http://") || strings.Contains(trimmed, "https://") {
			add("external_resource", "generated games load only local files; remove the external URL.", index+1)
			break
		}
	}
	scanned := 0
	for _, asset := range plan.Assets {
		if asset.Role == "" || len(findings) >= lintMaxFindings || scanned >= lintMaxRoleScan {
			continue
		}
		scanned++
		if !strings.Contains(source, asset.Role) {
			add("unused_role", fmt.Sprintf("planned role %q never appears in src/main.ts", asset.Role), 0)
		}
	}
	return findings
}

// LintJobSource lints the current src/main.ts against the stored plan. Missing
// files or plans stay silent so a lint check can never fail a tool call.
func (s *Service) LintJobSource(ctx context.Context, jobID string) []LintFinding {
	if s == nil {
		return nil
	}
	plan, err := s.GetPlan(ctx, jobID)
	if err != nil || plan == nil {
		return nil
	}
	source, err := s.ReadJobFile(ctx, jobID, "src/main.ts")
	if err != nil {
		return nil
	}
	return LintSource(plan, source)
}

func hasCodeToken(lines []string, token string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		if strings.Contains(trimmed, token) {
			return true
		}
	}
	return false
}
