package server

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/planner"
	"aurago/internal/tools"
)

func TestMissionFailureCancellationPreservesOperationalIssues(t *testing.T) {
	for _, reason := range []string{"shutdown", "owner_revoked", "user", "deadline", "transport"} {
		t.Run(reason, func(t *testing.T) {
			allowMissionMutationsForTest(t)
			t.Cleanup(tools.ClearRuntimePermissionsForTest)
			db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			manager := tools.NewMissionManagerV2(t.TempDir(), nil)
			defer manager.Stop()
			const id = "mission-cancellation"
			if err := manager.Create(&tools.MissionV2{ID: id, Name: "News", Enabled: true, ExecutionType: tools.ExecutionManual}); err != nil {
				t.Fatal(err)
			}
			if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{Source: "mission", Context: id, Title: "Previous failure", Detail: "connection failed", Severity: "error", Fingerprint: "mission|" + id}); err != nil {
				t.Fatal(err)
			}
			s := &Server{PlannerDB: db, MissionManagerV2: manager, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			ctx := context.Background()
			if reason == "shutdown" || reason == "owner_revoked" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			} else if reason == "deadline" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Unix(1, 0))
				defer cancel()
			} else if reason == "user" {
				_, release := s.missionRunTracker().begin(id)
				defer release()
				s.missionRunTracker().cancel(id)
			}
			started := make(chan struct{})
			manager.SetCallback(func(_, _ string) { close(started) })
			if err := manager.Start(); err != nil {
				t.Fatal(err)
			}
			if err := manager.RunNow(id); err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("mission did not start")
			}
			s.setMissionError(ctx, id, "", "context canceled")
			var occurrences int
			var status string
			if err := db.QueryRow(`SELECT occurrences,status FROM operational_issues WHERE fingerprint=?`, "mission|"+id).Scan(&occurrences, &status); err != nil {
				t.Fatal(err)
			}
			want := 1
			if reason == "deadline" || reason == "transport" {
				want = 2
			}
			if occurrences != want || status != "open" {
				t.Fatalf("issue occurrences=%d status=%s, want %d/open", occurrences, status, want)
			}
			mission, _ := manager.Get(id)
			if mission.LastResult != tools.MissionResultError {
				t.Fatalf("interrupted or failed run reported %q", mission.LastResult)
			}
			if reason == "user" && mission.LastOutput != tools.MissionCancelledOutput {
				t.Fatalf("user cancellation output=%q", mission.LastOutput)
			}
			if (reason == "shutdown" || reason == "owner_revoked") && mission.LastOutput == tools.MissionCancelledOutput {
				t.Fatal("owner cancellation was attributed to the user")
			}
		})
	}
}
