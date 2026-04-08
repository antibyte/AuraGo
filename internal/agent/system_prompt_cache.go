package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"aurago/internal/prompts"
)

type systemPromptCacheKey struct {
	PromptsDir  string             `json:"prompts_dir"`
	Flags       prompts.ContextFlags `json:"flags"`
	CoreMemory  string             `json:"core_memory"`
	BudgetHint  string             `json:"budget_hint"`
}

func buildSystemPromptCacheKey(promptsDir string, flags prompts.ContextFlags, coreMemory, budgetHint string) (string, error) {
	key := systemPromptCacheKey{
		PromptsDir: promptsDir,
		Flags:      flags,
		CoreMemory: coreMemory,
		BudgetHint: budgetHint,
	}
	b, err := json.Marshal(key)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

