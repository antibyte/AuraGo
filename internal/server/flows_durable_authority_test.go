package server

import (
	"context"
	"testing"
)

func TestDurableAuthorityChecksLivePolicyAndMissionOwnership(t *testing.T) {
	s := c16StartedServer(t, nil)
	rec := c16PublishedFlow(t, s, greetFlowJSON)
	ctx := context.Background()
	if err := s.Flows.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatal(err)
	}
	base := s.ConfigSnapshot().Clone()
	base.Flows.DurableEnabled = true
	s.cfgSnapshot.Store(base)
	bridge := flowMissionBridge{s: s}
	if enabled, err := bridge.DurableEnabled(ctx); err != nil || !enabled {
		t.Fatalf("enabled = %t, %v", enabled, err)
	}
	if err := bridge.AuthorizeDurable(ctx, rec.ID, rec.MissionID, rec.LiveRevision); err != nil {
		t.Fatalf("valid mission refused: %v", err)
	}
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"durable disabled", func() { c := base.Clone(); c.Flows.DurableEnabled = false; s.cfgSnapshot.Store(c) }},
		{"flows disabled", func() { c := base.Clone(); c.Flows.Enabled = false; s.cfgSnapshot.Store(c) }},
		{"missions disabled", func() { c := base.Clone(); c.Tools.Missions.Enabled = false; s.cfgSnapshot.Store(c) }},
		{"mission readonly", func() { c := base.Clone(); c.Tools.Missions.ReadOnly = true; s.cfgSnapshot.Store(c) }},
		{"desktop readonly", func() { c := base.Clone(); c.VirtualDesktop.ReadOnly = true; s.cfgSnapshot.Store(c) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.change()
			if err := bridge.AuthorizeDurable(ctx, rec.ID, rec.MissionID, rec.LiveRevision); err == nil {
				t.Fatal("revoked policy allowed durable work")
			}
			s.cfgSnapshot.Store(base)
		})
	}
	if err := bridge.AuthorizeDurable(ctx, "flow_wrong", rec.MissionID, rec.LiveRevision); err == nil {
		t.Fatal("foreign flow owner allowed")
	}
	if err := bridge.AuthorizeDurable(ctx, rec.ID, rec.MissionID, 0); err == nil {
		t.Fatal("invalid revision allowed")
	}
	if err := s.MissionManagerV2.SetFlowMissionEnabled(rec.MissionID, rec.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := bridge.AuthorizeDurable(ctx, rec.ID, rec.MissionID, rec.LiveRevision); err == nil {
		t.Fatal("paused mission allowed")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := bridge.DurableAllowed(ctx); err == nil {
		t.Fatal("cancelled authority request allowed")
	}
}
