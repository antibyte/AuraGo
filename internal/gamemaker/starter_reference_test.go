package gamemaker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/prompts"
)

func TestStarterReferenceVerifiedAndBounded(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			service := newTestService(t)
			design := ExampleGameDesign(Project{Dimension: dimension})
			if dimension == "3d" {
				design.Base = "fps"
			}
			plan, err := service.planFromDesign(context.Background(), "", Project{Dimension: dimension}, design)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := gameTemplateSources(plan)
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{}
			for _, path := range []string{"common.ts", "scene.json", "mechanics.json"} {
				files["src/"+path] = string(expected[path])
			}
			baseline, _ := json.Marshal(map[string]any{"mode": "full", "files": files})
			compact := starterReferences(&plan, files)
			if compact["mode"] != "verified_template_api" {
				t.Fatalf("unchanged helper not compact: %s", dimension)
			}
			encoded, _ := json.Marshal(compact)
			before, after := prompts.CountTokensForModel(string(baseline), "gpt-4o"), prompts.CountTokensForModel(string(encoded), "gpt-4o")
			t.Logf("reference tokens: baseline=%d compact=%d reduction=%.1f%%", before, after, 100*(1-float64(after)/float64(before)))
			if after*2 > before {
				t.Fatalf("reference reduction below 50%%: %d -> %d", before, after)
			}
			original := files["src/common.ts"]
			for _, changed := range []string{original + "\n// authored change", strings.Replace(original, "AURAGO_RUNTIME_API", "LEGACY_RUNTIME_API", 1)} {
				files["src/common.ts"] = changed
				result := starterReferences(&plan, files)
				full, ok := result["files"].(map[string]string)
				if result["mode"] != "full" || !ok || full["src/common.ts"] != changed {
					t.Fatal("custom/legacy helper truncated or misidentified")
				}
			}
			files["src/common.ts"] = original
			wrongPlan := plan
			wrongPlan.Assets = nil
			wrongPlan.Width++
			// A changed plan must not gain the other plan's API binding.
			if dimension == "2d" && starterReferences(&wrongPlan, files)["mode"] != "full" {
				t.Fatal("comparison omitted accepted-plan substitutions")
			}
		})
	}
}

func TestStarterStructureReportsOmissions(t *testing.T) {
	value := map[string]any{"nodes": make([]any, 50), "description": strings.Repeat("x", 900)}
	data, _ := json.Marshal(starterStructure(value, 0))
	if !strings.Contains(string(data), `"omitted_items":42`) || !strings.Contains(string(data), `"content_omitted":true`) {
		t.Fatal(string(data))
	}
}
