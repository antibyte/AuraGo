package cyd

import (
	"testing"
	"time"
)

type fakeWS struct {
	msgs []any
}

func (f *fakeWS) SendJSON(v any) error {
	f.msgs = append(f.msgs, v)
	return nil
}

func TestNotifyReplacesLowerPriority(t *testing.T) {
	h := NewHub()
	first := h.Notify("a", "normal body", "normal", 30)
	if first.ID == "" {
		t.Fatal("expected notify id")
	}
	second := h.Notify("b", "critical body", "critical", 60)
	snap := h.Snapshot()
	if snap.Notify == nil || snap.Notify.ID != second.ID {
		t.Fatalf("expected critical overlay, got %+v", snap.Notify)
	}
	third := h.Notify("c", "normal again", "normal", 30)
	snap = h.Snapshot()
	if snap.Notify == nil || snap.Notify.ID != second.ID {
		t.Fatalf("critical should not yield to normal, kept %+v got %+v", second, third)
	}
}

func TestAckClearsOverlay(t *testing.T) {
	h := NewHub()
	n := h.Notify("title", "body", "high", 30)
	h.Ack(n.ID)
	if h.Snapshot().Notify != nil {
		t.Fatal("expected overlay cleared")
	}
}

func TestHeartbeatAndRecentDevice(t *testing.T) {
	h := NewHub()
	if h.HasRecentDevice(time.Minute) {
		t.Fatal("expected no device")
	}
	h.Heartbeat("tok1", "CYD kitchen", Heartbeat{Firmware: "0.1.0", RSSI: -40})
	if !h.HasRecentDevice(time.Minute) {
		t.Fatal("expected recent device")
	}
	devs := h.Devices()
	if len(devs) != 1 || devs[0].Firmware != "0.1.0" {
		t.Fatalf("devices = %+v", devs)
	}
}

func TestBroadcastNotify(t *testing.T) {
	h := NewHub()
	ws := &fakeWS{}
	h.AddClient("tok1", ws)
	h.Notify("hi", "there", "normal", 10)
	if len(ws.msgs) != 1 {
		t.Fatalf("got %d ws messages", len(ws.msgs))
	}
}

func TestSetPageNames(t *testing.T) {
	if NormalizePage("LOAD") != "load" {
		t.Fatalf("load: %s", NormalizePage("LOAD"))
	}
	if NormalizePage("work") != "work" {
		t.Fatalf("work: %s", NormalizePage("work"))
	}
	if NormalizePage("host") != "host" {
		t.Fatalf("host: %s", NormalizePage("host"))
	}
	if NormalizePage("home") != "status" {
		t.Fatalf("home: %s", NormalizePage("home"))
	}
	if NormalizePage("nope") != "status" {
		t.Fatalf("unknown: %s", NormalizePage("nope"))
	}
	h := NewHub()
	h.SetPage("work")
	if h.Snapshot().Display.Page != "work" {
		t.Fatalf("snapshot page = %s", h.Snapshot().Display.Page)
	}
}

func TestBuildSnapshotTruncates(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz0123456789 extra"
	snap := BuildSnapshot(Inputs{Task: long, Model: long, Busy: true}, nil)
	if len(snap.Agent.Task) > TaskMax {
		t.Fatalf("task len %d", len(snap.Agent.Task))
	}
	if snap.Display.LED != "yellow" {
		t.Fatalf("led = %s", snap.Display.LED)
	}
}
