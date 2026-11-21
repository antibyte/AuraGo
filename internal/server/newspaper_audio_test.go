package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/agentmail"
	"aurago/internal/testutil"
	openai "github.com/sashabaranov/go-openai"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/newspaper"
	"aurago/internal/phonon"
	"aurago/internal/voice"
)

func TestNewspaperBroadcastCoversEveryStoryInPersona(t *testing.T) {
	for _, size := range []int{1, 31} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			e := newspaper.Edition{LocalDate: "2026-09-25", Language: "de", Partial: true, Corrections: []string{"Korrektur zum Vortag."}}
			for i := 0; i < size; i++ {
				e.Stories = append(e.Stories, newspaper.Story{ID: fmt.Sprintf("s%02d", i), Headline: "Ein Ereignis", SingleSource: true, Paragraphs: []newspaper.Paragraph{{Text: "Ein bestätigtes Detail."}}})
			}
			calls := 0
			script, err := newspaperBroadcastScript(context.Background(), e, "dry humour", func(ctx context.Context, guide, input string) (string, error) {
				calls++
				if !strings.HasPrefix(input, "<external_data>") || !strings.Contains(guide, "uncertainty") {
					t.Fatal("editor boundaries missing")
				}
				var p struct {
					Language             string
					Persona              string `json:"persona_style"`
					First, Last, Partial bool
					Stories              []struct {
						ID, Body string
						Single   bool `json:"single_source"`
					}
				}
				if err := json.Unmarshal([]byte(newspaperUnwrap(input)), &p); err != nil {
					t.Fatal(err)
				}
				if !p.Partial || p.Persona != "dry humour" || p.Language != "de" || len(p.Stories) > 6 || !p.Stories[0].Single || p.Stories[0].Body == "" {
					t.Fatalf("payload: %+v", p)
				}
				if calls == 1 {
					return "", llm.ErrJSONCompletionTruncated
				}
				if strings.Contains(guide, "one supplied story") {
					if len(p.Stories) != 1 {
						t.Fatal("repair requested more than one story")
					}
					b, _ := json.Marshal(map[string]string{"opening": "Ansage.", "closing": "Abspann.", "text": "Beitrag " + p.Stories[0].ID + "."})
					return string(b), nil
				}
				var items []map[string]string
				for _, story := range p.Stories {
					items = append(items, map[string]string{"id": story.ID, "text": "Beitrag " + story.ID + "."})
				}
				b, _ := json.Marshal(map[string]any{"opening": "Ansage.", "closing": "Abspann.", "items": items})
				return string(b), nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if calls != (size+5)/6+min(size, 6) || strings.Count(script, "Ansage.") != 1 || strings.Count(script, "Abspann.") != 1 || !strings.Contains(script, "Korrektur zum Vortag.") {
				t.Fatalf("script: %s", script)
			}
			for _, story := range e.Stories {
				if strings.Count(script, "Beitrag "+story.ID+".") != 1 {
					t.Fatal("missing or repeated " + story.ID)
				}
			}
		})
	}
	e := newspaper.Edition{Stories: []newspaper.Story{{ID: "a"}, {ID: "b"}}}
	calls := 0
	_, err := newspaperBroadcastScript(context.Background(), e, "", func(context.Context, string, string) (string, error) {
		calls++
		return `{"items":[{"id":"a","text":"A"},{"id":"a","text":"B"}]}`, nil
	})
	if err == nil || calls != 2 {
		t.Fatal("duplicate story IDs passed coverage validation")
	}
}

