package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/contacts"
	"aurago/internal/planner"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func newPlannerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("planner.InitDB() error = %v", err)
	}
	return db
}

func decodeToolOutput(t *testing.T, output string) map[string]interface{} {
	t.Helper()
	trimmed := strings.TrimPrefix(output, "Tool Output: ")
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", output, err)
	}
	return payload
}

func TestDispatchManageTodosSupportsChecklistOperations(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	addResp := dispatchManageTodos(ToolCall{
		Operation: "add",
		Params: map[string]interface{}{
			"title":        "Launch release",
			"priority":     "high",
			"status":       "open",
			"remind_daily": true,
			"items": []interface{}{
				map[string]interface{}{"title": "Prepare changelog"},
			},
		},
	}, db, nil, slog.Default())
	addPayload := decodeToolOutput(t, addResp)
	todoID, _ := addPayload["id"].(string)
	if todoID == "" {
		t.Fatalf("add todo response missing id: %#v", addPayload)
	}

	addItemResp := dispatchManageTodos(ToolCall{
		Operation: "add_item",
		Params: map[string]interface{}{
			"id":           todoID,
			"item_title":   "Publish binaries",
			"item_is_done": false,
		},
	}, db, nil, slog.Default())
	itemPayload := decodeToolOutput(t, addItemResp)
	itemID, _ := itemPayload["item_id"].(string)
	if itemID == "" {
		t.Fatalf("add_item response missing item_id: %#v", itemPayload)
	}

	toggleResp := dispatchManageTodos(ToolCall{
		Operation: "toggle_item",
		Params: map[string]interface{}{
			"id":           todoID,
			"item_id":      itemID,
			"item_is_done": true,
		},
	}, db, nil, slog.Default())
	togglePayload := decodeToolOutput(t, toggleResp)
	if done, ok := togglePayload["item_is_done"].(bool); !ok || !done {
		t.Fatalf("toggle_item response = %#v, want item_is_done=true", togglePayload)
	}

	completeResp := dispatchManageTodos(ToolCall{
		Operation: "complete",
		Params: map[string]interface{}{
			"id":                 todoID,
			"complete_items_too": true,
		},
	}, db, nil, slog.Default())
	completePayload := decodeToolOutput(t, completeResp)
	if status, _ := completePayload["status"].(string); status != "success" {
		t.Fatalf("complete response = %#v, want success", completePayload)
	}

	todo, err := planner.GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("planner.GetTodo() error = %v", err)
	}
	if !todo.RemindDaily {
		t.Fatal("RemindDaily = false, want true")
	}
	if todo.Status != "done" || todo.ProgressPercent != 100 {
		t.Fatalf("todo status/progress = %q/%d, want done/100", todo.Status, todo.ProgressPercent)
	}
	if todo.DoneItemCount != todo.ItemCount {
		t.Fatalf("done/item count = %d/%d, want all done", todo.DoneItemCount, todo.ItemCount)
	}
}

func TestDailyTodoReminderTextOnlyOnEligibleFirstContact(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	if _, err := planner.CreateTodo(db, planner.Todo{
		Title:       "Daily review",
		Priority:    "medium",
		Status:      "open",
		RemindDaily: true,
		Items: []planner.TodoItem{
			{Title: "Inbox zero"},
		},
	}); err != nil {
		t.Fatalf("planner.CreateTodo() error = %v", err)
	}

	now := time.Date(2026, 4, 16, 8, 0, 0, 0, time.UTC)
	runCfg := RunConfig{PlannerDB: db, MessageSource: "web_chat"}

	first := dailyTodoReminderText(runCfg, "Guten Morgen", now, slog.Default())
	if !strings.Contains(first, "You currently have 1 open todos") || !strings.Contains(first, "Daily review") {
		t.Fatalf("first reminder = %q, want summary and todo title", first)
	}

	second := dailyTodoReminderText(runCfg, "Noch mal hallo", now.Add(time.Hour), slog.Default())
	if second != "" {
		t.Fatalf("second reminder = %q, want empty on same day", second)
	}

	blocked := dailyTodoReminderText(RunConfig{PlannerDB: db, MessageSource: "mission", IsMission: true}, "hello", now.Add(24*time.Hour), slog.Default())
	if blocked != "" {
		t.Fatalf("mission reminder = %q, want empty", blocked)
	}
}

