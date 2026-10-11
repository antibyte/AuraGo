package server

import (
	"testing"

	"aurago/internal/retrogames/retrogamestest"
	"aurago/internal/tools"
)

func TestPublishAndWithdrawRetroGamesTool(t *testing.T) {
	svc := retrogamestest.NewService(t, &retrogamestest.Fetcher{})
	publishRetroGamesTool(svc)
	t.Cleanup(func() { tools.SetRetroGamesService(nil) })
	if !tools.RetroGamesAvailable() {
		t.Fatal("published service must be available to the tool")
	}
	withdrawRetroGamesTool(svc)
	if tools.RetroGamesAvailable() {
		t.Fatal("withdrawn service must not be available")
	}
	withdrawRetroGamesTool(nil)
}