func TestNewspaperBroadcastRepairsOnlyUnusableStories(t *testing.T) {
	e := newspaper.Edition{Stories: []newspaper.Story{{ID: "story-1"}, {ID: "story-2"}, {ID: "story-3"}, {ID: "story-4"}, {ID: "story-5"}}}
	initial := `{"opening":"Hallo.","items":[{"id":"story-5","text":"Fünf."},{"id":"unpublished","text":"Fremd."},{"id":"story-3","text":"A."},{"id":"story-1","text":"Eins."},{"id":"story-3","text":"B."},{"id":"story-4","text":"` + strings.Repeat("ä", 501) + `"}],"closing":"Bis morgen."}`
	calls := 0
	script, err := newspaperBroadcastScript(context.Background(), e, "", func(_ context.Context, _ string, input string) (string, error) {
		calls++
		if calls == 1 {
			return initial, nil
		}
		var p struct {
			Stories     []struct{ ID string }
			First, Last bool
		}
		if err := json.Unmarshal([]byte(newspaperUnwrap(input)), &p); err != nil || len(p.Stories) != 1 || p.Stories[0].ID != fmt.Sprintf("story-%d", calls) || p.First || p.Last {
			t.Fatalf("unexpected repair request %d: %+v", calls, p)
		}
		return fmt.Sprintf(`{"text":"Beitrag %d."}`, calls), nil
	})
	if err != nil || calls != 4 || script != "Hallo.\nEins.\nBeitrag 2.\nBeitrag 3.\nBeitrag 4.\nFünf.\nBis morgen." {
		t.Fatalf("calls=%d script=%q err=%v", calls, script, err)
	}
}

func TestNewspaperBroadcastStopsOnProviderError(t *testing.T) {
	e := newspaper.Edition{Stories: []newspaper.Story{{ID: "story-1", Deck: "Published summary."}}}
	failure := errors.New("provider unavailable")
	for _, failAt := range []int{1, 2} {
		calls := 0
		script, err := newspaperBroadcastScript(context.Background(), e, "", func(context.Context, string, string) (string, error) {
			calls++
			if calls == failAt {
				return "", failure
			}
			return `{"items":[]}`, nil
		})
		if !errors.Is(err, failure) || calls != failAt || script != "" {
			t.Fatalf("calls=%d script=%q err=%v", calls, script, err)
		}
	}
}

func TestNewspaperBroadcastSurvivesRejectedSingleStoryRepairs(t *testing.T) {
	e := newspaper.Edition{
		Language: "de-DE", Partial: true, Corrections: []string{"Korrektur zum Vortag."},
		Stories: []newspaper.Story{
			{ID: "a", Deck: "A aus der Zeitung."},
			{ID: "b", Deck: "B aus der Zeitung."},
			{ID: "c", Deck: "C aus der Zeitung.", SingleSource: true},
			{ID: "d", Headline: "D aus dem Archiv."},
		},
	}
	calls := 0
	script, err := newspaperBroadcastScript(context.Background(), e, "dry humour", func(_ context.Context, _ string, input string) (string, error) {
		calls++
		var p struct{ Stories []struct{ ID string } }
		if err := json.Unmarshal([]byte(newspaperUnwrap(input)), &p); err != nil {
			t.Fatal(err)
		}
		switch p.Stories[0].ID {
		case "a":
			if len(p.Stories) == 1 {
				t.Fatal("a missing greeting caused a completed story to be regenerated")
			}
			return `{"items":[{"id":"a","text":"A aus dem Studio."}],"closing":"Bis morgen."}`, nil
		case "b":
			return `{"items":[{"id":"b","text":"B aus dem Studio."}]}`, nil
		case "c":
			return "", llm.ErrJSONCompletionTruncated
		default:
			return `{"items":[{"id":"unknown","text":"Fremd."}],"text":"` + strings.Repeat("ü", 501) + `"}`, nil
		}
	})
	if err != nil || calls != 4 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	for _, s := range []string{"Teilausgabe", "Korrektur zum Vortag.", "A aus dem Studio.", "B aus dem Studio.", "C aus der Zeitung.", "Quelle", "D aus dem Archiv.", "Bis morgen."} {
		if strings.Count(script, s) != 1 {
			t.Fatalf("expected %q once in %q", s, script)
		}
	}
	if strings.Contains(script, "Fremd.") || strings.Contains(script, "B aus der Zeitung.") {
		t.Fatalf("unmatched or duplicate material in %q", script)
	}
}

