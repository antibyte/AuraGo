package prompts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// PromptSection is one typed prompt ledger entry. Priority is lower for
// sections that are shed earlier; Required sections are never section-shed.
type PromptSection struct {
	ID       string
	GroupID  string
	Text     string
	Priority int
	Required bool
	Tokens   int
}

// PromptDocument is the typed, token-accounted representation of a built
// system prompt.
type PromptDocument struct {
	Sections    []PromptSection
	TotalTokens int
	Revision    string
}

// PromptBuildResult extends the legacy text/token pair without changing the
// existing BuildSystemPrompt APIs.
type PromptBuildResult struct {
	Text            string
	Tokens          int
	RemovedSections []string
	Revision        string
	Document        PromptDocument
	BudgetExceeded  *PromptBudgetExceededError
}

// PromptBudgetExceededError reports that the mandatory system-prompt
// sections cannot fit. It intentionally carries no prompt text.
type PromptBudgetExceededError struct {
	Budget         int
	RequiredTokens int
}

func (e *PromptBudgetExceededError) Error() string {
	if e == nil {
		return "prompt_budget_exceeded"
	}
	return fmt.Sprintf("prompt_budget_exceeded: required=%d budget=%d", e.RequiredTokens, e.Budget)
}

func (e *PromptBudgetExceededError) Code() string { return "prompt_budget_exceeded" }

// DescribePromptDocument builds a section ledger for already-cached prompt
// text without rebuilding any prompt sources.
func DescribePromptDocument(text, model string, totalTokens int) PromptDocument {
	return newPromptDocumentContext(context.Background(), text, model, totalTokens)
}

type promptBuildDetailsCollector struct {
	removed []string
}

type promptBuildDetailsContextKey struct{}

// budgetShedFullTokenizeHook is test-only instrumentation for the ledger's
// full-document tokenization count. Section tokenizations do not invoke it.
var budgetShedFullTokenizeHook func()

func noteBudgetShedFullTokenization() {
	if budgetShedFullTokenizeHook != nil {
		budgetShedFullTokenizeHook()
	}
}

func promptBuildContextWithCollector(ctx context.Context, collector *promptBuildDetailsCollector) context.Context {
	return context.WithValue(ctx, promptBuildDetailsContextKey{}, collector)
}

func recordRemovedPromptSections(ctx context.Context, removed []string) {
	collector, _ := ctx.Value(promptBuildDetailsContextKey{}).(*promptBuildDetailsCollector)
	if collector == nil {
		return
	}
	collector.removed = append([]string(nil), removed...)
}

func newPromptDocumentContext(ctx context.Context, text, model string, totalTokens int) PromptDocument {
	if totalTokens < 0 {
		totalTokens = countTokensWithModelContext(ctx, text, model)
	}
	sections := splitPromptSections(text)
	for i := range sections {
		sections[i].Tokens = countTokensWithModelContext(ctx, sections[i].Text, model)
	}
	digest := sha256.Sum256([]byte(text))
	return PromptDocument{
		Sections:    sections,
		TotalTokens: totalTokens,
		Revision:    hex.EncodeToString(digest[:8]),
	}
}

func (d PromptDocument) sectionTokenLedger() map[string][]int {
	ledger := make(map[string][]int, len(d.Sections))
	for _, section := range d.Sections {
		ledger[section.ID] = append(ledger[section.ID], section.Tokens)
	}
	return ledger
}

func takeSectionTokens(ledger map[string][]int, id string) int {
	entries := ledger[id]
	if len(entries) == 0 {
		return 0
	}
	value := entries[0]
	if len(entries) == 1 {
		delete(ledger, id)
	} else {
		ledger[id] = entries[1:]
	}
	return value
}

func splitPromptSections(text string) []PromptSection {
	if text == "" {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	sections := make([]PromptSection, 0, 24)
	var current strings.Builder
	currentID := "preamble"
	currentGroupID := "preamble"
	currentPriority := 1000
	currentRequired := true
	type headingPolicy struct {
		level    int
		groupID  string
		priority int
		required bool
	}
	stack := make([]headingPolicy, 0, 6)
	flush := func() {
		if current.Len() == 0 {
			return
		}
		sections = append(sections, PromptSection{
			ID:       currentID,
			GroupID:  currentGroupID,
			Text:     current.String(),
			Priority: currentPriority,
			Required: currentRequired,
		})
		current.Reset()
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		level := markdownHeadingLevel(trimmed)
		if level > 0 {
			flush()
			currentID = trimmed
			for len(stack) > 0 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}
			priority, required, known := promptSectionDirectPolicy(currentID)
			groupID := currentID
			if !known && len(stack) > 0 && !stack[len(stack)-1].required {
				parent := stack[len(stack)-1]
				priority, required, groupID = parent.priority, false, parent.groupID
			}
			currentPriority, currentRequired, currentGroupID = priority, required, groupID
			stack = append(stack, headingPolicy{level: level, groupID: groupID, priority: priority, required: required})
		}
		current.WriteString(line)
	}
	flush()
	return sections
}

func isMarkdownHeading(line string) bool {
	return markdownHeadingLevel(line) > 0
}

func markdownHeadingLevel(line string) int {
	if !strings.HasPrefix(line, "#") {
		return 0
	}
	index := 0
	for index < len(line) && line[index] == '#' {
		index++
	}
	if index > 0 && index <= 6 && index < len(line) && line[index] == ' ' {
		return index
	}
	return 0
}

func promptSectionPolicy(id string) (int, bool) {
	priority, required, _ := promptSectionDirectPolicy(id)
	return priority, required
}

