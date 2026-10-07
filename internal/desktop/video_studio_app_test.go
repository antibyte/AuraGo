package desktop

import (
	"context"
	"testing"
)

func TestVideoStudioStartMenuBootstrap(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	check := func(visible bool) {
		t.Helper()
		payload, err := svc.Bootstrap(ctx)
		if err != nil {
			t.Fatal(err)
		}
		app := testFindApp(t, payload.BuiltinApps, "video-studio")
		if app.StartVisible != visible || app.Category != "creative" || app.Entry != "builtin://video-studio" {
			t.Fatalf("unexpected Video Studio launcher: %+v", app)
		}
	}
	// A fresh profile exposes the app even without optional host capabilities.
	check(true)
	// User visibility choices survive a bootstrap cache refresh.
	for _, visible := range []bool{false, true} {
		if err := svc.SetAppVisibility(ctx, "video-studio", nil, &visible, "user"); err != nil {
			t.Fatal(err)
		}
		check(visible)
	}
}
