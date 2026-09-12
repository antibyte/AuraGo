package server

import "testing"

type gameMakerEvalRequirement struct {
	ID      string   `json:"id"`
	Mode    string   `json:"mode"`
	Summary string   `json:"summary"`
	AllOf   []string `json:"all_of,omitempty"`
	AnyOf   []string `json:"any_of,omitempty"`
}

type gameMakerEvaluationTask struct {
	name         string
	dimension    string
	prompt       string
	requirements []gameMakerEvalRequirement
}

func gameMakerEvalRequirements(name string) []gameMakerEvalRequirement {
	manual := func(id, summary string) gameMakerEvalRequirement {
		return gameMakerEvalRequirement{ID: id, Mode: "manual", Summary: summary}
	}
	automatic := func(id, summary string, allOf ...string) gameMakerEvalRequirement {
		return gameMakerEvalRequirement{ID: id, Mode: "automatic", Summary: summary, AllOf: allOf}
	}
	switch name {
	case "breakout":
		return []gameMakerEvalRequirement{
			automatic("identity", "accepted plan retains the Neon Orchard identity", "neon orchard"),
			automatic("rules", "accepted plan states lives, scoring and reachable brick play", "three lives", "ten points", "reachable"),
			automatic("custom_mechanic", "accepted plan preserves the every-fifth collectible paddle mechanic", "every fifth", "paddle", "eight seconds"),
			automatic("terminal_states", "accepted plan distinguishes victory after clearing and defeat after misses", "victory", "defeat"),
			manual("visual_identity", "inspect the running game for a coherent Neon Orchard identity, readable HUD and clear terminal states"),
			manual("gameplay_quality", "play the exported game to verify reachability, collectible timing and natural win/loss behavior"),
		}
	case "platformer":
		return []gameMakerEvalRequirement{
			automatic("identity", "accepted plan retains the Moss Post woodland identity", "moss post"),
			automatic("level_contract", "accepted plan states six varied platforms and a reachable exit", "six", "platform", "reachable exit"),
			automatic("collectibles", "accepted plan requires four letters before the exit opens", "four", "letters", "exit"),
			automatic("safety", "accepted plan preserves crossable gaps and stable start and exit areas", "crossable", "stable", "start", "exit"),
			manual("visual_identity", "inspect the running game for friendly woodland presentation and readable letters/lives HUD"),
			manual("gameplay_quality", "play the exported game to verify every gap, collectible gate and falling-life behavior"),
		}
	case "forest-fps":
		return []gameMakerEvalRequirement{
			automatic("identity", "accepted plan retains the Lantern Patrol identity", "lantern patrol"),
			automatic("world_contract", "accepted plan names local low-poly forest assets and five targets", "low-poly", "five", "target"),
			automatic("combat_rules", "accepted plan states safe routes, health and five-target win/loss rules", "safe", "health", "five"),
			automatic("hit_feedback", "accepted plan binds effects and sounds to actual hits", "muzzle-flash", "actual hits"),
			manual("visual_identity", "inspect the running game for Lantern Patrol identity, legible objective/health and readable cover"),
			manual("gameplay_quality", "play the exported game to verify navigable routes, target damage and terminal outcomes"),
		}
	case "hybrid":
		return []gameMakerEvalRequirement{
			automatic("identity", "accepted plan retains the Quiet Orbit Courier identity", "quiet orbit courier"),
			automatic("delivery_loop", "accepted plan states three delivery rings and a destination marker", "three", "delivery", "destination"),
			automatic("custom_lantern_rule", "accepted plan records the custom lantern charge, brake and drain rule", "lantern", "brake", "drain"),
			automatic("peaceful_outcome", "accepted plan explicitly avoids combat and countdown loss", "no combat", "no countdown", "win"),
			manual("visual_identity", "inspect the running game for calm blue/gold space identity and readable charge/delivery state"),
			manual("gameplay_quality", "play the exported game to verify ring timing, charge/drain behavior and peaceful completion"),
		}
	default:
		return []gameMakerEvalRequirement{
			manual("visual_identity", "inspect the running game for a coherent requested identity"),
			manual("gameplay_quality", "play the exported game to verify the requested loop and terminal behavior"),
		}
	}
}

