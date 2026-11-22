package chibisensei

import "regexp"

const (
	RecipeAssistant  = "assistant"
	RecipeSpecialist = "specialist"
	RecipeFree       = "free"

	MaxMessages     = 64    // messages per row
	MaxMessageChars = 32000 // runes per message or text row
	PageDefault     = 50
	PageMax         = 200
	MaxRowsPerPost  = 200
)

// Recipe is one training recipe with the task kinds it accepts.
type Recipe struct {
	ID        string   `json:"id"`
	TaskKinds []string `json:"task_kinds"`
}

// Recipes lists the launch recipes in display order.
func Recipes() []Recipe {
	return []Recipe{{RecipeAssistant, []string{}},
		{RecipeSpecialist, []string{"classify", "extract", "rewrite", "to_json"}},
		{RecipeFree, []string{}}}
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// ValidID reports whether s is a lower-case UUID (project, document, job and batch IDs).
func ValidID(s string) bool { return uuidPattern.MatchString(s) }
