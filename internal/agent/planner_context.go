package agent

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"aurago/internal/planner"

	openai "github.com/sashabaranov/go-openai"
)

type plannerMemorySearchResult struct {
	Summary          string
	Todos            []planner.Todo
	Appointments     []planner.Appointment
	OpenTodoCount    int
	OverdueTodoCount int
}

var plannerIntentKeywords = []string{
	"todo", "todos", "task", "tasks", "aufgabe", "aufgaben",
	"termin", "termine", "appointment", "appointments", "meeting",
	"deadline", "deadlines", "calendar", "reminder", "reminders",
	"erinner", "heute", "morgen", "today", "tomorrow", "next week",
	"nächste woche", "was steht an", "what's due", "what is due",
	"was ist offen", "what is open",
}

var plannerTodoOverviewKeywords = []string{
	"todo", "todos", "task", "tasks", "aufgabe", "aufgaben",
	"offen", "open", "due", "fällig", "deadline", "deadlines",
}

var plannerAppointmentOverviewKeywords = []string{
	"termin", "termine", "appointment", "appointments", "meeting", "calendar",
	"heute", "morgen", "today", "tomorrow", "next week", "nächste woche",
	"was steht an", "what's on", "what is on",
}

func isFirstUserMessageInSession(messages []openai.ChatCompletionMessage) bool {
	nonSystemMessages := 0
	userMessages := 0
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleSystem {
			continue
		}
		if strings.TrimSpace(messageText(msg)) == "" && len(msg.ToolCalls) == 0 {
			continue
		}
		nonSystemMessages++
		if msg.Role == openai.ChatMessageRoleUser {
			userMessages++
		}
	}
	return nonSystemMessages == 1 && userMessages == 1
}

func containsPlannerKeyword(query string, keywords []string) bool {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return false
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func hasPlannerIntent(query string) bool {
	return containsPlannerKeyword(query, plannerIntentKeywords)
}

func hasPlannerTodoOverviewIntent(query string) bool {
	return containsPlannerKeyword(query, plannerTodoOverviewKeywords)
}

func hasPlannerAppointmentOverviewIntent(query string) bool {
	return containsPlannerKeyword(query, plannerAppointmentOverviewKeywords)
}

func shouldInjectPlannerContext(runCfg RunConfig, initialUserMsg string, isFirstTurn bool) bool {
	if runCfg.PlannerDB == nil || strings.TrimSpace(initialUserMsg) == "" {
		return false
	}
	if runCfg.IsCoAgent || runCfg.IsMission || runCfg.IsMaintenance {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(runCfg.MessageSource)) {
	case "mission", "a2a":
		return false
	}
	return isFirstTurn || hasPlannerIntent(initialUserMsg)
}

func plannerPromptContextText(runCfg RunConfig, initialUserMsg string, now time.Time, isFirstTurn bool, logger *slog.Logger) string {
	if !shouldInjectPlannerContext(runCfg, initialUserMsg, isFirstTurn) {
		return ""
	}
	snapshot, err := planner.BuildPromptSnapshot(runCfg.PlannerDB, now, planner.PromptSnapshotOptions{
		TodoLimit:         10,
		AppointmentLimit:  5,
		AppointmentWindow: 48 * time.Hour,
	})
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to build planner prompt context", "error", err)
		}
		return ""
	}
	return planner.BuildPromptContextText(snapshot)
}

func parseAgentPlannerTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts, true
	}
	if ts, err := time.Parse("2006-01-02", value); err == nil {
		return ts, true
	}
	return time.Time{}, false
}

func isPlannerTodoOverdue(todo planner.Todo, now time.Time) bool {
	if todo.Status != "open" && todo.Status != "in_progress" {
		return false
	}
	due, ok := parseAgentPlannerTime(todo.DueDate)
	if !ok {
		return false
	}
	return due.Before(now)
}

func plannerPriorityRank(priority string) int {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "high":
		return 0
	case "medium":
		return 1
	case "low":
		return 2
	default:
		return 3
	}
}

func sortPlannerTodos(todos []planner.Todo, now time.Time) {
	sort.SliceStable(todos, func(i, j int) bool {
		leftOverdue := isPlannerTodoOverdue(todos[i], now)
		rightOverdue := isPlannerTodoOverdue(todos[j], now)
		if leftOverdue != rightOverdue {
			return leftOverdue
		}
		leftPriority := plannerPriorityRank(todos[i].Priority)
		rightPriority := plannerPriorityRank(todos[j].Priority)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		leftDue, leftHasDue := parseAgentPlannerTime(todos[i].DueDate)
		rightDue, rightHasDue := parseAgentPlannerTime(todos[j].DueDate)
		if leftHasDue != rightHasDue {
			return leftHasDue
		}
		if leftHasDue && !leftDue.Equal(rightDue) {
			return leftDue.Before(rightDue)
		}
		leftCreated, leftHasCreated := parseAgentPlannerTime(todos[i].CreatedAt)
		rightCreated, rightHasCreated := parseAgentPlannerTime(todos[j].CreatedAt)
		if leftHasCreated && rightHasCreated && !leftCreated.Equal(rightCreated) {
			return leftCreated.Before(rightCreated)
		}
		return todos[i].Title < todos[j].Title
	})
}