func TestDispatchManageAppointmentsSupportsContactParticipants(t *testing.T) {
	plannerDB := newPlannerTestDB(t)
	defer plannerDB.Close()
	contactsDB, err := contacts.InitDB(filepath.Join(t.TempDir(), "contacts.db"))
	if err != nil {
		t.Fatalf("contacts.InitDB() error = %v", err)
	}
	defer contactsDB.Close()

	adaID, err := contacts.Create(contactsDB, contacts.Contact{
		Name:         "Ada Lovelace",
		Email:        "ada@example.com",
		Relationship: "friend",
	})
	if err != nil {
		t.Fatalf("contacts.Create(Ada) error = %v", err)
	}
	graceID, err := contacts.Create(contactsDB, contacts.Contact{
		Name:         "Grace Hopper",
		Email:        "grace@example.com",
		Relationship: "colleague",
	})
	if err != nil {
		t.Fatalf("contacts.Create(Grace) error = %v", err)
	}

	addResp := dispatchManageAppointments(ToolCall{
		Operation: "add",
		Params: map[string]interface{}{
			"title":       "Planning sync",
			"date_time":   "2099-05-01T10:00:00Z",
			"contact_ids": []interface{}{adaID},
		},
	}, plannerDB, contactsDB, nil, slog.Default())
	addPayload := decodeToolOutput(t, addResp)
	appointmentID, _ := addPayload["id"].(string)
	if appointmentID == "" {
		t.Fatalf("add appointment response missing id: %#v", addPayload)
	}
	ids, err := planner.GetAppointmentContactIDs(plannerDB, appointmentID)
	if err != nil {
		t.Fatalf("planner.GetAppointmentContactIDs() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != adaID {
		t.Fatalf("contact ids after add = %#v, want [%s]", ids, adaID)
	}

	updateResp := dispatchManageAppointments(ToolCall{
		Operation: "update",
		Params: map[string]interface{}{
			"id":          appointmentID,
			"contact_ids": []interface{}{graceID},
		},
	}, plannerDB, contactsDB, nil, slog.Default())
	updatePayload := decodeToolOutput(t, updateResp)
	if status, _ := updatePayload["status"].(string); status != "success" {
		t.Fatalf("update response = %#v, want success", updatePayload)
	}
	ids, err = planner.GetAppointmentContactIDs(plannerDB, appointmentID)
	if err != nil {
		t.Fatalf("planner.GetAppointmentContactIDs() after update error = %v", err)
	}
	if len(ids) != 1 || ids[0] != graceID {
		t.Fatalf("contact ids after update = %#v, want [%s]", ids, graceID)
	}

	getResp := dispatchManageAppointments(ToolCall{
		Operation: "get",
		Params:    map[string]interface{}{"id": appointmentID},
	}, plannerDB, contactsDB, nil, slog.Default())
	getPayload := decodeToolOutput(t, getResp)
	appointment, ok := getPayload["appointment"].(map[string]interface{})
	if !ok {
		t.Fatalf("get appointment payload = %#v, want appointment object", getPayload)
	}
	contactIDs, _ := appointment["contact_ids"].([]interface{})
	if len(contactIDs) != 1 || contactIDs[0] != graceID {
		t.Fatalf("get contact_ids = %#v, want [%s]", contactIDs, graceID)
	}
	participants, _ := appointment["participants"].([]interface{})
	if len(participants) != 1 {
		t.Fatalf("get participants = %#v, want one participant", participants)
	}
	participant, _ := participants[0].(map[string]interface{})
	if participant["name"] != "Grace Hopper" || participant["email"] != "grace@example.com" {
		t.Fatalf("get participant = %#v, want Grace Hopper", participant)
	}

	listResp := dispatchManageAppointments(ToolCall{
		Operation: "list",
		Params:    map[string]interface{}{"status": "upcoming"},
	}, plannerDB, contactsDB, nil, slog.Default())
	listPayload := decodeToolOutput(t, listResp)
	appointments, _ := listPayload["appointments"].([]interface{})
	if len(appointments) != 1 {
		t.Fatalf("list appointments = %#v, want one appointment", appointments)
	}
	listAppointment, _ := appointments[0].(map[string]interface{})
	listParticipants, _ := listAppointment["participants"].([]interface{})
	if len(listParticipants) != 1 {
		t.Fatalf("list participants = %#v, want one participant", listParticipants)
	}
	listParticipant, _ := listParticipants[0].(map[string]interface{})
	if listParticipant["name"] != "Grace Hopper" {
		t.Fatalf("list participant = %#v, want Grace Hopper", listParticipant)
	}
}

func TestOperationalIssueNoticeIsDeliveredWithoutLLMCooperation(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{
		Source:    "maintenance",
		Context:   "maintenance",
		Title:     "Maintenance agent loop failed",
		Detail:    "budget exceeded",
		Severity:  "error",
		Reference: "daily_maintenance",
	}); err != nil {
		t.Fatalf("planner.RecordOperationalIssue() error = %v", err)
	}

	runCfg := RunConfig{PlannerDB: db, MessageSource: "web_chat"}
	state := prepareOperationalIssueNotice(runCfg, "Hallo", slog.Default())
	if !strings.Contains(state.Text, "Maintenance agent loop failed") || !strings.Contains(state.Text, "budget exceeded") {
		t.Fatalf("prepared operational notice = %q, want issue title and detail", state.Text)
	}
	broker := &typedActionLedgerCaptureBroker{}
	deliverOperationalIssueNotice(&state, runCfg, broker, slog.Default())
	if !state.TypedDelivered || state.FallbackRequired {
		t.Fatalf("delivery state = %#v, want typed delivery", state)
	}
	if len(broker.typedEvents) != 1 || broker.typedEvents[0].eventType != operationalIssueNoticeEvent {
		t.Fatalf("typed events = %#v, want one %q event", broker.typedEvents, operationalIssueNoticeEvent)
	}
	pending, err := planner.ListPendingOperationalIssueNotices(db, time.Now(), 2)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending notices after typed delivery = %#v, err %v, want none", pending, err)
	}

	background := prepareOperationalIssueNotice(RunConfig{PlannerDB: db, MessageSource: "mission", IsMission: true}, "hello", slog.Default())
	if len(background.Items) != 0 || background.Text != "" {
		t.Fatalf("background run prepared a user notice: %#v", background)
	}
}

