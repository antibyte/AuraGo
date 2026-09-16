package server

import "testing"

func gameMakerWorldRequirements(name string) []gameMakerEvalRequirement {
	manual := func(id, summary string) gameMakerEvalRequirement {
		return gameMakerEvalRequirement{ID: id, Mode: "manual", Summary: summary}
	}
	common := []gameMakerEvalRequirement{
		manual("standalone_export", "Play the extracted ZIP on a static server under a subpath, with no AuraGo endpoints or remote assets"),
		manual("lifecycle", "Verify input, pause, focus changes and three restarts without duplicate actors, audio or animation loops"),
	}
	switch name {
	case "maritime-2d":
		return append(common,
			manual("directional_art", "Observe the planned pirate sloop and enemies using the catalog's 16 directions and real sailing animation"),
			manual("projectile_contacts", "Observe actual cannon projectile contacts removing the same target's hull, with feedback only on contact"),
			manual("navigation_outcomes", "Navigate around the island; win after the third real ship hit, lose at zero hull and reset all state with R"))
	case "maritime-3d":
		return append(common,
			manual("animated_models", "Observe the actual diver and reef shark with independent animation states and a usable following camera"),
			manual("depth_contacts", "Use Q/E depth movement to reach all three chests and observe real pickup and shark contact damage"),
			manual("outcomes", "Win only after three pickups, lose at zero health and reset collected chests and health with R"))
	case "isometric":
		return append(common,
			manual("elevation_navigation", "Walk the declared stairs in both directions, with stable eight-direction art and logical footprint collision"),
			manual("interior_transition", "Interact with the door, enter the separate interior, hide its roof and collect the actual lantern"),
			manual("sorting_completion", "Verify depth ordering, pointer selection, readable HUD and completion only after obtaining the lantern"))
	}
	return nil
}

func gameMakerWorldEvaluationTasks() []gameMakerEvaluationTask {
	return []gameMakerEvaluationTask{
		{name: "maritime-2d", dimension: "2d", prompt: "Create Brass Tide, a top-down pirate sea battle. Use the aurago-pirates-topdown pack: an animated sloop, three enemy ships and one island. Arrow/WASD steering, Space fires a cannon, three hull points, win only after three real ship hits and lose at zero hull. Use the real 16-direction artwork, proper projectile contacts, a clear HUD and R restart. Use the existing asset examples and local water/hit effects and sounds. No timer. Keep a navigable channel around the island. Build the custom rules in a minimal scene and test actual contacts.", requirements: gameMakerWorldRequirements("maritime-2d")},
		{name: "maritime-3d", dimension: "3d", prompt: "Build Copper Reef, a 3D underwater exploration game using aurago-pirates-3d: an independently animated brass diver and reef shark, three treasure chests and a reef. WASD moves the diver, Q/E controls depth, collect the three chests by actual contact. Avoid the patrolling shark; three health points, explicit win after all chests and lose at zero health, R restart. Show readable controls, health and treasure count. Use underwater presentation and the existing local asset helper, sockets, exact animation names and one game loop. Keep the camera behind the diver and all treasures reachable.", requirements: gameMakerWorldRequirements("maritime-3d")},
		{name: "isometric", dimension: "2d", prompt: "Create Lantern Harbour using aurago-isometric pixel art and the existing minimal/isometric scene helpers. Use scene schema 2, 128x64 projection and 32-pixel height steps. Build a walkable village terrace with two heights connected by declared stairs, an eight-direction adventurer, an interactive door and a separate indoor level. Enter the room to collect a lantern, then show an explicit completion. Arrow/WASD movement, Space interacts, R restarts. Use exact catalog IDs, separate roof art that hides when inside, correct depth ordering, logical-grid collision and a clear HUD. No forced combat or timer. Verify the elevation connection and room transition with actual input.", requirements: gameMakerWorldRequirements("isometric")},
	}
}

func TestGameMakerWorldEvaluationMatrix(t *testing.T) {
	tasks := gameMakerWorldEvaluationTasks()
	if len(tasks) != 3 {
		t.Fatal("expected three briefs for each configured model")
	}
	seen := map[string]bool{}
	for _, task := range tasks {
		if seen[task.name] || task.prompt == "" || len(task.requirements) < 5 {
			t.Fatal("invalid world evaluation brief")
		}
		seen[task.name] = true
	}
}