func TestNewspaperAudioMailAndOwnerAttachments(t *testing.T) {
	file := newspaper.AudioFile{MIME: "audio/mpeg", Data: bytes.Repeat([]byte("audio"), 34)}
	var body bytes.Buffer
	body.WriteString("both mail alternatives")
	kind, err := newspaperMixedMail(&body, "multipart/alternative; boundary=original", newspaperMailAudio{"news.mp3", file.MIME, file.Data})
	if err != nil {
		t.Fatal(err)
	}
	typ, params, err := mime.ParseMediaType(kind)
	if err != nil || typ != "multipart/mixed" {
		t.Fatal(kind, err)
	}
	mr := multipart.NewReader(&body, params["boundary"])
	p, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	original, _ := io.ReadAll(p)
	if !strings.Contains(p.Header.Get("Content-Type"), "boundary=original") || string(original) != "both mail alternatives" {
		t.Fatal("mail body lost")
	}
	p, err = mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, p))
	if err != nil || !bytes.Equal(decoded, file.Data) || p.FileName() != "news.mp3" || p.Header.Get("Content-Type") != file.MIME {
		t.Fatal("invalid attachment", err)
	}
	if _, err = mr.NextPart(); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}

	for _, both := range []bool{false, true} {
		t.Run(fmt.Sprint(both), func(t *testing.T) {
			fake := &fakeWhatsAppOwner{ready: true}
			useFakeWhatsAppOwner(t, fake)
			cfg := newspaperWhatsAppConfig()
			cfg.VirtualDesktop.Enabled = true
			s := &Server{Cfg: cfg}
			e := newspaperWhatsAppEdition("A visible headline")
			_, err := s.newspaperDeliverAudio(context.Background(), e, newspaper.Profile{}, "whatsapp", both, file)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if both {
				want = 2
			}
			if len(fake.attachments) != want || !strings.HasPrefix(fake.attachments[want-1], file.Name(e)+"|") || len(fake.messages) != 0 {
				t.Fatalf("attachments %v", fake.attachments)
			}
		})
	}
}

