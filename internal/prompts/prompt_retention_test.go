package prompts

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestUnifiedMemoryBuilderUsesCanonicalOptionalHeading(t *testing.T) {
	block := buildUnifiedMemoryContextBlock("full", &ContextFlags{RetrievedMemories: "one memory"})
	if !strings.HasPrefix(block, promptSectionUnifiedMemory+"\n") {
		t.Fatalf("unified memory heading = %q, want canonical %q", strings.SplitN(block, "\n", 2)[0], promptSectionUnifiedMemory)
	}
	if strings.Contains(block, "## UNIFIED MEMORY CONTEXT") {
		t.Fatal("builder emitted a non-canonical heading that prompt shedding cannot recognize")
	}
}

func TestUnifiedMemorySubsectionsShareOneOptionalGroup(t *testing.T) {
	text := "# UNIFIED MEMORY CONTEXT\nwarning\n" +
		"## Retrieved Memories\nsource\n### nested source\nmore\n" +
		"## Known Error Patterns\nerror notes\n" +
		"## Reuse-First Context\nreuse notes\n" +
		"# REQUIRED FOLLOWUP\nKeep this.\n"
	sections := splitPromptSections(text)
	for _, section := range sections {
		if section.ID == "# UNIFIED MEMORY CONTEXT" || strings.HasPrefix(section.ID, "## Retrieved Memories") || strings.HasPrefix(section.ID, "### nested source") || strings.HasPrefix(section.ID, "## Known Error Patterns") || strings.HasPrefix(section.ID, "## Reuse-First Context") {
			if section.Required || section.GroupID != promptSectionUnifiedMemory {
				t.Fatalf("unified-memory section was not grouped as optional: %+v", section)
			}
		}
		if section.ID == "# REQUIRED FOLLOWUP" && !section.Required {
			t.Fatal("unknown top-level heading inherited optional policy")
		}
	}
}

func TestOptionalPersonaNotesShedBeforeTaskRules(t *testing.T) {
	for _, header := range []string{promptSectionPersonaSignals, promptSectionPersonaCharacter} {
		priority, required, known := promptSectionDirectPolicy(header)
		taskPriority, _, taskKnown := promptSectionDirectPolicy(promptSectionTaskRules)
		if !known || required || !taskKnown || priority >= taskPriority {
			t.Fatalf("optional persona note %q policy = (%d,%v,%v), task rules=(%d,known=%v)", header, priority, required, known, taskPriority, taskKnown)
		}
	}
	for _, header := range []string{promptSectionPersona, promptSectionPersonaState} {
		_, required, known := promptSectionDirectPolicy(header)
		if !known || !required {
			t.Fatalf("core persona section %q must remain required", header)
		}
	}
}

func TestTruncateGuidePreservesUTF8AndTokenLimitIncludingSuffix(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)
	input := strings.Repeat("🌍überprüfung 世界段落\n", 40)
	for _, maxTokens := range []int{0, 1, 2, 3, 8, 20} {
		got := truncateGuide(input, maxTokens)
		if !utf8.ValidString(got) {
			t.Fatalf("maxTokens=%d returned invalid UTF-8: %q", maxTokens, got)
		}
		if CountTokens(got) > maxTokens {
			t.Fatalf("maxTokens=%d result uses %d tokens", maxTokens, CountTokens(got))
		}
	}
	if got := truncateGuide(input, 8); !strings.HasSuffix(got, "[...truncated]") {
		t.Fatalf("ordinary truncated guide omitted in-budget marker: %q", got)
	}
}

func TestTruncateWithEllipsisKeepsUTF8AndByteCapForTinyLimits(t *testing.T) {
	input := "🌍任务を確認する"
	for maxBytes := 1; maxBytes <= 8; maxBytes++ {
		got := TruncateWithEllipsis(input, maxBytes)
		if len(got) > maxBytes || !utf8.ValidString(got) {
			t.Fatalf("maxBytes=%d result=%q bytes=%d valid=%v", maxBytes, got, len(got), utf8.ValidString(got))
		}
	}
}
