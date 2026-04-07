package agent

import (
	"sync"
	"testing"
)

func TestParseWorkflowPlan(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantTools    []string
		wantStripped string
	}{
		{
			name:         "No tag",
			content:      "Hello, I will search for that.",
			wantTools:    nil,
			wantStripped: "Hello, I will search for that.",
		},
		{
			name:         "Single tool",
			content:      `<workflow_plan>["docker"]</workflow_plan>`,
			wantTools:    []string{"docker"},
			wantStripped: "",
		},
		{
			name:         "Multiple tools",
			content:      `I'll work on this now. <workflow_plan>["docker", "koofr", "webdav"]</workflow_plan> Let me start.`,
			wantTools:    []string{"docker", "koofr", "webdav"},
			wantStripped: "I'll work on this now.  Let me start.",
		},
		{
			name:         "Five tools max",
			content:      `<workflow_plan>["a","b","c","d","e","f","g"]</workflow_plan>`,
			wantTools:    []string{"a", "b", "c", "d", "e"},
			wantStripped: "",
		},
		{
			name:         "Malformed JSON fallback",
			content:      `<workflow_plan>[docker, koofr]</workflow_plan>`,
			wantTools:    []string{"docker", "koofr"},
			wantStripped: "",
		},
		{
			name:         "Empty array",
			content:      `<workflow_plan>[]</workflow_plan>`,
			wantTools:    nil,
			wantStripped: `<workflow_plan>[]</workflow_plan>`,
		},
		{
			name:         "Missing close tag",
			content:      `<workflow_plan>["docker"]`,
			wantTools:    nil,
			wantStripped: `<workflow_plan>["docker"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTools, gotStripped := parseWorkflowPlan(tt.content)

			if tt.wantTools == nil {
				if gotTools != nil {
					t.Errorf("parseWorkflowPlan() tools = %v, want nil", gotTools)
				}
			} else {
				if len(gotTools) != len(tt.wantTools) {
					t.Fatalf("parseWorkflowPlan() tools len = %d, want %d", len(gotTools), len(tt.wantTools))
				}
				for i, tool := range gotTools {
					if tool != tt.wantTools[i] {
						t.Errorf("parseWorkflowPlan() tools[%d] = %q, want %q", i, tool, tt.wantTools[i])
					}
				}
			}

			if gotStripped != tt.wantStripped {
				t.Errorf("parseWorkflowPlan() stripped = %q, want %q", gotStripped, tt.wantStripped)
			}
		})
	}
}

func TestGlobalTokenCountAtomic(t *testing.T) {
	initial := GlobalTokenCount()
	AddGlobalTokenCount(100)
	if got := GlobalTokenCount(); got != initial+100 {
		t.Fatalf("GlobalTokenCount() = %d, want %d", got, initial+100)
	}
	AddGlobalTokenCount(-100)
	if got := GlobalTokenCount(); got != initial {
		t.Fatalf("GlobalTokenCount() = %d, want %d", got, initial)
	}
}

func TestGlobalTokenEstimatedAtomic(t *testing.T) {
	SetGlobalTokenEstimated(false)
	if got := GlobalTokenEstimated(); got != false {
		t.Fatalf("GlobalTokenEstimated() = %v, want false", got)
	}
	SetGlobalTokenEstimated(true)
	if got := GlobalTokenEstimated(); got != true {
		t.Fatalf("GlobalTokenEstimated() = %v, want true", got)
	}
	SetGlobalTokenEstimated(false)
}

func TestGlobalTokenCountConcurrent(t *testing.T) {
	const n = 100
	const inc = 10
	var wg sync.WaitGroup
	wg.Add(n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < inc; j++ {
				AddGlobalTokenCount(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	expected := GlobalTokenCount()
	if expected != n*inc {
		t.Errorf("GlobalTokenCount() = %d after %d goroutines × %d increments, want %d", expected, n, inc, n*inc)
	}
}
