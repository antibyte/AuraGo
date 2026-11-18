package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/memory"
	"aurago/internal/tools"
	"aurago/ui"

	openai "github.com/sashabaranov/go-openai"
)

func newOperatorCommandTestServer(t *testing.T) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	promptsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(promptsDir, "personalities"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "personalities", "operatorcheck.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Server.UILanguage = "en"
	cfg.Directories.PromptsDir = promptsDir
	return &Server{
		Cfg: cfg, Logger: logger, ShortTermMem: stm, HistoryManager: memory.NewEphemeralHistoryManager(),
		internalToken: "operator-test-token",
	}
}

func postChatCommand(t *testing.T, s *Server, content string, headers map[string]string) string {
	t.Helper()
	rec := recordChatCommand(t, s, content, headers)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST %q: status=%d body=%s", content, rec.Code, rec.Body.String())
	}
	var resp openai.ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || len(resp.Choices) != 1 {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	return resp.Choices[0].Message.Content
}

func recordChatCommand(t *testing.T, s *Server, content string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{
		Role: openai.ChatMessageRoleUser, Content: content,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:43210"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handleChatCompletions(s, nil).ServeHTTP(rec, req)
	return rec
}

// The web console and the desktop chat are admin surfaces, so operator slash
// commands keep running there (commands.Context.AllowOperator).
func TestAdminChatSurfacesRunOperatorCommands(t *testing.T) {
	s := newOperatorCommandTestServer(t)

	answer, handled, err := handleDesktopSlashCommand(s, "/personality")
	if err != nil || !handled || !strings.Contains(answer, "operatorcheck") {
		t.Fatalf("desktop /personality: answer=%q handled=%v err=%v, want the personality list", answer, handled, err)
	}
	if got := postChatCommand(t, s, "/personality", nil); !strings.Contains(got, "operatorcheck") {
		t.Fatalf("web /personality = %q, want the personality list", got)
	}
}

// follow_up and wait_for_event deliver model-written prompts through the
// loopback with X-Internal-FollowUp. A mission ID alone is not an approval.
func TestFollowUpLoopbackTurnsRefuseOperatorCommands(t *testing.T) {
	i18n.Load(ui.Content, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s := newOperatorCommandTestServer(t)
	followUp := map[string]string{"X-Internal-FollowUp": "true", "X-Internal-Token": s.internalToken}

	refusal := i18n.T("en", "backend.cmd_operator_private_only", "/personality")
	if got := postChatCommand(t, s, "/personality", followUp); got != refusal {
		t.Fatalf("follow-up /personality = %q, want refusal %q", got, refusal)
	}
	if got := postChatCommand(t, s, "/help", followUp); !strings.Contains(got, "/reset") {
		t.Fatalf("follow-up /help must still run, got %q", got)
	}

	mission := map[string]string{"X-Internal-FollowUp": "true", "X-Internal-Token": s.internalToken, "X-Mission-ID": "operator-check"}
	resp := recordChatCommand(t, s, "/personality", mission)
	if resp.Code != http.StatusForbidden || !strings.Contains(resp.Body.String(), "mission_operator_not_approved") {
		t.Fatalf("unapproved mission: status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestMissionOperatorApprovalRequiresOwnerAndBindsRun(t *testing.T) {
	allowMissionMutationsForTest(t)
	s := newOperatorCommandTestServer(t)
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.PasswordHash = "configured-for-test"
	s.Cfg.Auth.SessionSecret = "test-only-session-signing-key"
	const origin = "https://aurago.example"
	cookie := &http.Cookie{Name: sessionCookieName, Value: signSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour), "owner-fixture")}
	dir := t.TempDir()
	cron := tools.NewCronManager(dir)
	t.Cleanup(func() { _ = cron.Close() })
	s.MissionManagerV2 = tools.NewMissionManagerV2(dir, cron)
	command := "/personality"
	save := func(confirmed, owner bool, requestOrigin string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(missionWriteRequest{
			MissionV2: tools.MissionV2{
				Name: "Check personalities", Prompt: command, ExecutionType: tools.ExecutionScheduled, Schedule: "0 0 29 2 *",
			},
			ConfirmOperatorCommand: confirmed,
		})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, origin+"/api/missions/v2", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", requestOrigin)
		if owner {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		handleCreateMissionV2(s).ServeHTTP(rec, req)
		return rec
	}
	if rec := save(false, true, origin); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "mission_operator_confirmation_required") {
		t.Fatalf("save without confirmation: %d %s", rec.Code, rec.Body.String())
	}
	if rec := save(true, false, origin); rec.Code != http.StatusForbidden {
		t.Fatalf("confirmation without owner session: %d", rec.Code)
	}
	if rec := save(true, true, "https://other.example"); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin confirmation: %d", rec.Code)
	}
	if len(s.MissionManagerV2.List()) != 0 {
		t.Fatal("an unconfirmed mission was saved")
	}
	rec := save(true, true, origin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("confirmed save: %d %s", rec.Code, rec.Body.String())
	}
	var created struct{ Mission tools.MissionV2 }
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.Mission.ID == "" {
		t.Fatalf("read saved mission: %v", err)
	}
	id := created.Mission.ID
	approval := created.Mission.OperatorCommandApproval
	edited := created.Mission
	edited.Prompt += " other"
	if approval == "" || approval == missionOperatorSignature(s.Cfg.Auth.SessionSecret, &edited) {
		t.Fatal("approval must bind the exact operator command")
	}
	rescheduled := created.Mission
	rescheduled.Schedule = "0 1 29 2 *"
	if approval == missionOperatorSignature(s.Cfg.Auth.SessionSecret, &rescheduled) {
		t.Fatal("approval must bind the schedule")
	}

	s.MissionManagerV2.Stop()
	s.MissionManagerV2 = tools.NewMissionManagerV2(dir, cron)
	t.Cleanup(s.MissionManagerV2.Stop)
	type runCheck struct {
		allowed, activeOwner, exactCommand, signature, ownerProof bool
		trigger                                                   string
	}
	results := make(chan runCheck, 2)
	s.MissionManagerV2.SetCallback(func(prompt, runID string) {
		mission, _ := s.MissionManagerV2.Get(runID)
		owner, owned := s.MissionManagerV2.ActiveOwnerContext(runID)
		check := runCheck{
			allowed:      s.missionRunAllowsOperator(runID, prompt) && !s.missionRunAllowsOperator(runID, prompt+" other"),
			activeOwner:  owned,
			exactCommand: mission.Prompt == prompt,
			signature:    mission.OperatorCommandApproval == missionOperatorSignature(s.Cfg.Auth.SessionSecret, mission),
			ownerProof:   owner != nil && owner.Value(missionOperatorRunKey{}) == mission.OperatorCommandApproval,
			trigger:      s.MissionManagerV2.ActiveMissionTrigger(runID),
		}
		s.MissionManagerV2.OnMissionComplete(runID, tools.MissionResultSuccess, "checked admission only")
		results <- check
	})
	if err := s.MissionManagerV2.Start(); err != nil {
		t.Fatal(err)
	}
	wait := func(want bool) {
		t.Helper()
		select {
		case got := <-results:
			if got.allowed != want {
				t.Fatalf("operator run %+v, want allowed=%t", got, want)
			}
		case <-time.After(8 * time.Second):
			t.Fatal("mission queue did not dispatch")
		}
	}
	if err := s.MissionManagerV2.RunNow(id); err != nil {
		t.Fatal(err)
	}
	wait(false)
	run := httptest.NewRequest(http.MethodPost, origin+"/api/missions/v2/"+id+"/run", nil)
	run.Header.Set("Origin", origin)
	run.AddCookie(cookie)
	runResponse := httptest.NewRecorder()
	handleMissionRunV2(s, runResponse, run, id)
	if runResponse.Code != http.StatusOK {
		t.Fatalf("owner run: %d %s", runResponse.Code, runResponse.Body.String())
	}
	wait(true)
	if err := s.MissionManagerV2.TriggerMission(id, "cron", ""); err != nil {
		t.Fatal(err)
	}
	wait(true)
}
