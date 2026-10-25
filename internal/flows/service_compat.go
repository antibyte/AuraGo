package flows

import (
	"context"
	"strings"
)

// LiveIssuesNoter is an optional extension of MissionBridge for CheckLiveRevisions: it shows on
// the flow's mission that its live revision no longer passes the current checks. It follows the
// bridge rule (no Service method that takes a flow lock).
type LiveIssuesNoter interface {
	NoteFlowLiveIssues(missionID string, issues []Issue)
	// ClearFlowLiveIssues takes that note away again: the flow was published again, or its live
	// revision passes the checks. It is a no-op when the mission shows no such note.
	ClearFlowLiveIssues(missionID string)
}

// LiveIssueReport names a flow whose live revision no longer passes the current checks.
type LiveIssueReport struct {
	FlowID    string
	MissionID string
	Revision  int
	Issues    []Issue
}

// liveCompat is what CheckLiveRevisions found for one flow: the error issues of its live
// revision, which was revision.
type liveCompat struct {
	revision int
	issues   []Issue
}

// liveIssueSkipped are the codes the compatibility check leaves out: they describe the tool
// configuration at the moment of the check (a tool switched off, a generic tool gone), not a rule
// the revision breaks, and publishing again would not fix them.
var liveIssueSkipped = map[string]bool{IssueNodeUnavailable: true, IssueNodeTypeUnknown: true}

// CheckLiveRevisions validates every live revision against the publish rules of this build and
// reports the flows whose live revision has errors now (the server runs it once after Start and
// ReconcileMissions). A tightened rule otherwise breaks published flows at run time without a
// warning. Each flagged flow is logged at Warn and noted on its mission through the optional
// LiveIssuesNoter; the result is kept per flow and revision for LiveIssues. Visited flows replace
// their entry, others keep theirs, so a pass that saw fewer flows erases nothing. It takes no flow
// lock and only reads the store; Shutdown waits for it. It returns ErrRunnerClosed after
// Shutdown, ctx's error when ctx ends (with what it found so far kept) and the store's error when
// the flows cannot be listed.
func (s *Service) CheckLiveRevisions(ctx context.Context) ([]LiveIssueReport, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrRunnerClosed
	}
	s.reconciling.Add(1)
	s.mu.Unlock()
	defer s.reconciling.Done()

	records, err := s.store.ListFlows(ctx, KindFlow)
	if err != nil {
		return nil, s.reconcileStopped(err)
	}
	noter, _ := s.bridge.(LiveIssuesNoter)
	visited := map[string]*liveCompat{}
	var reports []LiveIssueReport
	var stop error
	for _, rec := range records {
		if stop = ctx.Err(); stop != nil {
			break
		}
		if s.isClosed() {
			stop = ErrRunnerClosed
			break
		}
		if rec.Live == nil {
			visited[rec.ID] = nil
			continue
		}
		issues := s.liveRevisionErrors(rec)
		if len(issues) == 0 {
			visited[rec.ID] = nil
			if noter != nil && rec.MissionID != "" {
				noter.ClearFlowLiveIssues(rec.MissionID)
			}
			continue
		}
		// Published again while the check ran: the finding belongs to a revision that is gone.
		if cur, err := s.store.GetFlow(ctx, rec.ID); err == nil && cur.LiveRevision != rec.LiveRevision {
			continue
		}
		visited[rec.ID] = &liveCompat{revision: rec.LiveRevision, issues: issues}
		reports = append(reports, LiveIssueReport{FlowID: rec.ID, MissionID: rec.MissionID, Revision: rec.LiveRevision, Issues: issues})
		s.logger.Warn("the published version of a flow no longer passes the current checks; its runs may fail until it is opened in EasyDrag and published again",
			"flow", rec.ID, "revision", rec.LiveRevision, "codes", liveIssueCodes(issues))
		if noter != nil && rec.MissionID != "" {
			noter.NoteFlowLiveIssues(rec.MissionID, issues)
		}
	}
	s.mu.Lock()
	if s.liveIssues == nil {
		s.liveIssues = map[string]liveCompat{}
	}
	for id, c := range visited {
		if c == nil {
			delete(s.liveIssues, id)
		} else {
			s.liveIssues[id] = *c
		}
	}
	s.mu.Unlock()
	return reports, stop
}

// liveRevisionErrors validates rec's live revision with the publish rules of this build at the
// time it was published (PublishedAt; now when unknown), so a one-off date that has passed since
// counts as it did then, plus the self-trigger rule Publish adds. It keeps the errors, without
// the codes in liveIssueSkipped.
func (s *Service) liveRevisionErrors(rec *FlowRecord) []Issue {
	at := rec.PublishedAt
	if at.IsZero() {
		at = s.now()
	}
	all := append(Validate(rec.Live, s.reg, ValidateContext{Mode: ModePublish, Now: at, Location: s.services.Loc()}),
		selfTriggerIssues(rec.Live, rec.MissionID)...)
	var out []Issue
	for _, is := range all {
		if is.Severity == SeverityError && !liveIssueSkipped[is.Code] {
			out = append(out, is)
		}
	}
	return out
}

// LiveIssues returns what CheckLiveRevisions found for rec's live revision: its error issues
// under the current rules; nil when it found none, has not run, or the flow was published again
// since (a publication passes the current rules). The result is a copy.
func (s *Service) LiveIssues(rec *FlowRecord) []Issue {
	if rec == nil || rec.Live == nil {
		return nil
	}
	s.mu.Lock()
	c, ok := s.liveIssues[rec.ID]
	s.mu.Unlock()
	if !ok || c.revision != rec.LiveRevision {
		return nil
	}
	return append([]Issue(nil), c.issues...)
}

// liveIssueCodes lists the distinct codes of issues in order, for a log line.
func liveIssueCodes(issues []Issue) string {
	seen := map[string]bool{}
	var codes []string
	for _, is := range issues {
		if !seen[is.Code] {
			seen[is.Code] = true
			codes = append(codes, is.Code)
		}
	}
	return strings.Join(codes, ",")
}
