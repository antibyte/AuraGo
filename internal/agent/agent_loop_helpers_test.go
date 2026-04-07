package agent

import (
	"path/filepath"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func intPtr(v int) *int { return &v }

func TestStreamToolCallAssemblerEmptyReturnsNil(t *testing.T) {
	asm := NewStreamToolCallAssembler()
	result := asm.Assemble()
	if result != nil {
		t.Fatalf("expected nil for empty assembler, got %d items", len(result))
	}
}

func TestStreamToolCallAssemblerMergesAndSorts(t *testing.T) {
	asm := NewStreamToolCallAssembler()
	asm.Merge(openai.ToolCall{
		Index: intPtr(7),
		ID:    "call-7",
		Function: openai.FunctionCall{
			Name:      "filesystem",
			Arguments: `{"operation":"stat"}`,
		},
	})
	asm.Merge(openai.ToolCall{
		Index: intPtr(2),
		ID:    "call-2",
		Function: openai.FunctionCall{
			Name:      "execute_shell",
			Arguments: `{"command":"pwd"}`,
		},
	})

	result := asm.Assemble()
	if len(result) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(result))
	}
	if result[0].ID != "call-2" || result[1].ID != "call-7" {
		t.Fatalf("unexpected assembly order: %q, %q", result[0].ID, result[1].ID)
	}
}

func TestStreamToolCallAssemblerAppendsFragments(t *testing.T) {
	asm := NewStreamToolCallAssembler()
	asm.Merge(openai.ToolCall{
		Index: intPtr(1),
		ID:    "call-1",
		Function: openai.FunctionCall{
			Name:      "exec",
			Arguments: `{"comm`,
		},
	})
	asm.Merge(openai.ToolCall{
		Index: intPtr(1),
		Function: openai.FunctionCall{
			Name:      "ute_shell",
			Arguments: `and":"pwd"}`,
		},
	})

	result := asm.Assemble()
	if len(result) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result))
	}
	if result[0].Function.Name != "execute_shell" {
		t.Fatalf("Function.Name = %q, want %q", result[0].Function.Name, "execute_shell")
	}
	if result[0].Function.Arguments != `{"command":"pwd"}` {
		t.Fatalf("Function.Arguments = %q", result[0].Function.Arguments)
	}
}

func TestAssembleSortedStreamToolCallsHandlesSparseIndices(t *testing.T) {
	streamToolCalls := map[int]*openai.ToolCall{}
	mergeStreamToolCallChunk(streamToolCalls, openai.ToolCall{
		Index: intPtr(7),
		ID:    "call-7",
		Function: openai.FunctionCall{
			Name:      "filesystem",
			Arguments: `{"operation":"stat"}`,
		},
	})
	mergeStreamToolCallChunk(streamToolCalls, openai.ToolCall{
		Index: intPtr(2),
		ID:    "call-2",
		Function: openai.FunctionCall{
			Name:      "execute_shell",
			Arguments: `{"command":"pwd"}`,
		},
	})

	assembled := assembleSortedStreamToolCalls(streamToolCalls)
	if len(assembled) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(assembled))
	}
	if assembled[0].ID != "call-2" || assembled[1].ID != "call-7" {
		t.Fatalf("unexpected assembly order: %q, %q", assembled[0].ID, assembled[1].ID)
	}
}

func TestMergeStreamToolCallChunkAppendsFragments(t *testing.T) {
	streamToolCalls := map[int]*openai.ToolCall{}
	mergeStreamToolCallChunk(streamToolCalls, openai.ToolCall{
		Index: intPtr(1),
		ID:    "call-1",
		Function: openai.FunctionCall{
			Name:      "exec",
			Arguments: `{"comm`,
		},
	})
	mergeStreamToolCallChunk(streamToolCalls, openai.ToolCall{
		Index: intPtr(1),
		Function: openai.FunctionCall{
			Name:      "ute_shell",
			Arguments: `and":"pwd"}`,
		},
	})

	assembled := assembleSortedStreamToolCalls(streamToolCalls)
	if len(assembled) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(assembled))
	}
	if assembled[0].Function.Name != "execute_shell" {
		t.Fatalf("Function.Name = %q, want %q", assembled[0].Function.Name, "execute_shell")
	}
	if assembled[0].Function.Arguments != `{"command":"pwd"}` {
		t.Fatalf("Function.Arguments = %q", assembled[0].Function.Arguments)
	}
}

type fakeGuideSearcher struct {
	paths []string
	err   error
}

func (f fakeGuideSearcher) SearchToolGuides(query string, topK int) ([]string, error) {
	return f.paths, f.err
}

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

func TestFilterToolSchemas_PrioritizesPreferredOrder(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("filesystem"),
		makeTool("docker"),
		makeTool("homepage"),
	}

	result := filterToolSchemas(schemas, []string{"homepage", "docker"}, nil, 2, nil)
	names := toolNames(result)
	if len(names) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(names))
	}
	if names[0] != "homepage" || names[1] != "docker" {
		t.Fatalf("expected prioritized order homepage,docker got %v", names)
	}
}

func TestExtractIntentMatchedToolsMatchesSplitToolNames(t *testing.T) {
	matches := extractIntentMatchedTools("please scan the network and then inspect devices", []string{"network_scan", "notes", "homepage"})
	if len(matches) == 0 || matches[0] != "network_scan" {
		t.Fatalf("expected network_scan intent match, got %v", matches)
	}
}

func TestBuildAdaptiveToolPriorityPrefersIntentAndSemanticHits(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("shell"),
		makeTool("docker"),
		makeTool("homepage"),
		makeTool("netlify"),
	}

	prioritized := buildAdaptiveToolPriority(
		schemas,
		[]string{"shell", "docker"},
		"please deploy the homepage to netlify",
		fakeGuideSearcher{paths: []string{filepath.Join("tools_manuals", "homepage.md")}},
		nil,
	)

	if len(prioritized) < 3 {
		t.Fatalf("expected at least 3 prioritized tools, got %v", prioritized)
	}
	if prioritized[0] != "homepage" {
		t.Fatalf("expected homepage to be prioritized first, got %v", prioritized)
	}
	if !containsName(prioritized, "netlify") {
		t.Fatalf("expected netlify to be included from direct intent match, got %v", prioritized)
	}
	if !containsName(prioritized, "shell") {
		t.Fatalf("expected weighted usage fallback to remain, got %v", prioritized)
	}
}

func TestBuildAdaptiveToolPriorityAddsCuratedDependencyNeighbors(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("homepage"),
		makeTool("netlify"),
		makeTool("homepage_registry"),
		makeTool("filesystem"),
		makeTool("shell"),
	}

	prioritized := buildAdaptiveToolPriority(
		schemas,
		[]string{"shell"},
		"please deploy the homepage",
		nil,
		nil,
	)

	if len(prioritized) < 3 {
		t.Fatalf("expected at least 3 prioritized tools, got %v", prioritized)
	}
	if prioritized[0] != "homepage" {
		t.Fatalf("expected homepage first, got %v", prioritized)
	}
	if !containsName(prioritized, "netlify") {
		t.Fatalf("expected curated homepage neighbor netlify, got %v", prioritized)
	}
	if !containsName(prioritized, "homepage_registry") {
		t.Fatalf("expected curated homepage neighbor homepage_registry, got %v", prioritized)
	}
}
