package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

func TestFB4FlowStartErrorMarksAFullQueue(t *testing.T) {
	err := flowStartError(flows.ErrRunnerQueueFull)
	if !errors.Is(err, tools.ErrFlowQueueFull) || !errors.Is(err, flows.ErrQueueFull) || missionErrorStatus(err) != http.StatusTooManyRequests {
		t.Fatalf("flowStartError = %v", err)
	}
	other := errors.New("x")
	if flowStartError(other) != other {
		t.Fatal("other errors must pass unchanged")
	}
}

func TestFB4MissionControlRunNowReportsASkippedFlowStart(t *testing.T) {
	allowMissionMutationsForTest(t)
	s := c16StartedServer(t, nil)
	doc := strings.Replace(waitFlowJSON, `"name":"Pause",`, `"name":"Pause","settings":{"concurrency":"skip"},`, 1)
	rec := c16PublishedFlow(t, s, doc)
	if err := s.Flows.SetEnabled(context.Background(), rec.ID, true); err != nil {
		t.Fatal(err)
	}
	first, err := s.Flows.RunNow(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Flows.CancelRun(first.RunID)
	c16WaitStatus(t, s, rec.MissionID, tools.MissionStatusRunning)
	var skipped *tools.MissionTriggerSkippedError
	if err := s.MissionManagerV2.RunNow(rec.MissionID); !errors.As(err, &skipped) || skipped.Reason != tools.FlowStartSkippedReason {
		t.Fatalf("Mission Control Run of a busy skip flow = %v", err)
	}
}
