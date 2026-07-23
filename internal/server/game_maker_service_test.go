package server

import (
	"slices"
	"testing"

	"aurago/internal/gamemaker"
)

func TestGameMakerAgentScopeContainsOnlyCuratedToolsAndSkills(t *testing.T) {
	wantTools := []string{
		"game_maker_project",
		"game_maker_file",
		"game_maker_asset",
		"game_maker_validate",
		"list_agent_skills",
		"activate_agent_skill",
	}
	if !slices.Equal(gameMakerAllowedTools, wantTools) {
		t.Fatalf("Game Maker AllowedTools = %v, want %v", gameMakerAllowedTools, wantTools)
	}
	for _, forbidden := range []string{
		"invoke_tool", "filesystem", "execute_shell", "execute_python",
		"api_request", "homepage_project", "desktop_computer",
	} {
		if slices.Contains(gameMakerAllowedTools, forbidden) {
			t.Fatalf("forbidden tool %q present in Game Maker scope", forbidden)
		}
	}
	wantSkills := []string{
		"aurago-game-assets",
		"aurago-game-maker-director",
		"aurago-game-qa",
		"aurago-phaser4-gameplay",
		"aurago-threejs-gameplay",
	}
	if got := gamemaker.CuratedSkillNames(); !slices.Equal(got, wantSkills) {
		t.Fatalf("curated Game Maker skills = %v, want %v", got, wantSkills)
	}
}