// Keep the exact same human briefs for both models and baseline/candidate builds.
func gameMakerEvaluationTasks(builder bool) []gameMakerEvaluationTask {
	if !builder {
		return []gameMakerEvaluationTask{
			{name: "forest-fps", dimension: "3d", prompt: "Make a first-person forest patrol game using the local low-poly models: modern arms, rifle, pine trees and five soldiers as targets. WASD movement, mouse or arrow aiming, Space to shoot, F reload, health, a 90-second limit, restart and a visible Forest Patrol title. Award ten points per target and complete after five targets. Use the supported FPS base and implement all requested scoring rules.", requirements: gameMakerEvalRequirements("forest-fps")},
			{name: "breakout", dimension: "2d", prompt: "Create Breakout using built-in sprite art for the paddle, ball and bricks. Three lives, ten points per brick, left/right movement, Space to launch and R restart. Show a clear victory when all bricks are cleared. Use the blocks base and retain its working lifecycle.", requirements: gameMakerEvalRequirements("breakout")},
		}
	}
	return []gameMakerEvaluationTask{
		{name: "breakout", dimension: "2d", prompt: "Create Neon Orchard Breakout with built-in paddle, ball and brick sprites. Three lives, ten points per brick, left/right movement, Space to launch and R restart. All bricks must be reachable. Every fifth destroyed brick drops a collectible that widens the paddle for eight seconds; collect it with the paddle. Show victory only after all bricks are cleared and defeat after the third missed ball, regardless of score. Add stone-debris for real brick hits, pickup-glow when collecting and suitable hit/pickup/win/lose sounds. Keep HUD clear of the bricks. Use seed 1729 for any generated layout.", requirements: gameMakerEvalRequirements("breakout")},
		{name: "platformer", dimension: "2d", prompt: "Build Moss Post, a side-view forest platform game with local sprite art, six varied platforms and a reachable exit. Collect four letters before the exit opens. Left/right movement, Space to jump, three lives after falling, R restart. Show remaining letters and lives. All gaps must be crossable; retain stable starting and exit areas when varying the surrounding vegetation. Use seed 1729. Add forest ambience and feedback for jumping, collecting and finishing. Give the game a friendly woodland identity.", requirements: gameMakerEvalRequirements("platformer")},
		{name: "forest-fps", dimension: "3d", prompt: "Build Lantern Patrol, a first-person forest game using local low-poly modern arms, rifle, pine trees, cover and five soldier targets. WASD movement, mouse or arrow aiming, Space to shoot, F reload, health, R restart. Targets can damage the player; provide safe cover and navigable routes from spawn. Award ten points per defeated target; win after five, lose at zero health. Add forest-rain, muzzle-flash on firing and hit effects/sounds only on actual hits. Use seed 1729. Show objective and health clearly.", requirements: gameMakerEvalRequirements("forest-fps")},
		{name: "hybrid", dimension: "3d", prompt: "Create Quiet Orbit Courier: a low-poly space delivery game mixing flight with a peaceful timing puzzle. Steer a spacecraft through three delivery rings using movement and altitude controls. A delivery ring opens only while the ship's lantern is charged: holding the brake charges it, ordinary flying drains it. Implement this lantern rule as custom code. Show charge, delivered parcels and a readable destination marker. No combat or countdown loss. Win after all deliveries, R restarts. Use local ship/planet assets, a space atmosphere and delivery sounds. Use seed 1729 and a distinct calm blue/gold visual identity.", requirements: gameMakerEvalRequirements("hybrid")},
	}
}

func TestGameMakerBuilderEvaluationMatrix(t *testing.T) {
	tasks := gameMakerEvaluationTasks(true)
	if len(tasks) != 4 || len(gameMakerEvaluationTasks(false)) != 2 {
		t.Fatal("unexpected comparison matrix")
	}
	seen := map[string]bool{}
	for _, task := range tasks {
		if seen[task.name] || task.prompt == "" || task.dimension != "2d" && task.dimension != "3d" || len(task.requirements) == 0 {
			t.Fatalf("invalid task: %+v", task)
		}
		seen[task.name] = true
	}
}
