package prompts

import (
	"context"
	"sync/atomic"
	"testing"

	"aurago/internal/memory"
)

func TestPrepareDynamicGuidesSkipsSearchWhenAllowedSetIsFullySkipped(t *testing.T) {
	original := searchDynamicToolGuides
	defer func() { searchDynamicToolGuides = original }()
	var calls atomic.Int32
	searchDynamicToolGuides = func(context.Context, memory.VectorDB, string, int) ([]string, error) {
		calls.Add(1)
		return nil, nil
	}

	got := PrepareDynamicGuidesWithStrategyContext(
		context.Background(),
		&memory.ChromemVectorDB{},
		nil,
		"use docker",
		"",
		t.TempDir(),
		nil,
		nil,
		3,
		DynamicGuideStrategy{AllowedTools: []string{"docker", "shell"}, SkipTools: []string{"docker", "shell"}},
		nil,
	)
	if len(got) != 0 || calls.Load() != 0 {
		t.Fatalf("guides=%v search_calls=%d, want none/0", got, calls.Load())
	}
}

func TestPrepareDynamicGuidesHonorsCancelledRequestBeforeSearch(t *testing.T) {
	original := searchDynamicToolGuides
	defer func() { searchDynamicToolGuides = original }()
	var calls atomic.Int32
	searchDynamicToolGuides = func(context.Context, memory.VectorDB, string, int) ([]string, error) {
		calls.Add(1)
		return nil, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := PrepareDynamicGuidesWithStrategyContext(
		ctx, &memory.ChromemVectorDB{}, nil, "query", "", t.TempDir(), nil, nil, 3, DynamicGuideStrategy{}, nil,
	)
	if len(got) != 0 || calls.Load() != 0 {
		t.Fatalf("guides=%v search_calls=%d, want none/0", got, calls.Load())
	}
}
