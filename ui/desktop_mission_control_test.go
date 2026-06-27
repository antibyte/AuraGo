package ui

import (
	"strings"
	"testing"
)

func TestDesktopMainBundleIncludesCheaterStartMenuRuntime(t *testing.T) {
	t.Parallel()

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, marker := range []string{
		"cheater: 'CheaterApp'",
		"appId === 'cheater'",
		"'mission-control': 'workflow'",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop main bundle missing runtime marker %q", marker)
		}
	}
}

func TestDesktopMissionControlNormalizesListPayload(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/mission-control.js")
	for _, marker := range []string{
		"function normalizeMissionControlPayload(data)",
		"function normalizeMissionQueue(queue)",
		"function missionIsRunning(mission",
		"function getRunningMissions(",
		"function toastForMissionDispatch(data)",
		"Array.isArray(data.missions)",
		"Array.isArray(data.data)",
		"Array.isArray(data)",
		"state.missions = normalized.missions",
		"state.queue = normalized.queue",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("mission-control.js missing payload normalizer marker %q", marker)
		}
	}
}