func searchPlannerTodos(db *sql.DB, query string, now time.Time, limit int) ([]planner.Todo, int, int, error) {
	allTodos, err := planner.ListTodos(db, strings.TrimSpace(query), "")
	if err != nil {
		return nil, 0, 0, err
	}
	filtered := make([]planner.Todo, 0, len(allTodos))
	overdueCount := 0
	for _, todo := range allTodos {
		if todo.Status != "open" && todo.Status != "in_progress" {
			continue
		}
		if isPlannerTodoOverdue(todo, now) {
			overdueCount++
		}
		filtered = append(filtered, todo)
	}
	sortPlannerTodos(filtered, now)
	openCount := len(filtered)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, openCount, overdueCount, nil
}

func searchPlannerAppointments(db *sql.DB, query string, from, to time.Time, limit int, includePast bool) ([]planner.Appointment, error) {
	status := ""
	if !includePast {
		status = "upcoming"
	}
	list, err := planner.ListAppointments(db, strings.TrimSpace(query), status)
	if err != nil {
		return nil, err
	}
	filtered := make([]planner.Appointment, 0, len(list))
	for _, appointment := range list {
		if appointment.Status == "cancelled" {
			continue
		}
		when, ok := parseAgentPlannerTime(appointment.DateTime)
		if !ok {
			continue
		}
		if !from.IsZero() && when.Before(from) {
			continue
		}
		if !to.IsZero() && when.After(to) {
			continue
		}
		filtered = append(filtered, appointment)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left, _ := parseAgentPlannerTime(filtered[i].DateTime)
		right, _ := parseAgentPlannerTime(filtered[j].DateTime)
		if !left.Equal(right) {
			return left.Before(right)
		}
		return filtered[i].Title < filtered[j].Title
	})
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func buildPlannerSummary(result plannerMemorySearchResult) string {
	parts := make([]string, 0, 2)
	if result.OpenTodoCount > 0 {
		part := fmt.Sprintf("%d open todos", result.OpenTodoCount)
		if result.OverdueTodoCount > 0 {
			part += fmt.Sprintf(" (%d overdue)", result.OverdueTodoCount)
		}
		parts = append(parts, part)
	}
	if len(result.Appointments) > 0 {
		parts = append(parts, fmt.Sprintf("%d relevant appointments", len(result.Appointments)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Planner overview: " + strings.Join(parts, "; ")
}

func searchPlannerMemory(db *sql.DB, query, timeRange string, now time.Time, todoLimit, appointmentLimit int) (plannerMemorySearchResult, error) {
	result := plannerMemorySearchResult{}
	if db == nil {
		return result, nil
	}
	query = strings.TrimSpace(query)
	lowerQuery := strings.ToLower(query)

	todoOverview := query == "" || hasPlannerTodoOverviewIntent(lowerQuery) || timeRange == "today"
	appointmentOverview := query == "" || hasPlannerAppointmentOverviewIntent(lowerQuery) || timeRange == "today"

	if todoLimit <= 0 {
		todoLimit = 5
	}
	if appointmentLimit <= 0 {
		appointmentLimit = 3
	}

	includePastAppointments := false
	from := time.Time{}
	to := time.Time{}
	switch strings.ToLower(strings.TrimSpace(timeRange)) {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		from = start
		to = start.Add(24 * time.Hour)
	case "last_week":
		from = now.AddDate(0, 0, -7)
		to = now
		includePastAppointments = true
	case "last_month":
		from = now.AddDate(0, 0, -30)
		to = now
		includePastAppointments = true
	default:
		if appointmentOverview {
			from = now
			to = now.Add(48 * time.Hour)
		}
	}

	todoQuery := query
	if todoOverview {
		todoQuery = ""
	}
	todos, openCount, overdueCount, err := searchPlannerTodos(db, todoQuery, now, todoLimit)
	if err != nil {
		return result, err
	}
	result.Todos = todos
	result.OpenTodoCount = openCount
	result.OverdueTodoCount = overdueCount

	appointmentQuery := query
	if appointmentOverview {
		appointmentQuery = ""
	}
	if appointmentOverview || hasPlannerIntent(lowerQuery) || timeRange != "" {
		appointments, err := searchPlannerAppointments(db, appointmentQuery, from, to, appointmentLimit, includePastAppointments)
		if err != nil {
			return result, err
		}
		result.Appointments = appointments
	}

	if todoOverview || appointmentOverview || timeRange != "" {
		result.Summary = buildPlannerSummary(result)
	}
	return result, nil
}
