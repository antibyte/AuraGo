package gamemaker

import (
	"context"
	"strings"
)

const runtimeContractPrefix = "// AURAGO_RUNTIME_API "

func (s *Service) FrameworkReference(ctx context.Context, jobID, framework string) map[string]any {
	versions := map[string]string{"phaser": "phaser-2", "three": "three-4", "voxel": "voxel-1"}
	if version, ok := versions[framework]; ok {
		return map[string]any{"framework": framework, "bundled_helper_version": version, "api_reference": runtimeReferences[version].Text, "guidance": "Optional bundled helper reference. use_template installs a starter explicitly; inspect existing local code before using these APIs."}
	}
	result := s.RuntimeContext(ctx, jobID)
	result["available_frameworks"] = []string{"phaser", "three", "voxel"}
	return result
}

// RuntimeContext locates authored source; optional bundled APIs are fetched explicitly.
func (s *Service) RuntimeContext(ctx context.Context, jobID string) map[string]any {
	result := map[string]any{"version": "legacy/custom", "guidance": "Read the installed local helper at the supplied locations before using its APIs. Frameworks and hooks are optional."}
	var files []map[string]any
	for _, path := range []string{"src/common.ts", "src/main.ts", "src/voxel.json"} {
		content, err := s.ReadJobFile(ctx, jobID, path)
		if err != nil {
			continue
		}
		lines := strings.Split(content, "\n")
		file := map[string]any{"path": path, "sha256": sourceHash(content), "lines": len(lines)}
		var hooks []SourceMatch
		for i, line := range lines {
			for _, hook := range []string{"setup(", "step(", "action(", "reset(", "startGame(", "startVoxelGame(", "class GameScene"} {
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
