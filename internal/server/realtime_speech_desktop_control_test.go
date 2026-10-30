package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/desktop"
	"aurago/internal/llm"
	"aurago/internal/realtimespeech"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func TestRealtimeSpeechDesktopOpensAppsAndExistingDocuments(t *testing.T) {
	for _, test := range []struct {
		name      string
		operation string
		appID     string
		path      string
	}{
		{"app", "open_app", "calculator", ""},
		{"document", "open_in_app", "writer", "Documents/Notizen.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newTestDesktopChatServer(t)
			root := t.TempDir()
			t.Cleanup(s.shutdownDesktopStorage)
			s.Cfg.LLM.UseNativeFunctions = true
			s.Cfg.Directories.PromptsDir = testPromptsDir(t)
			s.Cfg.Directories.SkillsDir = t.TempDir()
			s.Cfg.Directories.DataDir = root
			s.Cfg.Agent.SystemLanguage = "English"
			s.Cfg.Agent.ContextWindow = 65536
			s.Cfg.VirtualDesktop.Enabled = true
			s.Cfg.VirtualDesktop.AllowAgentControl = true
			s.Cfg.VirtualDesktop.WorkspaceDir = filepath.Join(root, "desktop")
			s.Cfg.VirtualDesktop.MaxFileSizeMB = 1
			s.Cfg.SQLite.VirtualDesktopPath = filepath.Join(root, "desktop.db")
			s.Cfg.Tools.VirtualDesktop.Enabled = true
			s.Registry = tools.NewProcessRegistry(s.Logger)
			original := []byte("# Besprechung\nVorhandener Inhalt mit Umlauten: Grüße.\n")
			documentPath := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(test.path))
			if test.path != "" {
				if err := os.MkdirAll(filepath.Dir(documentPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(documentPath, original, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var calls atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				if calls.Add(1) == 1 {
					writeToolCall(w, "desktop-launch", "virtual_desktop_apps", map[string]string{
						"operation": test.operation, "app_id": test.appID, "path": test.path,
					})
					return
				}
				writeText(w, "Geöffnet.")
			}))
			defer provider.Close()
			clientConfig := openai.DefaultConfig("fixture-key")
			clientConfig.BaseURL = provider.URL
			s.LLMClient = llm.WrapOpenAIClient(openai.NewClientWithConfig(clientConfig))

			registry := realtimespeech.NewRegistry(nil)
			session, _, err := registry.Acquire("browser", realtimespeech.Session{
				ProfileID: "voice", Provider: realtimespeech.ProviderOpenAI,
				Surface: "desktop", ChatSessionID: desktopChatSessionID,
			}, false)
			if err != nil {
				t.Fatal(err)
			}
			globalEvents := s.SSE.subscribe()
			defer s.SSE.unsubscribe(globalEvents)
			payload, _ := json.Marshal(map[string]string{
				"session_id": session.ID, "client_id": "browser", "request_id": "desktop-launch",
				"request": "Öffne " + test.appID + " " + test.path,
			})
			request := httptest.NewRequest(http.MethodPost, "/api/realtime-speech/actions", bytes.NewReader(payload))
			recorder := httptest.NewRecorder()
			handleRealtimeSpeechActions(s, registry, nil, handleDesktopChatStream(s)).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			var launch struct {
				Type    string        `json:"type"`
				Payload desktop.Event `json:"payload"`
			}
			for _, line := range strings.Split(recorder.Body.String(), "\n") {
				if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"type":"virtual_desktop_event"`) {
					if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &launch); err != nil {
						t.Fatal(err)
					}
					break
				}
			}
			if launch.Payload.ID == "" || launch.Payload.Type != "open_app" {
				t.Fatalf("missing identified launch in voice stream: %s", recorder.Body.String())
			}
			data := launch.Payload.Payload.(map[string]interface{})
			if data["app_id"] != test.appID || (test.path != "" && data["path"] != test.path) {
				t.Fatalf("launch payload=%v", data)
			}
			foundGlobal := false
			for len(globalEvents) > 0 {
				var event struct {
					Type    string        `json:"type"`
					Payload desktop.Event `json:"payload"`
				}
				if json.Unmarshal([]byte(<-globalEvents), &event) == nil && event.Type == "virtual_desktop_event" {
					foundGlobal = true
					if event.Payload.ID != launch.Payload.ID {
						t.Fatal("direct and global streams must retain the same event ID")
					}
				}
			}
			if !foundGlobal {
				t.Fatal("launch event missing from the existing Desktop event channel")
			}
			if test.path != "" {
				got, err := os.ReadFile(documentPath)
				if err != nil || !bytes.Equal(got, original) {
					t.Fatalf("opening changed the document: err=%v", err)
				}
			}
		})
	}
}

func TestRealtimeSpeechDesktopReadonlyBlocksAppLaunch(t *testing.T) {
	s := newTestDesktopChatServer(t)
	s.Cfg.VirtualDesktop.ReadOnly = true
	registry := realtimespeech.NewRegistry(nil)
	session, _, err := registry.Acquire("browser", realtimespeech.Session{
		Surface: "desktop", ChatSessionID: desktopChatSessionID,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{
		"session_id": session.ID, "client_id": "browser", "request_id": "readonly-launch",
		"request": "Öffne den Taschenrechner",
	})
	recorder := httptest.NewRecorder()
	handleRealtimeSpeechActions(s, registry, nil, handleDesktopChatStream(s)).ServeHTTP(recorder,
		httptest.NewRequest(http.MethodPost, "/api/realtime-speech/actions", bytes.NewReader(body)))
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "desktop_readonly") {
		t.Fatalf("readonly action status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
