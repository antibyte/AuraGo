package hass

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/tools"
)

func TestHAHistoryBoundsRangeAndPoints(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	ctx, at := context.Background(), hasstest.DemoTime
	pts, err := a.History(ctx, "sensor.outdoor_temperature", at.Add(-3*time.Hour), at, 100)
	if err != nil || len(pts) != 2 || pts[0].V != 4.0 || pts[1].V != 4.5 || pts[0].T.Location() != time.UTC {
		t.Fatalf("sensor history = %+v %v", pts, err)
	}
	q := ha.LastHistoryQuery()
	for _, part := range []string{"filter_entity_id=sensor.outdoor_temperature", "end_time=2026-11-25T09%3A00%3A00Z", "minimal_response", "no_attributes"} {
		if !strings.Contains(q, part) {
			t.Fatalf("query %q lacks %q", q, part)
		}
	}
	if pts, err = a.History(ctx, "switch.coffee", at.Add(-3*time.Hour), at, 100); err != nil || len(pts) != 2 || pts[0].V != 1 || pts[1].V != 0 {
		t.Fatalf("switch history = %+v %v", pts, err)
	}
	before := ha.HistoryRequests()
	if pts, err = a.History(ctx, "light.kitchen", at.Add(-time.Hour), at, 10); err != nil || len(pts) != 0 || ha.HistoryRequests() != before {
		t.Fatalf("light history = %+v %v (no request expected)", pts, err)
	}
	if pts, _ = a.History(ctx, "sensor.outdoor_temperature", at.Add(-8*24*time.Hour), at, 10); len(pts) != 0 || ha.HistoryRequests() != before {
		t.Fatal("a range over 7 days must not reach Home Assistant")
	}
	if _, err = a.History(ctx, "bad id", at.Add(-time.Hour), at, 10); err == nil {
		t.Fatal("invalid id accepted")
	}
}

func TestHACameraSnapshotSizeCap(t *testing.T) {
	o := hasstest.DemoOptions()
	o.Cameras["camera.big"] = append(slices.Clone(o.Cameras["camera.door"]), make([]byte, 8<<20)...)
	ha := hasstest.New(t, o)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	ctx := context.Background()
	img, ct, err := a.Snapshot(ctx, "camera.door", 640, 360)
	if err != nil || ct != "image/jpeg" || len(img) == 0 {
		t.Fatalf("snapshot = %d bytes %q %v", len(img), ct, err)
	}
	if _, _, err := a.Snapshot(ctx, "camera.big", 640, 360); err == nil {
		t.Fatal("over 8 MiB accepted")
	}
	if _, _, err := a.Snapshot(ctx, "light.kitchen", 640, 360); err == nil {
		t.Fatal("non-camera accepted")
	}
	o2 := hasstest.DemoOptions()
	o2.CameraContentType = "text/html"
	ha2 := hasstest.New(t, o2)
	if _, _, err := newTestAdapter(t, ha2, tools.HAConfig{}).Snapshot(ctx, "camera.door", 640, 360); err == nil {
		t.Fatal("non-image content type accepted")
	}
}

func TestHACameraSnapshotRejectsMismatchedBytes(t *testing.T) {
	o := hasstest.DemoOptions()
	o.Cameras["camera.fake"] = []byte("<html>not an image</html>")
	ha := hasstest.New(t, o)
	if _, _, err := newTestAdapter(t, ha, tools.HAConfig{}).Snapshot(context.Background(), "camera.fake", 0, 0); err == nil {
		t.Fatal("HTML bytes declared as image/jpeg accepted")
	}
}