func TestNewspaperAudioAgentMailUsesSelectedFormat(t *testing.T) {
	if testutil.RunTLSFixtureSubprocess(t) {
		return
	}
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.Newspaper.Enabled = true
	cfg.Newspaper.AllowEmail = true
	cfg.AgentMail.Enabled = true
	cfg.AgentMail.InboxID = "test-inbox"
	cfg.AgentMail.APIKey = "test-key"
	var sent []agentmail.SendMessageRequest
	provider := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req agentmail.SendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		sent = append(sent, req)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"message_id":"1"}`)
	}))
	defer provider.Close()
	testutil.TrustHTTPSTestServer(t, provider)
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	cfg.AgentMail.BaseURL = provider.URL
	s := &Server{Cfg: cfg}
	e := newspaperWhatsAppEdition("A visible headline")
	p := newspaper.Profile{EmailVerified: true, EmailAccountID: "agentmail", EmailTo: "reader@example.org"}
	file := newspaper.AudioFile{MIME: "audio/mpeg", Data: []byte("same revision")}
	for _, both := range []bool{false, true} {
		if _, err := s.newspaperDeliverAudio(context.Background(), e, p, "email", both, file); err != nil {
			t.Fatal(err)
		}
		req := sent[len(sent)-1]
		if len(req.Attachments) != 1 || req.Attachments[0].ContentBase64 != base64.StdEncoding.EncodeToString(file.Data) || req.Attachments[0].Filename != file.Name(e) || strings.Contains(req.Text, e.Stories[0].Headline) != both || strings.Contains(req.HTML, e.Stories[0].Headline) != both {
			t.Fatalf("format both=%v, wrong attachment or edition", both)
		}
	}
}

func TestNewspaperTelegramAudioFormats(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.Newspaper.Enabled = true
	cfg.Newspaper.AllowTelegram = true
	cfg.Telegram.UserID = 123
	cfg.Telegram.BotToken = "test-audio"
	e := newspaperWhatsAppEdition("A visible headline")
	for _, test := range []struct{ mime, field, method string }{{"audio/mpeg", "audio", "sendAudio"}, {"audio/wav", "document", "sendDocument"}} {
		t.Run(test.method, func(t *testing.T) {
			file := newspaper.AudioFile{MIME: test.mime, Data: []byte("recording")}
			count := 0
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/getMe") {
					fmt.Fprint(w, `{"ok":true,"result":{"id":1,"is_bot":true}}`)
					return
				}
				if !strings.HasSuffix(r.URL.Path, "/"+test.method) {
					t.Errorf("unexpected send %s", r.URL.Path)
					return
				}
				part, header, err := r.FormFile(test.field)
				if err != nil {
					t.Error(err)
					return
				}
				defer part.Close()
				data, _ := io.ReadAll(part)
				if !bytes.Equal(data, file.Data) || header.Filename != file.Name(e) {
					t.Error("incorrect recording")
				}
				count++
				fmt.Fprint(w, `{"ok":true,"result":{"message_id":7}}`)
			}))
			defer fixture.Close()
			old := newspaperTelegramEndpoint
			newspaperTelegramEndpoint = fixture.URL + "/bot%s/%s"
			defer func() { newspaperTelegramEndpoint = old }()
			_, err := (&Server{Cfg: cfg}).newspaperDeliverAudio(context.Background(), e, newspaper.Profile{}, "telegram", false, file)
			if err != nil || count != 1 {
				t.Fatalf("requests %d, %v", count, err)
			}
		})
	}
}

func TestNewspaperGoogleTTSChunksPreserveUnicodeText(t *testing.T) {
	cfg := &config.Config{}
	cfg.TTS.Provider = "google"
	var chunks []string
	synth := &sipSpeechSynthesizer{cfg: cfg, synthesizeForTest: func(_ context.Context, text, lang string) ([]int16, int, error) {
		chunks = append(chunks, text)
		return []int16{1}, 16000, nil
	}}
	input := strings.Repeat("ü界", 285)
	if _, _, err := synth.Synthesize(context.Background(), input, "de"); err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 3 || strings.Join(chunks, "") != input {
		t.Fatal("Google omitted part of a long sentence")
	}
	for _, chunk := range chunks {
		if len([]rune(chunk)) > 200 {
			t.Fatal("provider would truncate a chunk")
		}
	}
}

func TestNewspaperAudioStreamPermissionsAndSeeking(t *testing.T) {
	s, reader, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.Newspaper.Enabled = true
	ctx := context.Background()
	file := []byte("a complete recorded edition")
	var attempted atomic.Bool
	var err error
	s.Newspaper, err = newspaper.New(newspaper.Options{Path: filepath.Join(t.TempDir(), "news.db"), Policy: func() newspaper.Policy {
		return newspaper.Policy{Enabled: true, ReadOnly: s.ConfigSnapshot().Newspaper.ReadOnly}
	}, Research: func(context.Context, newspaper.Profile, time.Time, func(newspaper.Progress)) (newspaper.Draft, error) {
		return newspaper.Draft{}, nil
	}, Narrate: func(context.Context, newspaper.Edition) (newspaper.AudioFile, error) {
		if !attempted.Swap(true) {
			return newspaper.AudioFile{}, errors.New("script_model_failed")
		}
		return newspaper.AudioFile{MIME: "audio/mpeg", Data: file}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Newspaper.Close()
	admin, _, err := s.TokenManager.Create("newspaper audio", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	run, err := s.Newspaper.Store().Start(ctx, "2026-09-25", false, now)
	if err != nil {
		t.Fatal(err)
	}
	e := newspaperWhatsAppEdition("Recorded headline")
	e.CreatedAt = now
	e.ID = run.ID
	e.LocalDate = run.LocalDate
	if err = e.Seal(); err != nil {
		t.Fatal(err)
	}
	if err = s.Newspaper.Store().Publish(ctx, run, e); err != nil {
		t.Fatal(err)
	}
	call := func(method, op, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/desktop/newspaper/editions/"+e.ID+"/"+op, strings.NewReader("{}"))
		r.Header.Set("Range", "bytes=2-9")
		r.Header.Set("Origin", "http://example.com")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.handleNewspaper(w, r)
		return w
	}
	if w := call("GET", "audio/file", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := call("GET", "audio/file", reader); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := call("GET", "audio/file", admin); w.Code != 404 {
		t.Fatal(w.Code)
	}
	for _, want := range []string{"failed", "ready"} {
		if w := call("POST", "audio", admin); w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		for deadline := time.Now().Add(5 * time.Second); ; {
			a, err := s.Newspaper.Store().AudioState(ctx, e.ID)
			if err != nil {
				t.Fatal(err)
			}
			if a.Status == want {
				if want == "failed" && (a.Reason != "script_model_failed" || call("GET", "audio/file", admin).Code != 404) {
					t.Fatal("incorrect failed recording state", a)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal(a)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	cfg := s.ConfigSnapshot().Clone()
	cfg.Newspaper.ReadOnly = true
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(cfg)
	s.CfgMu.Unlock()
	if w := call("POST", "audio", admin); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := call("POST", "cast", admin); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := call("GET", "audio/file", admin); w.Code != http.StatusPartialContent || !bytes.Equal(w.Body.Bytes(), file[2:10]) || w.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("range %d %q", w.Code, w.Body.Bytes())
	}
	if w := call("HEAD", "audio/file", admin); w.Code != 206 || w.Body.Len() != 0 {
		t.Fatal(w.Code, w.Body.Len())
	}
}

func TestNewspaperRecordingUsesWAVWithoutEncoder(t *testing.T) {
	wav, err := voice.EncodeWAVPCM16(make([]int16, 8000), 8000)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	file, err := encodeNewspaperRecording(context.Background(), wav)
	if err != nil || file.MIME != "audio/wav" || !bytes.Equal(file.Data, wav) {
		t.Fatal(file.MIME, err)
	}
}

func TestNewspaperAudioPhononSmoke(t *testing.T) {
	root := os.Getenv("AURAGO_PHONON_SMOKE_DIR")
	if root == "" {
		t.Skip("set AURAGO_PHONON_SMOKE_DIR to the prepared local Phonon resources")
	}
	// The installed Chat TTS route resolves the bundle ID below AURAGO_PHONON_DIR.
	t.Setenv("AURAGO_PHONON_DIR", t.TempDir())
	if err := os.CopyFS(phonon.Root(), os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	if err := phonon.Ready(phonon.Root()); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.TTS.Provider = "phonon"
	cfg.TTS.Language = "de"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cfg.Newspaper.Enabled = true
	cfg.VirtualDesktop.Enabled = true
	cfg.LLM.Model = "test-editor"
	cfg.Personality.CorePersonality = "professional"
	if _, err := phonon.VoicePath(phonon.Root(), cfg.Directories.DataDir, phonon.DefaultVoice("de", cfg.Personality.CorePersonality)); err != nil {
		t.Fatal(err)
	}
	e := newspaper.Edition{Language: "de", LocalDate: "2026-09-25", Stories: []newspaper.Story{
		{ID: "library", Headline: "Stadtrat beschließt Bibliotheksbau", Deck: "Die Planung beginnt im Frühjahr."},
		{ID: "sports", Headline: "Stadtfinale am Sonntag"},
	}}
	const output = `{"opening":"Guten Morgen aus der Nachrichtenredaktion.","items":[{"id":"library","text":"Der Stadtrat hat den Bau einer neuen Bibliothek beschlossen. Die Planung beginnt im Frühjahr."},{"id":"sports","text":"Und noch eine Nachricht vom Sport: Das Stadtfinale findet am Sonntag statt."}],"closing":"Das war unsere kurze Beispielausgabe."}`
	client := &newspaperStoryTestClient{response: openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonStop, Message: openai.ChatCompletionMessage{Content: output}}}}}
	file, err := (&Server{Cfg: cfg, LLMClient: client, newspaperSkillReady: true}).narrateNewspaper(ctx, e)
	var payload struct {
		Style string `json:"persona_style"`
	}
	if len(client.request.Messages) != 2 || json.Unmarshal([]byte(newspaperUnwrap(client.request.Messages[1].Content)), &payload) != nil || strings.TrimSpace(payload.Style) == "" {
		t.Fatal("active persona was not passed to the editor")
	}
	if err != nil || len(file.Data) < 15000 {
		t.Fatal(file.MIME, len(file.Data), err)
	}
	if _, err := exec.LookPath("ffmpeg"); err == nil && file.MIME != "audio/mpeg" {
		t.Fatal(file.MIME)
	}
	if path := os.Getenv("AURAGO_NEWSPAPER_AUDIO_SAMPLE"); path != "" {
		if err := os.WriteFile(path, file.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
