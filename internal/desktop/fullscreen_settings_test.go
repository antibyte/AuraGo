package desktop

import (
	"context"
	"testing"
)

func TestServiceFullscreenSetting(t *testing.T) {
	t.Parallel()
	svc := testService(t)
	ctx := context.Background()
	const key = "desktop.open_fullscreen"
	check := func(want string) {
		t.Helper()
		boot, err := svc.Bootstrap(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got := boot.Settings[key]; got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	check("false")
	for _, value := range []string{"true", "false"} {
		if err := svc.SetSettings(ctx, map[string]string{key: value}, SourceUser); err != nil {
			t.Fatal(err)
		}
		check(value)
	}
	if err := svc.SetSetting(ctx, key, "always", SourceUser); err == nil {
		t.Fatal("accepted a non-boolean fullscreen preference")
	}
	check("false")
}
