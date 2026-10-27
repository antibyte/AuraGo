package discord

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// discordRESTRecorder is a Discord REST fixture: a real discordgo session whose
// HTTP client sends everything to a local server that records the message
// bodies it receives.
type discordRESTRecorder struct {
	mu      sync.Mutex
	bodies  []map[string]any
	session *discordgo.Session
}

type redirectTransport struct{ target *url.URL }

func (r redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = r.target.Scheme
	clone.URL.Host = r.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}

func newDiscordRESTRecorder(t *testing.T) *discordRESTRecorder {
	t.Helper()
	recorder := &discordRESTRecorder{}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.HasSuffix(r.URL.Path, "/messages") {
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("decode the Discord request %s: %v", r.URL.Path, err)
			}
			recorder.mu.Lock()
			recorder.bodies = append(recorder.bodies, body)
			recorder.mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"1","channel_id":"chan","content":"ok"}`)
	}))
	t.Cleanup(remote.Close)
	target, err := url.Parse(remote.URL)
	if err != nil {
		t.Fatalf("parse the fixture URL: %v", err)
	}
	session, err := discordgo.New("Bot fixture-token")
	if err != nil {
		t.Fatalf("construct the Discord session: %v", err)
	}
	session.Client = &http.Client{Transport: redirectTransport{target: target}}
	recorder.session = session
	return recorder
}

func (r *discordRESTRecorder) messages() []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]map[string]any(nil), r.bodies...)
}

func requireSuppressedEmbeds(t *testing.T, bodies []map[string]any, want int) {
	t.Helper()
	if len(bodies) != want {
		t.Fatalf("message requests = %d, want %d", len(bodies), want)
	}
	for i, body := range bodies {
		flags, _ := body["flags"].(float64)
		if int(flags)&int(discordgo.MessageFlagsSuppressEmbeds) == 0 {
			t.Errorf("message %d: flags = %v, want SUPPRESS_EMBEDS (%d) set", i, body["flags"], discordgo.MessageFlagsSuppressEmbeds)
		}
	}
}

// Discord fetches the URL of an embed itself, so an agent text that ends in an
// attacker's URL with conversation data in its query would leak without any tool
// call. The answer path (SendMessage, also behind the send_discord tool and the
// command results) has to suppress embeds on every chunk.
func TestDiscordSendMessageSuppressesEmbeds(t *testing.T) {
	recorder := newDiscordRESTRecorder(t)
	setSession(recorder.session)
	t.Cleanup(func() { setSession(nil) })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	const link = "https://attacker.example/r?d=summary-of-the-mails"

	if err := SendMessage("chan", "Reference: "+link, logger); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if err := SendMessage("chan", strings.Repeat("a ", 1500)+link, logger); err != nil {
		t.Fatalf("SendMessage (chunked): %v", err)
	}
	bodies := recorder.messages()
	requireSuppressedEmbeds(t, bodies, 3)
	if content, _ := bodies[0]["content"].(string); !strings.Contains(content, link) {
		t.Errorf("the link was dropped from the text instead of its embed being suppressed: %q", content)
	}
}

// The broker sends status events, questions and budget notices while the agent
// runs; their text can carry a URL from a tool call.
func TestDiscordBrokerDeliverySuppressesEmbeds(t *testing.T) {
	recorder := newDiscordRESTRecorder(t)
	broker := &DiscordBroker{session: recorder.session, channelID: "chan", logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	broker.Send("tool_start", "fetch https://attacker.example/r?d=1")
	broker.Send("question_user", "Open https://attacker.example/r?d=2 ?")
	broker.Send("budget_warning", "see https://attacker.example/r?d=3")

	requireSuppressedEmbeds(t, recorder.messages(), 3)
}

var discordPlainSend = regexp.MustCompile(`\.ChannelMessageSend\(`)

// A text sender that calls the plain ChannelMessageSend embeds its links. Only
// the fixed notices (busy reply, error and block notices) may use it.
func TestDiscordPlainChannelMessageSendIsLimitedToFixedNotices(t *testing.T) {
	allowed := map[string]bool{"runDiscordMessage": true, "sendDiscordNotice": true}
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("list the discord package files: %v", err)
	}
	header := regexp.MustCompile(`^func (?:\([^)]*\)\s*)?([A-Za-z0-9_]+)\(`)
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		current := ""
		for _, line := range strings.Split(string(source), "\n") {
			if match := header.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
				current = match[1]
			}
			if discordPlainSend.MatchString(line) && !allowed[current] {
				t.Errorf("%s: %s sends with the plain ChannelMessageSend, which embeds links", file, current)
			}
		}
	}
}
