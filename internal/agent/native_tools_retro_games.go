package agent

import openai "github.com/sashabaranov/go-openai"

// retroGamesSchema describes the read-only retro_games tool. It appears only
// while the server publishes an enabled service (resolveToolFeatureState) and
// is non-displacing (prompts.NonDisplacingTools); the manual has the details.
func retroGamesSchema() openai.Tool {
	return tool("retro_games",
		"Browser-playable retro games from archive.org on the virtual desktop: search the local index (DOS, arcade, consoles, C64, Amiga, Apple II, ZX, Windows 3.x) or open a game in the Retro Games app.",
		schema(map[string]interface{}{
			"operation":  operationProperty("search lists games (no query: Hall of Fame), play opens one", []string{"search", "play"}),
			"query":      prop("string", "search: title, maker or keywords"),
			"platform":   prop("string", "search: msdos, arcade, console, c64, amiga, apple2, zx or win3"),
			"decade":     prop("integer", "search: 1970, 1980, 1990 or 2000"),
			"limit":      prop("integer", "search: 1-20, default 10"),
			"identifier": prop("string", "play: identifier from search"),
		}, "operation"),
	)
}
