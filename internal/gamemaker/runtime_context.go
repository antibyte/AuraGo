package gamemaker

import (
	"context"
	"encoding/json"
	"strings"
)

const runtimeContractPrefix = "// AURAGO_RUNTIME_API "

// RuntimeContext describes the helper installed in this job, never a newer
// bundled helper. Authored/legacy files without a descriptor stay explicit.
func (s *Service) RuntimeContext(ctx context.Context, jobID string) map[string]any {
	result := map[string]any{"version": "legacy/custom", "guidance": "Read/search src/common.ts before using helpers; retain its lifecycle."}
	var files []map[string]any
	for _, path := range []string{"src/common.ts", "src/main.ts"} {
		content, err := s.ReadJobFile(ctx, jobID, path)
		if err != nil {
			continue
		}
		lines := strings.Split(content, "\n")
		file := map[string]any{"path": path, "sha256": sourceHash(content), "lines": len(lines)}
		var hooks []SourceMatch
		for i, line := range lines {
			if path == "src/common.ts" && strings.Contains(line, "pickup_events:builder?sceneState.pickup_events:0") {
				result["observation_warnings"] = []map[string]any{{
					"path": path, "line": i + 1, "metric": "pickup_events",
					"issue":  "The installed helper always reports zero pickups in non-scene mode (config.objects). Scene-builder counters are separate.",
					"repair": "For non-scene games, count actual item/cargo/flight-goal collections, clear that counter on reset, and return it from snapshot(). Preserve the installed gameplay and lifecycle; never substitute score, combat hits or feedback events for pickups.",
				}}
			}
			if path == "src/common.ts" && i < 8 && strings.HasPrefix(line, runtimeContractPrefix) && len(line) <= 3000 {
				var descriptor map[string]any
				if json.Unmarshal([]byte(strings.TrimPrefix(line, runtimeContractPrefix)), &descriptor) == nil && len(descriptor) <= 8 {
					result["api"] = descriptor
					result["version"] = descriptor["version"]
				}
			}
			for _, hook := range []string{"setup(", "step(", "action(", "reset(", "startGame(", "class GameScene"} {
				if !strings.HasPrefix(strings.TrimSpace(line), "//") && strings.Contains(line, hook) && len(hooks) < 8 {
					runes := []rune(strings.TrimSpace(line))
					hooks = append(hooks, SourceMatch{Line: i + 1, Text: string(runes[:min(180, len(runes))])})
					break
				}
			}
		}
		file["hooks"] = hooks
		files = append(files, file)
	}
	result["files"] = files
	return result
}
