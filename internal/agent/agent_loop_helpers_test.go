package agent

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

// makeTool is a test helper that builds a minimal openai.Tool with the given function name.
func makeTool(name string) openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: name,
		},
	}
}

// toolNames extracts function names from a tool slice.
func toolNames(tools []openai.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		if t.Function != nil {
			names = append(names, t.Function.Name)
		}
	}
	return names
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func TestFilterToolSchemas_AlwaysIncludeKept(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("filesystem"),
		makeTool("docker"),
		makeTool("rarely_used"),
	}
	result := filterToolSchemas(schemas, []string{}, []string{"filesystem"}, 10, nil)
	names := toolNames(result)
	if !containsName(names, "filesystem") {
		t.Error("alwaysInclude tool 'filesystem' should always be kept")
	}
}

func TestFilterToolSchemas_FrequentToolKept(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("docker"),
		makeTool("never_used"),
	}
	result := filterToolSchemas(schemas, []string{"docker"}, []string{}, 10, nil)
	names := toolNames(result)
	if !containsName(names, "docker") {
		t.Error("frequent tool 'docker' should be kept")
	}
}

func TestFilterToolSchemas_SkillPrefixAlwaysKept(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("skill__backup"),
		makeTool("tool__my_custom"),
		makeTool("obscure_tool"),
	}
	result := filterToolSchemas(schemas, []string{}, []string{}, 0, nil)
	names := toolNames(result)
	if !containsName(names, "skill__backup") {
		t.Error("skill__-prefixed tool should always be kept")
	}
	if !containsName(names, "tool__my_custom") {
		t.Error("tool__-prefixed tool should always be kept")
	}
}

func TestFilterToolSchemas_MaxToolsLimit(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("a"), makeTool("b"), makeTool("c"), makeTool("d"), makeTool("e"),
	}
	result := filterToolSchemas(schemas, []string{"a", "b", "c", "d", "e"}, []string{}, 3, nil)
	if len(result) > 3 {
		t.Errorf("expected at most 3 tools, got %d", len(result))
	}
}

func TestFilterToolSchemas_MaxToolsZeroDisablesLimit(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("a"), makeTool("b"), makeTool("c"),
	}
	// maxTools=0 → no limit; all frequent tools are kept
	result := filterToolSchemas(schemas, []string{"a", "b", "c"}, []string{}, 0, nil)
	if len(result) != 3 {
		t.Errorf("expected all 3 tools with maxTools=0, got %d", len(result))
	}
}

func TestFilterToolSchemas_EmptyFrequentFallsBackToDropped(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("x"), makeTool("y"),
	}
	// No frequent tools, no alwaysInclude, maxTools=5 → remaining slots filled from dropped list
	result := filterToolSchemas(schemas, []string{}, []string{}, 5, nil)
	// Both tools land in 'dropped', then are added via remaining-slots fill-up
	if len(result) != 2 {
		t.Errorf("expected 2 tools from fill-up, got %d", len(result))
	}
}

func TestFilterToolSchemas_AlwaysIncludeNotDuplicatedByFrequent(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("filesystem"),
		makeTool("docker"),
	}
	result := filterToolSchemas(schemas,
		[]string{"filesystem"}, // also in frequentTools
		[]string{"filesystem"}, // and in alwaysInclude
		10, nil)
	count := 0
	for _, t2 := range result {
		if t2.Function != nil && t2.Function.Name == "filesystem" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("'filesystem' should appear exactly once, got %d", count)
	}
}

func TestFilterToolSchemas_OriginalOrderPreservedForDropped(t *testing.T) {
	// Dropped tools are appended in original schema order
	schemas := []openai.Tool{
		makeTool("rare1"), makeTool("rare2"), makeTool("rare3"),
	}
	result := filterToolSchemas(schemas, []string{}, []string{}, 2, nil)
	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}
	if result[0].Function.Name != "rare1" || result[1].Function.Name != "rare2" {
		t.Errorf("expected original order rare1,rare2; got %s,%s",
			result[0].Function.Name, result[1].Function.Name)
	}
}
