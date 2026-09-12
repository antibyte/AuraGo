package gamemaker

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBlocksTemplateKeepsNaturalProgressContract(t *testing.T) {
	data, err := gameTemplates.ReadFile("templates/blocks.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, required := range []string{
		"this.physics.world.checkCollision.down=false",
		"this.lives--; this.state.lives=this.lives",
		"this.state.goal_remaining=this.remainingBricks()",
		"this.state.hit_events++",
		"if(this.state.goal_remaining===0)this.end(true)",
		"auditGame()",
		"__auragoBrickID='brick-'+row+'-'+col",
		"const roles=this.assetRoles('block')",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("Blocks template lost required gameplay contract %q", required)
		}
	}
	if strings.Contains(source, "roles[(row*8+col)%roles.length]") {
		t.Fatal("Blocks template still assumes an eight-column layout when selecting asset roles")
	}
}

func TestBlocksTemplateBuildsAtSmallAndLargeResolutions(t *testing.T) {
	for _, size := range []struct{ width, height int }{{320, 240}, {640, 360}, {1920, 1080}} {
		t.Run(strings.Join([]string{strconv.Itoa(size.width), strconv.Itoa(size.height)}, "x"), func(t *testing.T) {
			root := t.TempDir()
			project := Project{Name: "Blocks", Dimension: "2d"}
			if err := WriteScaffold(root, project); err != nil {
				t.Fatal(err)
			}
			plan := ExampleGamePlan(project)
			plan.Template, plan.Width, plan.Height = "blocks", size.width, size.height
			if err := installGameTemplate(root, plan); err != nil {
				t.Fatal(err)
			}
			result := buildDirectory(context.Background(), root, 150, 64<<20)
			if !result.OK {
				t.Fatalf("build failed at %dx%d: %+v", size.width, size.height, result.Diagnostics)
			}
			if _, err := os.Stat(filepath.Join(root, "dist", "game.js")); err != nil {
				t.Fatalf("compiled game missing at %dx%d: %v", size.width, size.height, err)
			}
		})
	}
}
