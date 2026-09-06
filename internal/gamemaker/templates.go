package gamemaker

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed templates/*.ts
var gameTemplates embed.FS

func installGameTemplate(stage string, plan GamePlan) error {
	if !slices.Contains(templateNames()[:6], plan.Template) {
		return fmt.Errorf("unknown 2D template %q", plan.Template)
	}
	for source, target := range map[string]string{plan.Template + ".ts": "main.ts", "common.ts": "common.ts"} {
		data, err := gameTemplates.ReadFile("templates/" + source)
		if err != nil {
			return fmt.Errorf("read game template: %w", err)
		}
		if source == "common.ts" {
			data = []byte(strings.Replace(string(data), "width: 960, height: 540", fmt.Sprintf("width: %d, height: %d", plan.Width, plan.Height), 1))
		}
		if err := os.WriteFile(filepath.Join(stage, "src", target), data, 0o640); err != nil {
			return fmt.Errorf("install game template: %w", err)
		}
	}
	return nil
}