func promptSectionDirectPolicy(id string) (int, bool, bool) {
	for priority, header := range promptOptionalHeaders(false) {
		if id == header {
			return priority, false, true
		}
	}
	for priority, header := range promptOptionalHeaders(true) {
		if id == header {
			return priority, false, true
		}
	}
	if id == "# RETRIEVED MEMORIES" {
		return 100, false, true
	}
	return 1000, true, false
}

type promptSectionGroup struct {
	ID       string
	Priority int
	Tokens   int
}

func (d PromptDocument) optionalGroups() []promptSectionGroup {
	byID := make(map[string]*promptSectionGroup)
	order := make([]string, 0, len(d.Sections))
	for _, section := range d.Sections {
		if section.Required {
			continue
		}
		id := section.GroupID
		if id == "" {
			id = section.ID
		}
		group := byID[id]
		if group == nil {
			group = &promptSectionGroup{ID: id, Priority: section.Priority}
			byID[id] = group
			order = append(order, id)
		}
		group.Tokens += section.Tokens
	}
	groups := make([]promptSectionGroup, 0, len(order))
	for _, id := range order {
		groups = append(groups, *byID[id])
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Priority < groups[j].Priority })
	return groups
}

func (d PromptDocument) estimatedTokens() int {
	total := 0
	for _, section := range d.Sections {
		total += section.Tokens
	}
	return total
}

func (d PromptDocument) renderWithoutGroups(removed map[string]bool) string {
	var result strings.Builder
	for _, section := range d.Sections {
		groupID := section.GroupID
		if groupID == "" {
			groupID = section.ID
		}
		if !removed[groupID] {
			result.WriteString(section.Text)
		}
	}
	return result.String()
}

func promptOptionalHeaders(unifiedMemory bool) []string {
	headers := []string{"# TOOL GUIDES"}
	if unifiedMemory {
		headers = append(headers, "## USER PROFILING", "# UNIFIED MEMORY CONTEXT")
	} else {
		headers = append(headers, "# PREDICTED CONTEXT", "# LAST 7 DAYS OVERVIEW", "## USER PROFILING")
	}
	return append(headers,
		"# RELEVANT KNOWLEDGE",
		"# KNOWN ERROR PATTERNS",
		"# LEARNED RULES",
		"# REUSE-FIRST CONTEXT",
		"### ACTIVE REMINDERS",
		"### PLANNER CONTEXT",
		"### DAILY TODO REMINDER",
		"### OPERATIONAL ISSUE REMINDER",
		"### ACTIVE TASK LIST",
		"# OUTGOING WEBHOOKS",
		"# TASK RULES",
		"# HOMEPAGE DESIGN SYSTEM",
		"# AGENT SKILLS CATALOG",
		"### PERSONA SIGNALS",
		"### INNER VOICE",
		"### CURRENT EMOTIONAL STATE & MOOD",
		"### CURRENT PERSONALITY TRAITS",
		"# PERSONA",
		"# YOUR PERSONALITY",
	)
}

func removedTextBetween(before, after string) string {
	if len(after) >= len(before) {
		return ""
	}
	prefix := 0
	for prefix < len(after) && before[prefix] == after[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(after)-prefix &&
		before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	end := len(before) - suffix
	if end < prefix {
		return ""
	}
	return before[prefix:end]
}

func trimRetrievedMemoriesLedgerContext(ctx context.Context, prompt string, budget, estimatedTokens int, model string) (string, bool, bool, int, error) {
	const header = "# RETRIEVED MEMORIES"
	const separator = "\n---\n"
	if err := promptContextErr(ctx); err != nil {
		return "", false, false, estimatedTokens, err
	}
	index := strings.Index(prompt, header)
	if index < 0 || (index > 0 && prompt[index-1] != '\n') || isInsideCodeBlock(prompt, index) {
		return prompt, false, false, estimatedTokens, nil
	}
	rest := prompt[index+len(header):]
	nextHeader := -1
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\n' && i+1 < len(rest) && rest[i+1] == '#' {
			j := i + 1
			for j < len(rest) && rest[j] == '#' {
				j++
			}
			if j < len(rest) && rest[j] == ' ' {
				nextHeader = i + 1
				break
			}
		}
	}
	sectionContent, afterSection := rest, ""
	if nextHeader >= 0 {
		sectionContent, afterSection = rest[:nextHeader], rest[nextHeader:]
	}
	beforeSection := prompt[:index]
	rawEntries := strings.Split(strings.TrimSpace(sectionContent), separator)
	entries := make([]string, 0, len(rawEntries))
	for _, entry := range rawEntries {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			entries = append(entries, trimmed)
		}
	}
	if len(entries) == 0 {
		updated := removeSection(prompt, header)
		removedTokens := countTokensWithModelContext(ctx, removedTextBetween(prompt, updated), model)
		return updated, false, true, maxInt(0, estimatedTokens-removedTokens), nil
	}

	originalCount := len(entries)
	for estimatedTokens > budget && len(entries) > 0 {
		last := len(entries) - 1
		removedTokens := countTokensWithModelContext(ctx, entries[last], model)
		if last > 0 {
			removedTokens += countTokensWithModelContext(ctx, separator, model)
		}
		estimatedTokens = maxInt(0, estimatedTokens-removedTokens)
		entries = entries[:last]
	}
	if len(entries) == originalCount {
		return prompt, false, false, estimatedTokens, nil
	}
	if len(entries) == 0 {
		updated := strings.TrimRight(beforeSection, "\n ") + "\n\n" + afterSection
		return updated, false, true, estimatedTokens, nil
	}
	updated := beforeSection + header + "\n" + strings.Join(entries, separator) + "\n\n" + afterSection
	return updated, true, false, estimatedTokens, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