func TestExecuteAgentLoopDeliversOperationalNoticeWhenLLMIgnoresIt(t *testing.T) {
	initialTokenCount := globalTokenCount.Load()
	initialTokenEstimated := globalTokenEstimated.Load()
	defer func() {
		globalTokenCount.Store(initialTokenCount)
		globalTokenEstimated.Store(initialTokenEstimated)
	}()
	runCfg, client, cleanup := newPromptPipelineTestRunConfig(t, "notice-uncooperative-llm", "web_chat")
	defer cleanup()
	db := newPlannerTestDB(t)
	defer db.Close()
	runCfg.PlannerDB = db
	client.response = "Die aktuelle Nutzerfrage ist beantwortet."

	if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{
		Source: "maintenance", Context: "nightly", Title: "Background probe failed",
		Detail: "service unavailable", Severity: "error", Reference: "probe",
	}); err != nil {
		t.Fatalf("planner.RecordOperationalIssue() error = %v", err)
	}
	broker := &typedActionLedgerCaptureBroker{}
	resp, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{
		Model: runCfg.Config.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{
			Role: openai.ChatMessageRoleUser, Content: "Hallo",
		}},
	}, runCfg, false, broker)
	if err != nil {
		t.Fatalf("ExecuteAgentLoop() error = %v", err)
	}
	if client.calls != 1 || len(resp.Choices) == 0 || !strings.Contains(resp.Choices[0].Message.Content, "Nutzerfrage") {
		t.Fatalf("LLM response/calls = %#v/%d", resp, client.calls)
	}
	foundNotice := false
	for _, event := range broker.typedEvents {
		if event.eventType == operationalIssueNoticeEvent {
			foundNotice = true
		}
	}
	if !foundNotice {
		t.Fatalf("typed events = %#v, want deterministic operational notice", broker.typedEvents)
	}
	pending, err := planner.ListPendingOperationalIssueNotices(db, time.Now(), 2)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending after deterministic delivery = %#v, err %v", pending, err)
	}
}

