package prompts

import (
	"aurago/internal/memory"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolGuideIsOptionalAtomicAcrossOwnHeadings(t *testing.T) {
	flags := &ContextFlags{PredictedGuides: []string{"# Docker Management\n" + strings.Repeat("Required-looking body.\n", 1000) + "# Another Heading\nlate operation"}}
	sections := ToolGuideSections(flags)
	result, err := FitSystemPromptToBudget(context.Background(), PromptFitRequest{Text: "# CORE IDENTITY\nRequired identity.", Tokens: -1, TokenBudget: 120, OptionalSections: sections}, nil)
	if err != nil || result.BudgetExceeded != nil {
		t.Fatalf("guide became required: %+v %v", result, err)
	}
	if strings.Contains(result.Text, "Docker") || len(result.ExposedSections) != 0 || len(result.RemovedSections) != 1 {
		t.Fatalf("guide not atomically removed: %+v", result)
	}
	result, err = FitSystemPromptToBudget(context.Background(), PromptFitRequest{Text: "# CORE IDENTITY\nRequired identity.", Tokens: -1, OptionalSections: sections}, nil)
	if err != nil || len(result.ExposedSections) != 1 || !strings.Contains(result.Text, "late operation") {
		t.Fatal("missing exposure")
	}
}

func TestToolGuideVariantExposureFollowsFinalFit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docker.md")
	if err := os.WriteFile(path, []byte("Canonical docker workflow."), 0600); err != nil {
		t.Fatal(err)
	}
	old := SelectToolGuideVariant
	defer func() { SelectToolGuideVariant = old }()
	SelectToolGuideVariant = func(_, _, key string) ToolGuideVariant {
		if key != "run" {
			t.Fatal("missing assignment key")
		}
		return ToolGuideVariant{Text: strings.Repeat("Selected variant workflow. ", 100), Version: "candidate"}
	}
	guide, _ := ReadToolGuideFull(path)
	sections := ToolGuideSections(&ContextFlags{OptimizerEnabled: true, OptimizerRunID: "run", PredictedGuides: []string{guide}, GuideSources: map[string]string{PromptRevision(guide): path}})
	for _, budget := range []int{100, 10000} {
		result, err := FitSystemPromptToBudget(context.Background(), PromptFitRequest{Text: "# CORE IDENTITY\nRequired identity.", Tokens: -1, TokenBudget: budget, OptionalSections: sections}, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := budget > 100
		if strings.Contains(result.Text, "Selected variant") != want || (len(result.GuideExposures) == 1) != want {
			t.Fatalf("exposure mismatch at budget %d: %+v", budget, result.GuideExposures)
		}
	}
}

func TestSemanticGuideUsesMatchedLateChunkAndRejectsStaleRevision(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "docker.md")
	body := "HEADER_WORKFLOW\n" + strings.Repeat("Long early material. ", 400) + "\nLATE_SPECIFIC_WORKFLOW"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	old := searchDynamicToolGuides
	defer func() { searchDynamicToolGuides = old }()
	for _, fresh := range []bool{true, false} {
		revision := PromptRevision(body)
		chunk := "LATE_SPECIFIC_WORKFLOW"
		if !fresh {
			revision = "outdated"
			chunk = "OUTDATED_UNTRUSTED_CHUNK"
		}
		searchDynamicToolGuides = func(context.Context, memory.VectorDB, string, int) ([]memory.ToolGuideMatch, error) {
			return []memory.ToolGuideMatch{{Path: path, Content: chunk, Revision: revision, ChunkID: "docker:2"}}, nil
		}
		guides := PrepareDynamicGuidesWithStrategyContext(context.Background(), &memory.ChromemVectorDB{}, nil, "specific operation", "", root, nil, nil, 1, DynamicGuideStrategy{PreferSemantics: true}, nil)
		if len(guides) != 1 || strings.Contains(guides[0], "OUTDATED_UNTRUSTED_CHUNK") || (fresh && !strings.Contains(guides[0], "LATE_SPECIFIC_WORKFLOW")) || (!fresh && !strings.Contains(guides[0], "HEADER_WORKFLOW")) {
			t.Fatalf("matched chunk lost or stale: %v %v", fresh, guides)
		}
	}
}

func TestFullToolGuideRetainsLateSectionsAndRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "late.md")
	body := strings.Repeat("A long manual paragraph. ", 4000) + "LATE_SECTION_A"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	guide, ok := ReadToolGuideFull(path)
	if !ok || !strings.Contains(guide, "LATE_SECTION_A") {
		t.Fatal("late section lost")
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(body, "LATE_SECTION_A", "LATE_SECTION_B")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	guide, ok = ReadToolGuideFull(path)
	if !ok || !strings.Contains(guide, "LATE_SECTION_B") {
		t.Fatal("same-size correction remained stale")
	}
}
