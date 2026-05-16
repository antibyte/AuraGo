package agentmail

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

type contentEvaluator interface {
	EvaluateContent(ctx context.Context, contentType string, content string) security.GuardianResult
}

type NotifyFunc func(context.Context, string) error

type ServiceConfig struct {
	Config      Config
	Client      *Client
	Logger      *slog.Logger
	Guardian    *security.Guardian
	LLMGuardian contentEvaluator
	ScanEmails  bool
	Notify      NotifyFunc
}

type Service struct {
	cfg         Config
	client      *Client
	logger      *slog.Logger
	guardian    *security.Guardian
	llmGuardian contentEvaluator
	scanEmails  bool
	notify      NotifyFunc

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
	seen    map[string]struct{}
}

var (
	agentMailWebSocketPingInterval = 45 * time.Second
	agentMailWebSocketPongWait     = 2 * time.Minute
	agentMailWebSocketWriteWait    = 10 * time.Second
)

type WebSocketMessageEvent struct {
	Type    string  `json:"type,omitempty"`
	InboxID string  `json:"inbox_id,omitempty"`
	Message Message `json:"message,omitempty"`
}

func NewService(cfg ServiceConfig) *Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		cfg:         normalizeConfig(cfg.Config),
		client:      cfg.Client,
		logger:      logger,
		guardian:    cfg.Guardian,
		llmGuardian: cfg.LLMGuardian,
		scanEmails:  cfg.ScanEmails,
		notify:      cfg.Notify,
		seen:        make(map[string]struct{}),
	}
}

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}
	if err := s.validateConfig(); err != nil {
		return err
	}
	if s.client == nil {
		client, err := NewClient(ClientConfig{BaseURL: s.cfg.BaseURL, APIKey: s.cfg.APIKey})
		if err != nil {
			return err
		}
		s.client = client
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	go s.run(runCtx)
	return nil
}

func (s *Service) Stop(ctx context.Context) {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.running = false
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Service) validateConfig() error {
	if !s.cfg.Enabled || !s.cfg.RelayToAgent {
		return fmt.Errorf("agentmail relay is disabled")
	}
	if strings.TrimSpace(s.cfg.APIKey) == "" {
		return fmt.Errorf("agentmail api key is not configured")
	}
	if strings.TrimSpace(s.cfg.InboxID) == "" {
		return fmt.Errorf("agentmail inbox_id is required for relay")
	}
	if s.notify == nil {
		return fmt.Errorf("agentmail notify callback is required")
	}
	return nil
}

func (s *Service) run(ctx context.Context) {
	defer func() {
		s.mu.Lock()
		if s.cancel != nil {
			s.running = false
			s.cancel = nil
		}
		s.mu.Unlock()
	}()

	s.seed(ctx)
	if s.cfg.UseWebSocket {
		for ctx.Err() == nil {
			if err := s.runWebSocket(ctx); err != nil && ctx.Err() == nil {
				s.logWebSocketFallback(err)
				s.pollOnce(ctx)
				wait := minDuration(s.pollInterval(), 30*time.Second)
				if !sleepContext(ctx, wait) {
					return
				}
			}
		}
		return
	}
	s.runPolling(ctx)
}

func (s *Service) pollInterval() time.Duration {
	interval := time.Duration(s.cfg.PollIntervalSeconds) * time.Second
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}
	return interval
}

func (s *Service) runPolling(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollOnce(ctx)
		}
	}
}

func (s *Service) seed(ctx context.Context) {
	res, err := s.client.ListMessages(ctx, s.cfg.InboxID, ListMessagesOptions{Limit: 50, Labels: []string{"unread"}})
	if err != nil {
		s.logger.Warn("[AgentMail] Initial seed failed", "error", err)
		return
	}
	s.mu.Lock()
	for _, msg := range res.Messages {
		if msg.ID != "" {
			s.seen[msg.ID] = struct{}{}
		}
	}
	s.mu.Unlock()
	s.logger.Info("[AgentMail] Seeded inbox relay", "inbox_id", s.cfg.InboxID, "messages", len(res.Messages))
}

func (s *Service) pollOnce(ctx context.Context) {
	res, err := s.client.ListMessages(ctx, s.cfg.InboxID, ListMessagesOptions{Limit: 25, Labels: []string{"unread"}})
	if err != nil {
		s.logger.Warn("[AgentMail] Poll failed", "error", err)
		return
	}
	for _, msg := range res.Messages {
		if msg.ID == "" || s.isSeen(msg.ID) {
			continue
		}
		if err := s.handleMessage(ctx, msg); err != nil {
			s.logger.Warn("[AgentMail] Relay failed", "message_id", msg.ID, "error", err)
			continue
		}
		s.markSeen(msg.ID)
	}
}

func (s *Service) runWebSocket(ctx context.Context) error {
	wsURL, err := url.Parse(strings.TrimSpace(s.cfg.WebSocketURL))
	if err != nil || wsURL.Scheme == "" || wsURL.Host == "" {
		return fmt.Errorf("invalid websocket URL %q", s.cfg.WebSocketURL)
	}
	q := wsURL.Query()
	q.Set("inbox_id", s.cfg.InboxID)
	wsURL.RawQuery = q.Encode()

	header := http.Header{}
	header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL.String(), header)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(agentMailWebSocketPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(agentMailWebSocketPongWait))
	})

	if err := conn.WriteJSON(map[string]string{"type": "subscribe", "inbox_id": s.cfg.InboxID}); err != nil {
		return fmt.Errorf("subscribe agentmail websocket: %w", err)
	}
	stopPing := make(chan struct{})
	defer close(stopPing)
	go s.keepWebSocketAlive(ctx, conn, stopPing)

	for ctx.Err() == nil {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		event, ok := ParseWebSocketMessageEvent(raw)
		if !ok || event.Message.ID == "" || s.isSeen(event.Message.ID) {
			continue
		}
		if event.InboxID != "" && event.InboxID != s.cfg.InboxID {
			continue
		}
		if err := s.handleMessage(ctx, event.Message); err != nil {
			s.logger.Warn("[AgentMail] WebSocket relay failed", "message_id", event.Message.ID, "error", err)
			continue
		}
		s.markSeen(event.Message.ID)
	}
	return ctx.Err()
}