func TestExecuteAgentLoopCameraRetryUsesExactlyOneTrustedVisionRoute(t *testing.T) {
	initialTokenCount := globalTokenCount.Load()
	initialTokenEstimated := globalTokenEstimated.Load()
	defer func() {
		globalTokenCount.Store(initialTokenCount)
		globalTokenEstimated.Store(initialTokenEstimated)
	}()
	previousManager := tools.DefaultGo2RTCManager()
	previousAnalyze := dispatchAnalyzeImageWithPrompt
	defer func() {
		tools.SetDefaultGo2RTCManager(previousManager)
		dispatchAnalyzeImageWithPrompt = previousAnalyze
	}()

	const password = "internal-password"
	jpeg := []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0xff, 0xd9}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, gotPassword, ok := r.BasicAuth()
		if !ok || gotPassword != password {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/go2rtc/proxy/api":
			_, _ = w.Write([]byte(`{"version":"1.9.14"}`))
		case "/api/go2rtc/proxy/api/frame.jpeg":
			if r.URL.Query().Get("src") != "aurago_driveway" {
				http.Error(w, "unexpected stream", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(jpeg)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	parsedURL, _ := url.Parse(upstream.URL)
	apiPort, _ := strconv.Atoi(parsedURL.Port())

	runCfg, client, cleanup := newPromptPipelineTestRunConfig(t, "camera-retry", "web_chat")
	defer cleanup()
	runCfg.Config.Directories.DataDir = t.TempDir()
	runCfg.Config.Go2RTC = config.Go2RTCConfig{
		Enabled: true, AgentAccess: true, URL: upstream.URL, APIHostPort: apiPort, APIPassword: password,
		Streams: []config.Go2RTCStreamConfig{{ID: "driveway", Name: "Driveway", Enabled: true, Source: "rtsp://camera.local/live"}},
	}
	mediaDB, err := tools.InitMediaRegistryDB(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatalf("InitMediaRegistryDB: %v", err)
	}
	defer mediaDB.Close()
	manager := tools.NewGo2RTCManager(runCfg.Config, nil, mediaDB, runCfg.Logger)
	if _, err := manager.Test(context.Background()); err != nil {
		t.Fatalf("prime go2rtc manager: %v", err)
	}
	tools.SetDefaultGo2RTCManager(manager)

	visionCalls := 0
	dispatchAnalyzeImageWithPrompt = func(filePath, prompt string, _ *config.Config) (string, int, int, error) {
		visionCalls++
		if !strings.Contains(prompt, "AURAGO_STRUCTURED_OBJECT_COUNT_V1") || !strings.Contains(prompt, "Wie viele PKW") {
			t.Fatalf("vision prompt was not structured or did not reuse the prior question: %q", prompt)
		}
		return `{"confirmed_count":2,"possible_additional_count":1,"other_vehicles":["van"],"items":[{"index":1,"type":"car","confidence":0.98,"confirmed":true},{"index":2,"type":"car","confidence":0.93,"confirmed":true},{"index":3,"type":"car","confidence":0.42,"confirmed":false}],"uncertainty":"one partially hidden candidate"}`, 12, 8, nil
	}
	client.response = "Bestätigt sind 2 PKW; ein weiterer Kandidat bleibt unsicher."
	broker := &captureBroker{}
	resp, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{
		Model: runCfg.Config.LLM.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleAssistant,
				ToolCalls: []openai.ToolCall{{ID: "prior-analysis", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{
					Name: "go2rtc", Arguments: `{"operation":"analyze_snapshot","stream_id":"driveway","prompt":"Wie viele PKW sind sichtbar?"}`,
				}}},
			},
			{Role: openai.ChatMessageRoleTool, ToolCallID: "prior-analysis", Content: `{"status":"ok","artifact":{"media_type":"image","stream_id":"driveway","registered_path":"/files/prior.jpg","source_tool":"go2rtc"}}`},
			{Role: openai.ChatMessageRoleUser, Content: "Versuche es erneut"},
		},
	}, runCfg, false, broker)
	if err != nil {
		t.Fatalf("ExecuteAgentLoop() error = %v", err)
	}
	if client.calls != 1 || visionCalls != 1 {
		t.Fatalf("LLM/vision calls = %d/%d, want 1/1", client.calls, visionCalls)
	}
	toolStarts := 0
	for _, event := range broker.events {
		if event.event != "tool_start" {
			continue
		}
		toolStarts++
		if event.message != "go2rtc" {
			t.Fatalf("unexpected retry tool %q", event.message)
		}
	}
	if toolStarts != 1 {
		t.Fatalf("tool starts = %d, want exactly one; events=%#v", toolStarts, broker.events)
	}
	if len(resp.Choices) == 0 || !strings.Contains(resp.Choices[0].Message.Content, "2 PKW") || !strings.Contains(resp.Choices[0].Message.Content, "unsicher") {
		t.Fatalf("inconsistent camera answer: %#v", resp)
	}
	for _, message := range client.lastReq.Messages {
		if message.Role != openai.ChatMessageRoleTool {
			continue
		}
		lower := strings.ToLower(message.Content)
		for _, forbidden := range []string{"execute_shell", "execute_python", "credential", "filesystem"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("tool result contained forbidden recovery route %q: %s", forbidden, message.Content)
			}
		}
	}
}

