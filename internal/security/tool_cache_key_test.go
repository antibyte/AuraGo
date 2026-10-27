package security

import (
	"context"
	"fmt"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// legacyCacheKey is GenerateCacheKey as it was before the key became injective:
// the operation was written unprefixed, so a context spelled like a parameter
// list could stand in for the parameters.
func legacyCacheKey(operation string, params map[string]string) string {
	keys := sortedGuardianParamKeys(params)
	var sb strings.Builder
	sb.WriteString(operation)
	for _, k := range keys {
		fmt.Fprintf(&sb, ":%d:%s=%d:%s", len(k), k, len(params[k]), params[k])
	}
	return sb.String()
}

func TestGenerateCacheKeyContextCannotImpersonateParams(t *testing.T) {
	first := map[string]string{"command": "echo :7:command=6:reboot"}
	second := map[string]string{"command": "reboot"}
	const spelled = ":7:command=24:echo "
	// The premise: the old encoding gave both calls one key.
	if legacyCacheKey("execute_shell|", first) != legacyCacheKey("execute_shell|"+spelled, second) {
		t.Fatal("fixture no longer collides under the legacy encoding")
	}
	if GenerateCacheKey("execute_shell|", first) == GenerateCacheKey("execute_shell|"+spelled, second) {
		t.Fatal("the operation text impersonates a parameter list")
	}
	if generateToolCacheKey("execute_shell", "", first) == generateToolCacheKey("execute_shell", spelled, second) {
		t.Fatal("the context snippet impersonates a parameter list")
	}
}

func TestToolCacheKeySeparatesEveryField(t *testing.T) {
	params := map[string]string{"command": "ls"}
	keys := map[string]string{
		"base":                    generateToolCacheKey("execute_shell", "ctx", params),
		"operation moved into it": generateToolCacheKey("execute_shell|ctx", "", params),
		"context moved into op":   generateToolCacheKey("execute_shell", "|ctx", params),
		"other operation":         generateToolCacheKey("execute_sudo", "ctx", params),
		"other context":           generateToolCacheKey("execute_shell", "ctx2", params),
		"other value":             generateToolCacheKey("execute_shell", "ctx", map[string]string{"command": "ls "}),
		"other name":              generateToolCacheKey("execute_shell", "ctx", map[string]string{"command ": "ls"}),
		"no context field":        GenerateCacheKey("execute_shell", params),
	}
	seen := map[string]string{}
	for name, key := range keys {
		if other, dup := seen[key]; dup {
			t.Errorf("%q and %q share the cache key %s", name, other, key)
		}
		seen[key] = name
	}
	if generateToolCacheKey("execute_shell", "ctx", map[string]string{"a": "1", "b": "2"}) !=
		generateToolCacheKey("execute_shell", "ctx", map[string]string{"b": "2", "a": "1"}) {
		t.Error("parameter order changes the key")
	}
}

// Before the fix, the second call was served the first call's judged-safe verdict.
func TestEvaluateDoesNotServeACachedAllowToAContextThatSpellsTheParams(t *testing.T) {
	g, calls, _ := newCompletionGuardian(t, "block", completionReply(openai.FinishReasonStop, "safe 5 routine cron list"))
	if got := g.Evaluate(context.Background(), GuardianCheck{
		Operation: "execute_shell", Parameters: map[string]string{"command": "echo :7:command=6:reboot"},
	}); got.Decision != DecisionAllow {
		t.Fatalf("first call = %+v, want allow", got)
	}
	second := g.Evaluate(context.Background(), GuardianCheck{
		Operation: "execute_shell", Parameters: map[string]string{"command": "reboot"}, Context: ":7:command=24:echo ",
	})
	if second.Cached || calls.Load() != 2 {
		t.Fatalf("second call = %+v after %d provider calls, want a fresh judgement", second, calls.Load())
	}
}
