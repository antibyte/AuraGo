package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/tools"
)

// newFlowsCronTestServer is newFlowsTestServer with a started cron engine, so schedule
// triggers register their jobs.
func newFlowsCronTestServer(t *testing.T) *Server {
	t.Helper()
	s, _, _ := testDesktopPermissionServer(t)
	dir := t.TempDir()
	s.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s.Cfg.Flows.Enabled = true
	s.Cfg.Tools.Missions.Enabled = true
	s.Cfg.Directories.DataDir = dir
	s.Cfg.Directories.WorkspaceDir = filepath.Join(dir, "workspace")
	s.Cfg.Directories.ToolsDir = filepath.Join(dir, "tools")
	s.Cfg.Directories.SkillsDir = filepath.Join(dir, "skills")
	s.Cfg.SQLite.GameMakerPath = filepath.Join(dir, "game_maker.db")
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{SchedulerEnabled: true, MissionsEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	cronMgr := tools.NewCronManager(dir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	t.Cleanup(func() { _ = cronMgr.Close() })
	s.CronManager = cronMgr
	s.MissionManagerV2 = tools.NewMissionManagerV2(dir, cronMgr)
	s.initFlows()
	if s.Flows == nil {
		t.Fatal("flows were not initialised")
	}
	if err := s.MissionManagerV2.Start(); err != nil {
		t.Fatalf("missions: %v", err)
	}
	t.Cleanup(s.MissionManagerV2.Stop)
	if err := flowsStart(context.Background(), s.Flows); err != nil {
		t.Fatalf("flows: %v", err)
	}
	s.afterFlowsStart(context.Background(), s.Flows, s.Logger)
	t.Cleanup(func() { s.shutdownFlows(context.Background()) })
	return s
}

// timeFlowJSON is a flow with an every-six-hours schedule trigger (node n_aaaaaaaa) and/or a
// Date/Time trigger at the given time (node n_cccccccc), both feeding one set step.
func timeFlowJSON(withSchedule bool, at time.Time) string {
	nodes := `{"id":"n_bbbbbbbb","key":"greet","type":"logic.set","type_version":1,"label":"Gruss","position":{"x":300,"y":0},"params":{"fields":[{"name":"x","value":"1"}]}}`
	edges := ""
	if withSchedule {
		nodes += `,{"id":"n_aaaaaaaa","key":"takt","type":"trigger.schedule","type_version":1,"label":"Takt","position":{"x":0,"y":0},"params":{"mode":"interval_hours","hours":6}}`
		edges += `{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}`
	}
	if !at.IsZero() {
		nodes += fmt.Sprintf(`,{"id":"n_cccccccc","key":"wann","type":"trigger.datetime","type_version":1,"label":"Wann","position":{"x":0,"y":120},"params":{"at":%q}}`, at.Format(time.RFC3339))
		if edges != "" {
			edges += ","
		}
		edges += `{"id":"e_cccccccc","source":{"node":"n_cccccccc","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}`
	}
	return `{"schema":1,"name":"Zeit","nodes":[` + nodes + `],"edges":[` + edges + `]}`
}

// Mission Control's next run and the flow's NextRun are one function: they agree for a
// schedule, a Date/Time trigger and both, and both are empty while the flow is paused.
func TestMissionControlNextRunEqualsTheFlowNextRun(t *testing.T) {
	s := newFlowsCronTestServer(t)
	ctx := context.Background()
	at := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	cases := []struct {
		name     string
		schedule bool
		datetime time.Time
	}{
		{"schedule only", true, time.Time{}},
		{"date and time only", false, at},
		{"both", true, at},
	}
	for _, c := range cases {
		rec := createTestFlow(t, s, timeFlowJSON(c.schedule, c.datetime))
		if _, _, err := s.Flows.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
			t.Fatalf("%s: Publish: %v", c.name, err)
		}
		if got, ok := s.MissionManagerV2.NextRun(rec.MissionID); ok {
			t.Fatalf("%s: a flow that is switched off has a Mission Control next run %v", c.name, got)
		}
		if err := s.Flows.SetEnabled(ctx, rec.ID, true); err != nil {
			t.Fatalf("%s: SetEnabled: %v", c.name, err)
		}
		flowNext, ok := s.Flows.NextRun(rec.ID)
		if !ok {
			t.Fatalf("%s: NextRun has no next run", c.name)
		}
		missionNext, ok := s.MissionManagerV2.NextRun(rec.MissionID)
		if !ok || !missionNext.Equal(flowNext.At) {
			t.Fatalf("%s: Mission Control = %v %v, NextRun = %v", c.name, missionNext, ok, flowNext.At)
		}
		// The shared answer is the earlier of the two sources.
		var want time.Time
		wantNode := ""
		if c.schedule {
			job, ok := s.CronManager.NextRun("mission_" + rec.MissionID + "__n_aaaaaaaa")
			if !ok {
				t.Fatalf("%s: the cron engine has no job for the schedule trigger", c.name)
			}
			want, wantNode = job, "n_aaaaaaaa"
		}
		if !c.datetime.IsZero() && (want.IsZero() || c.datetime.Before(want)) {
			want, wantNode = c.datetime, "n_cccccccc"
		}
		if !flowNext.At.Equal(want) || flowNext.NodeID != wantNode {
			t.Fatalf("%s: NextRun = %v at %s, want %v at %s", c.name, flowNext.At, flowNext.NodeID, want, wantNode)
		}
		if err := s.Flows.SetEnabled(ctx, rec.ID, false); err != nil {
			t.Fatalf("%s: SetEnabled(false): %v", c.name, err)
		}
		if got, ok := s.Flows.NextRun(rec.ID); ok {
			t.Fatalf("%s: a paused flow has a next run %+v", c.name, got)
		}
		if got, ok := s.MissionManagerV2.NextRun(rec.MissionID); ok {
			t.Fatalf("%s: a paused flow has a Mission Control next run %v", c.name, got)
		}
	}
}
