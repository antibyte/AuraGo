package prompts

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestBudgetShed_RemovesUserProfilingSection(t *testing.T) {
	// Create a prompt with the ## USER PROFILING section that should be removed
	prompt := `# SYSTEM IDENTITY
You are AuraGo.

# TOOL GUIDES
tool guide content here

## USER PROFILING
Your goal: build a comprehensive user profile over time.

### Known User Profile
User name: Test User

# RETRIEVED MEMORIES
memory entry 1

# NOW
2026-04-09 12:00`

	flags := ContextFlags{
		Tier:        "full",
		TokenBudget: 5, // Very small budget - should shed everything possible
	}

	logger := slog.Default()
	result, shedSections := budgetShed(prompt, flags, "", "", time.Now(), logger)

	// Check that ## USER PROFILING was shed
	foundProfiling := false
	for _, section := range shedSections {
		if section == "## USER PROFILING" {
			foundProfiling = true
			break
		}
	}
	if !foundProfiling {
		t.Errorf("expected ## USER PROFILING to be in shedSections, got: %v", shedSections)
	}

	// Verify the section is actually removed from the result
	if strings.Contains(result, "## USER PROFILING") {
		t.Errorf("expected ## USER PROFILING to be removed from result, but it was found")
	}
	if strings.Contains(result, "Your goal: build a comprehensive user profile") {
		t.Errorf("expected user profiling content to be removed from result")
	}
}

func TestBudgetShed_HardTruncateWhenCoreExceedsBudget(t *testing.T) {
	prompt := strings.Repeat("word ", 5000)

	flags := ContextFlags{
		Tier:        "minimal",
		TokenBudget: 10,
	}

	logger := slog.Default()
	result, shedSections := budgetShed(prompt, flags, "", "", time.Now(), logger)

	if !strings.Contains(result, "[BUDGET TRUNCATED]") {
		t.Errorf("expected hard-truncate marker, got result len=%d", len(result))
	}

	foundHardTruncate := false
	for _, s := range shedSections {
		if s == "HARD_TRUNCATE" {
			foundHardTruncate = true
		}
	}
	if !foundHardTruncate {
		t.Errorf("expected HARD_TRUNCATE in shedSections, got: %v", shedSections)
	}

	if len(result) >= len(prompt) {
		t.Errorf("expected result to be shorter than original, got len=%d vs orig=%d", len(result), len(prompt))
	}
}

func TestBudgetShed_UnifiedMemoryRemovesUserProfile(t *testing.T) {
	// Test with UnifiedMemoryBlock enabled
	prompt := `# SYSTEM IDENTITY
You are AuraGo.

## USER PROFILING
Your goal: build a comprehensive user profile.

### Known User Profile
User name: Test User

# UNIFIED MEMORY CONTEXT
## Recent Activity
some activity`

	flags := ContextFlags{
		Tier:               "compact",
		TokenBudget:        5,
		UnifiedMemoryBlock: true,
	}

	logger := slog.Default()
	result, shedSections := budgetShed(prompt, flags, "", "", time.Now(), logger)

	// Check that ## USER PROFILING was shed (under UnifiedMemoryBlock path)
	foundProfiling := false
	for _, section := range shedSections {
		if section == "## USER PROFILING" {
			foundProfiling = true
			break
		}
	}
	if !foundProfiling {
		t.Errorf("expected ## USER PROFILING to be in shedSections (unified path), got: %v", shedSections)
	}

	// Verify the section is actually removed
	if strings.Contains(result, "## USER PROFILING") {
		t.Errorf("expected ## USER PROFILING to be removed from result")
	}
}
