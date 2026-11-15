package gamemaker

func defaultDesignAssets(base string) []DesignAsset {
	roles := []string{"player", "enemy", "item", "goal", "obstacle"}
	switch base {
	case "shooter":
		roles = []string{"player", "enemy", "projectile", "goal", "obstacle"}
	case "board":
		roles = []string{"cell_hidden", "cell_shown", "token"}
	}
	fallbacks := map[string]string{
		"player":      "procedural hero capsule",
		"enemy":       "procedural hostile shape",
		"item":        "procedural pickup diamond",
		"projectile":  "procedural shot capsule",
		"goal":        "procedural goal ring",
		"obstacle":    "procedural obstacle block",
		"cell_hidden": "procedural closed cell",
		"cell_shown":  "procedural revealed cell",
		"token":       "procedural player marker",
	}
	out := make([]DesignAsset, 0, len(roles))
	for _, role := range roles {
		fallback := fallbacks[role]
		if fallback == "" {
			fallback = "procedural " + role + " shape"
		}
		out = append(out, DesignAsset{Role: role, Fallback: fallback})
	}
	return out
}