func TestOperationalIssueNoticeFallbackMarksOnlyAfterPersistence(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{
		Source:    "maintenance",
		Context:   "maintenance",
		Title:     "Maintenance agent loop failed",
		Detail:    "budget exceeded",
		Severity:  "error",
		Reference: "daily_maintenance",
	}); err != nil {
		t.Fatalf("planner.RecordOperationalIssue() error = %v", err)
	}
	runCfg := RunConfig{PlannerDB: db, MessageSource: "web_chat"}
	state := prepareOperationalIssueNotice(runCfg, "Hallo", slog.Default())
	deliverOperationalIssueNotice(&state, runCfg, NoopBroker{}, slog.Default())
	if !state.FallbackRequired || state.TypedDelivered {
		t.Fatalf("delivery state = %#v, want pending final-prefix fallback", state)
	}
	pending, err := planner.ListPendingOperationalIssueNotices(db, time.Now(), 2)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending notices before fallback persistence = %#v, err %v, want one", pending, err)
	}
	final := prependOperationalIssueNotice(state, "Die Aufgabe ist abgeschlossen.")
	if !strings.HasPrefix(final, state.Text) || !strings.Contains(final, "Die Aufgabe ist abgeschlossen.") {
		t.Fatalf("prefixed final answer = %q", final)
	}
	markPersistedOperationalIssueNotice(state, runCfg, slog.Default())
	pending, err = planner.ListPendingOperationalIssueNotices(db, time.Now(), 2)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending notices after fallback persistence = %#v, err %v, want none", pending, err)
	}
}

func TestRecordToolFailureOperationalIssueOnlyForBackgroundRuns(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	recordToolFailureOperationalIssue(
		RunConfig{PlannerDB: db, MessageSource: "web_chat", SessionID: "default"},
		ToolCall{Action: "docker"},
		`Tool Output: {"status":"error","message":"container failed"}`,
		slog.Default(),
	)
	issues, err := planner.ListOperationalIssueTodos(db, 10)
	if err != nil {
		t.Fatalf("ListOperationalIssueTodos() error = %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("direct web chat created operational issues: %#v", issues)
	}

	recordToolFailureOperationalIssue(
		RunConfig{PlannerDB: db, IsMission: true, MissionID: "mission-42", MessageSource: "mission"},
		ToolCall{Action: "docker"},
		`Tool Output: {"status":"error","message":"container failed"}`,
		slog.Default(),
	)
	issues, err = planner.ListOperationalIssueTodos(db, 10)
	if err != nil {
		t.Fatalf("ListOperationalIssueTodos() second error = %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	if !strings.Contains(issues[0].Title, "docker") || !strings.Contains(issues[0].Description, "container failed") {
		t.Fatalf("recorded issue = %#v, want tool and detail", issues[0])
	}
}