func (s *Service) keepWebSocketAlive(ctx context.Context, conn *websocket.Conn, stop <-chan struct{}) {
	interval := agentMailWebSocketPingInterval
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = conn.Close()
			return
		case <-stop:
			return
		case <-ticker.C:
			deadline := time.Now().Add(agentMailWebSocketWriteWait)
			if err := conn.WriteControl(websocket.PingMessage, []byte("agentmail"), deadline); err != nil {
				s.logger.Debug("[AgentMail] WebSocket keepalive failed", "error", err)
				_ = conn.Close()
				return
			}
		}
	}
}

func (s *Service) logWebSocketFallback(err error) {
	msg := "[AgentMail] WebSocket relay failed; falling back to poll cycle"
	if isTransientWebSocketClose(err) {
		s.logger.Info("[AgentMail] WebSocket relay disconnected; falling back to poll cycle", "error", err)
		return
	}
	s.logger.Warn(msg, "error", err)
}

func isTransientWebSocketClose(err error) bool {
	if err == nil {
		return false
	}
	if websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
		websocket.CloseAbnormalClosure,
	) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "unexpected EOF") ||
		strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "i/o timeout")
}

func (s *Service) handleMessage(ctx context.Context, msg Message) error {
	msg = s.sanitizeMessage(ctx, msg)
	if err := s.notify(ctx, BuildNotificationPrompt(s.cfg.InboxID, msg)); err != nil {
		return err
	}
	_, err := s.client.UpdateMessage(ctx, s.cfg.InboxID, msg.ID, UpdateMessageRequest{
		AddLabels:    []string{"processed", "read"},
		RemoveLabels: []string{"unread"},
	})
	return err
}

func (s *Service) sanitizeMessage(ctx context.Context, msg Message) Message {
	combined := fmt.Sprintf("From: %s <%s>\nSubject: %s\nText: %s", msg.From.Name, msg.From.Email, msg.Subject, msg.Text)
	if s.guardian == nil {
		return msg
	}
	scan := s.guardian.ScanForInjection(combined)
	if scan.Level >= security.ThreatHigh {
		s.logger.Warn("[AgentMail] Guardian blocked message", "message_id", msg.ID, "threat", scan.Level.String())
		msg.Subject = security.SanitizedText("guardian scan flagged this message")
		msg.Text = security.RedactedText("guardian blocked content after injection detection")
		msg.Snippet = security.RedactedText("")
		return msg
	}
	if s.llmGuardian != nil && s.scanEmails {
		llmResult := s.llmGuardian.EvaluateContent(ctx, "email", combined)
		if llmResult.Decision == security.DecisionBlock {
			s.logger.Warn("[AgentMail] LLM Guardian blocked message", "message_id", msg.ID, "reason", llmResult.Reason)
			msg.Subject = security.SanitizedText("llm guardian blocked this message")
			msg.Text = security.RedactedText("llm guardian blocked content: " + llmResult.Reason)
			msg.Snippet = security.RedactedText("")
			return msg
		}
	}
	msg.Text = s.guardian.SanitizeToolOutput("agentmail", msg.Text)
	msg.Snippet = s.guardian.SanitizeToolOutput("agentmail", msg.Snippet)
	return msg
}

func (s *Service) isSeen(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.seen[id]
	return ok
}

func (s *Service) markSeen(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen[id] = struct{}{}
}

func BuildNotificationPrompt(inboxID string, msg Message) string {
	body := msg.Text
	if body == "" {
		body = msg.Snippet
	}
	if len(body) > 4000 {
		body = body[:4000] + "\n[truncated]"
	}
	summary := fmt.Sprintf("Inbox ID: %s\nMessage ID: %s\nThread ID: %s\nFrom: %s <%s>\nSubject: %s\nLabels: %s\nBody:\n%s",
		inboxID,
		msg.ID,
		msg.ThreadID,
		msg.From.Name,
		msg.From.Email,
		msg.Subject,
		strings.Join(msg.Labels, ", "),
		body,
	)
	return "[AGENTMAIL NOTIFICATION] A new AgentMail message arrived.\n\n" +
		security.IsolateExternalData(summary) +
		"\n\nUse the agentmail tool with operation \"get_message\" for full content, \"reply_message\" to answer, or \"update_message_labels\" to mark it."
}

func ParseWebSocketMessageEvent(raw []byte) (WebSocketMessageEvent, bool) {
	var generic struct {
		Type      string          `json:"type"`
		InboxID   string          `json:"inbox_id"`
		Message   json.RawMessage `json:"message"`
		MessageID string          `json:"message_id"`
		Subject   string          `json:"subject"`
	}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return WebSocketMessageEvent{}, false
	}
	event := WebSocketMessageEvent{Type: generic.Type, InboxID: generic.InboxID}
	if len(generic.Message) > 0 && string(generic.Message) != "null" {
		if err := json.Unmarshal(generic.Message, &event.Message); err != nil {
			return WebSocketMessageEvent{}, false
		}
		return event, event.Message.ID != ""
	}
	if generic.MessageID != "" {
		event.Message = Message{ID: generic.MessageID, Subject: generic.Subject, InboxID: generic.InboxID}
		return event, true
	}
	return WebSocketMessageEvent{}, false
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
