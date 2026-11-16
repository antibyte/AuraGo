package server

import (
	"context"
	"time"

	"aurago/internal/flows"
)

// addFlowDraftFacts adds the three read-only facts the editor's header shows to the answer of
// GET {flow} (or GET block): "tested" (a test run or simulation of exactly the current draft
// revision succeeded), "changes" (steps and connections that differ from the live version; only
// for a flow that was published) and, when withNextRun is set, "next_run" ({"at": RFC 3339 in
// the server's location, "node": the trigger node}; only when the flow starts by itself).
// rec is a record that was read with its draft and live documents.
func (s *Server) addFlowDraftFacts(ctx context.Context, answer map[string]any, rec *flows.FlowRecord, withNextRun bool) {
	answer["tested"] = s.Flows.IsTested(ctx, rec.ID, rec.DraftRevision)
	if changes, ok := flows.CountChanges(rec.Live, rec.Draft); ok {
		answer["changes"] = changes
	}
	if !withNextRun {
		return
	}
	// time.Local is the location the flow service runs its timers and schedules in (initFlows).
	if next, ok := s.Flows.NextRun(rec.ID); ok {
		answer["next_run"] = map[string]string{"at": next.At.In(time.Local).Format(time.RFC3339), "node": next.NodeID}
	}
}

// addSavedDraftFacts is addFlowDraftFacts for the answer to a save, which carries the new draft
// revision. It reads the stored record again; when another save has moved on meanwhile the
// count would belong to a different revision, so only "tested" (for the revision this save
// made) is given.
func (s *Server) addSavedDraftFacts(ctx context.Context, answer map[string]any, id string, revision int, block bool) {
	read := s.Flows.GetFlow
	if block {
		read = s.Flows.GetBlock
	}
	rec, err := read(ctx, id)
	if err != nil || rec.DraftRevision != revision {
		answer["tested"] = s.Flows.IsTested(ctx, id, revision)
		return
	}
	s.addFlowDraftFacts(ctx, answer, rec, !block)
}
