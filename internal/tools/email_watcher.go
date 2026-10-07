package tools

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// EmailWatcher polls IMAP folders for unseen messages across all configured
// email accounts and wakes the agent via a loopback HTTP request when new
// mail arrives.
type EmailWatcher struct {
	cfg         *config.Config
	logger      *slog.Logger
	guardian    *security.Guardian
	llmGuardian *security.LLMGuardian
	relaySheet  EmailRelayCheatsheet
	// internalToken is the per-process loopback auth token. When set, the
	// wake-the-agent request carries the internal follow-up header and token so
	// the turn is labelled internal and is accepted when web auth is enabled.
	internalToken string
	cancel        context.CancelFunc
	done          chan struct{}
	mu            sync.Mutex
	running       bool
	// per-account UID tracking: accountID → set of known UIDs
	lastUIDs map[string]map[uint32]bool
	// mission trigger callbacks
	missionCallbacks []missionTriggerCallback
}

type missionTriggerCallback struct {
	folder          string
	subjectContains string
	fromContains    string
	callback        func(subject, from, body string)
}

// EmailRelayCheatsheet contains optional trusted instructions appended to
// inbound email notifications after external email content has been isolated.
type EmailRelayCheatsheet struct {
	ID      string
	Name    string
	Content string
}

func NewEmailWatcher(cfg *config.Config, logger *slog.Logger, guardian *security.Guardian, llmGuardian *security.LLMGuardian) *EmailWatcher {
	return &EmailWatcher{
		cfg:         cfg,
		logger:      logger,
		guardian:    guardian,
		llmGuardian: llmGuardian,
		lastUIDs:    make(map[string]map[uint32]bool),
	}
}

// Start begins the polling loop in a background goroutine.
func (ew *EmailWatcher) Start() {
	ew.StartContext(context.Background())
}

func (ew *EmailWatcher) StartContext(parent context.Context) {
	ew.mu.Lock()
	defer ew.mu.Unlock()
	if ew.running {
		return
	}
	ew.running = true
	ctx, cancel := context.WithCancel(parent)
	ew.cancel = cancel
	done := make(chan struct{})
	ew.done = done

	// Find minimum interval across accounts
	interval := 120 * time.Second
	for _, acct := range ew.cfg.EmailAccounts {
		if !acct.WatchEnabled {
			continue
		}
		acctInterval := time.Duration(acct.WatchInterval) * time.Second
		if acctInterval > 0 && acctInterval < interval {
			interval = acctInterval
		}
	}
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}

	ew.logger.Info("[EmailWatcher] Starting multi-account watcher", "accounts", len(ew.cfg.EmailAccounts), "interval", interval)

	go func() {
		defer func() {
			cancel()
			ew.mu.Lock()
			ew.running = false
			close(done)
			ew.mu.Unlock()
		}()
		// Initial seed: record current unseen UIDs without triggering
		ew.seedAllAccounts(ctx)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				ew.logger.Info("[EmailWatcher] Stopped")
				return
			case <-ticker.C:
				ew.pollAllAccounts(ctx)
			}
		}
	}()
}

func (ew *EmailWatcher) Stop() {
	ew.mu.Lock()
	if !ew.running {
		ew.mu.Unlock()
		return
	}
	ew.cancel()
	done := ew.done
	ew.mu.Unlock()
	<-done
}

// RegisterMissionTrigger registers a callback for email-triggered missions.
func (ew *EmailWatcher) RegisterMissionTrigger(folder, subjectContains, fromContains string, callback func(subject, from, body string)) {
	ew.mu.Lock()
	defer ew.mu.Unlock()
	ew.missionCallbacks = append(ew.missionCallbacks, missionTriggerCallback{
		folder:          folder,
		subjectContains: subjectContains,
		fromContains:    fromContains,
		callback:        callback,
	})
}

// seedAllAccounts fetches current unseen UIDs for every watched account so we
// don't alert on old mail at startup.
func (ew *EmailWatcher) seedAllAccounts(ctx context.Context) {
	for _, acct := range ew.cfg.EmailAccounts {
		if ctx.Err() != nil {
			return
		}
		if !acct.WatchEnabled || acct.IMAPHost == "" || acct.Username == "" || acct.Password == "" {
			continue
		}
		uids, err := searchUnseenUIDsContext(ctx,
			acct.IMAPHost, acct.IMAPPort,
			acct.Username, acct.Password,
			acct.WatchFolder,
		)
		if err != nil {
			ew.logger.Warn("[EmailWatcher] Seed fetch failed", "account", acct.ID, "error", err)
			continue
		}
		uidSet := make(map[uint32]bool, len(uids))
		for _, uid := range uids {
			uidSet[uid] = true
		}
		ew.lastUIDs[acct.ID] = uidSet
		ew.logger.Info("[EmailWatcher] Seeded existing unseen UIDs", "account", acct.ID, "count", len(uids))
	}
}

func (ew *EmailWatcher) pollAllAccounts(ctx context.Context) {
	for _, acct := range ew.cfg.EmailAccounts {
		if ctx.Err() != nil {
			return
		}
		if !acct.WatchEnabled || acct.IMAPHost == "" || acct.Username == "" || acct.Password == "" {
			continue
		}
		ew.pollAccount(ctx, acct)
	}
}

func (ew *EmailWatcher) pollAccount(ctx context.Context, acct config.EmailAccount) {
	uids, err := searchUnseenUIDsContext(ctx,
		acct.IMAPHost, acct.IMAPPort,
		acct.Username, acct.Password,
		acct.WatchFolder,
	)
	if err != nil {
		ew.logger.Warn("[EmailWatcher] Poll failed", "account", acct.ID, "error", err)
		return
	}

	if ew.lastUIDs[acct.ID] == nil {
		// A failed initial seed must succeed before old mail can be relayed.
		ew.lastUIDs[acct.ID] = make(map[uint32]bool, len(uids))
		for _, uid := range uids {
			ew.lastUIDs[acct.ID][uid] = true
		}
		return
	}

	var newUIDs []uint32
	for _, uid := range uids {
		if !ew.lastUIDs[acct.ID][uid] {
			newUIDs = append(newUIDs, uid)
			ew.lastUIDs[acct.ID][uid] = true
		}
	}

	if len(newUIDs) == 0 {
		return
	}

	ew.logger.Info("[EmailWatcher] New unseen emails detected", "account", acct.ID, "count", len(newUIDs))

	// Fetch the specific new messages by UID (not by sequence number)
	messages, err := fetchEmailsByUIDContext(ctx,
		acct.IMAPHost, acct.IMAPPort,
		acct.Username, acct.Password,
		acct.WatchFolder, newUIDs,
		ew.logger,
	)
	if err != nil {
		ew.logger.Warn("[EmailWatcher] Fetch for notification failed", "account", acct.ID, "error", err)
		ew.notifyAgent(ctx, fmt.Sprintf("[EMAIL NOTIFICATION] Account: %s — %d new email(s) in %s. Fetch details with fetch_email (account: \"%s\").", acct.Name, len(newUIDs), acct.WatchFolder, acct.ID))
		return
	}

	// Build summary and run through Guardian; fire mission triggers
	var summary string
	for i, msg := range messages {
		if ctx.Err() != nil {
			return
		}
		content := fmt.Sprintf("From: %s | Subject: %s | Snippet: %s", msg.From, msg.Subject, msg.Snippet)
		// Guardian scan on email content
		if ew.guardian != nil {
			scanResult := ew.guardian.ScanForInjection(content)
			if scanResult.Level >= security.ThreatHigh {
				ew.logger.Warn("[EmailWatcher] HIGH threat detected in email, sanitizing", "account", acct.ID, "from", msg.From, "subject", msg.Subject, "threat", scanResult.Level.String())
				content = fmt.Sprintf("From: %s | Subject: %s | Snippet: %s", msg.From, security.SanitizedText("guardian scan flagged this message"), security.RedactedText(""))
			} else if ew.llmGuardian != nil && ew.cfg.LLMGuardian.ScanEmails {
				// LLM Guardian: deeper content scan if regex didn't flag HIGH
				llmResult := ew.llmGuardian.EvaluateContent(ctx, "email", content)
				if llmResult.Decision == security.DecisionBlock {
					ew.logger.Warn("[EmailWatcher] LLM Guardian blocked email content", "account", acct.ID, "from", msg.From, "reason", llmResult.Reason)
					content = fmt.Sprintf("From: %s | Subject: %s | Snippet: %s", msg.From, security.SanitizedText("llm guardian blocked message: "+llmResult.Reason), security.RedactedText(""))
				}
			}
		}
		summary += fmt.Sprintf("\n%d. %s", i+1, content)

		// Fire registered mission triggers
		ew.mu.Lock()
		callbacks := append([]missionTriggerCallback(nil), ew.missionCallbacks...)
		ew.mu.Unlock()
		for _, mt := range callbacks {
			if ctx.Err() != nil {
				return
			}
			if mt.folder != "" && mt.folder != acct.WatchFolder {
				continue
			}
			if mt.subjectContains != "" && !containsCI(msg.Subject, mt.subjectContains) {
				continue
			}
			if mt.fromContains != "" && !containsCI(msg.From, mt.fromContains) {
				continue
			}
			mt.callback(msg.Subject, msg.From, msg.Body)
		}
	}

	ew.notifyAgent(ctx, buildEmailNotificationPrompt(acct, len(messages), summary, ew.relaySheet))
}

func buildEmailNotificationPrompt(acct config.EmailAccount, messageCount int, summary string, relaySheets ...EmailRelayCheatsheet) string {
	summaryBlock := ""
	if summary != "" {
		summaryBlock = "\n\nEmail summary:\n" + security.IsolateExternalData(summary)
	}
	prompt := fmt.Sprintf("[EMAIL NOTIFICATION] Account: %s (%s) - %d new email(s) in %s.%s\n\nYou can use fetch_email with account \"%s\" for full content or send_email to reply.", acct.Name, acct.FromAddress, messageCount, acct.WatchFolder, summaryBlock, acct.ID)
	if len(relaySheets) > 0 {
		prompt += formatEmailRelayCheatsheet(relaySheets[0])
	}
	return prompt
}

func formatEmailRelayCheatsheet(sheet EmailRelayCheatsheet) string {
	content := strings.TrimSpace(sheet.Content)
	if content == "" {
		return ""
	}
	if len(content) > 6000 {
		content = content[:6000] + "\n[truncated]"
	}
	name := strings.TrimSpace(sheet.Name)
	if name == "" {
		name = strings.TrimSpace(sheet.ID)
	}
	if name == "" {
		name = "configured email relay cheatsheet"
	}
	return "\n\n[EMAIL CHEATSHEET INSTRUCTIONS]\n" +
		"Cheatsheet: " + name + "\n\n" +
		content
}

// containsCI is a case-insensitive contains check.
func containsCI(s, substr string) bool {
	return len(s) >= len(substr) && (substr == "" || len(s) > 0 && containsFold(s, substr))
}

func containsFold(s, substr string) bool {
	// Simple case-insensitive contains
	sl := len(substr)
	for i := 0; i+sl <= len(s); i++ {
		if equalFold(s[i:i+sl], substr) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ac, bc := a[i], b[i]
		if ac >= 'A' && ac <= 'Z' {
			ac += 'a' - 'A'
		}
		if bc >= 'A' && bc <= 'Z' {
			bc += 'a' - 'A'
		}
		if ac != bc {
			return false
		}
	}
	return true
}

// internalAPIURL returns the base URL for internal loopback API calls, respecting HTTPS config.
func (ew *EmailWatcher) internalAPIURL() string {
	scheme := "http"
	port := ew.cfg.Server.Port
	if ew.cfg.Server.HTTPS.Enabled {
		scheme = "https"
		if ew.cfg.Server.HTTPS.HTTPSPort > 0 {
			port = ew.cfg.Server.HTTPS.HTTPSPort
		} else {
			port = 443
		}
	}
	return fmt.Sprintf("%s://127.0.0.1:%d", scheme, port)
}

// notifyAgent sends a loopback HTTP request to wake the agent.
func (ew *EmailWatcher) notifyAgent(ctx context.Context, prompt string) {
	url := ew.internalAPIURL() + "/v1/chat/completions"

	msg := map[string]interface{}{
		"model": ew.cfg.LLM.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	payload, _ := json.Marshal(msg)

	// Use custom transport to skip TLS verification for self-signed certs on loopback
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 3 * time.Minute, Transport: transport, CheckRedirect: security.SameOriginRedirect}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if ew.internalToken != "" {
		req.Header.Set("X-Internal-FollowUp", "true")
		req.Header.Set("X-Internal-Token", ew.internalToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		ew.logger.Error("[EmailWatcher] Loopback notification failed", "error", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		ew.logger.Warn("[EmailWatcher] Agent notification rejected", "status", resp.StatusCode)
		return
	}
	ew.logger.Info("[EmailWatcher] Agent notified", "status", resp.Status)
}

// StartEmailWatcher creates and starts an email watcher if any account has
// watch_enabled=true (or the legacy email.enabled + watch_enabled is set).
func StartEmailWatcher(cfg *config.Config, logger *slog.Logger, guardian *security.Guardian, llmGuardian *security.LLMGuardian, internalToken string, cheatsheetDBs ...*sql.DB) *EmailWatcher {
	return StartEmailWatcherContext(context.Background(), cfg, logger, guardian, llmGuardian, internalToken, cheatsheetDBs...)
}

func StartEmailWatcherContext(ctx context.Context, cfg *config.Config, logger *slog.Logger, guardian *security.Guardian, llmGuardian *security.LLMGuardian, internalToken string, cheatsheetDBs ...*sql.DB) *EmailWatcher {
	hasWatchAccount := false
	for _, acct := range cfg.EmailAccounts {
		if acct.WatchEnabled && acct.IMAPHost != "" && acct.Username != "" && acct.Password != "" {
			hasWatchAccount = true
			break
		}
	}
	if !hasWatchAccount {
		if cfg.Email.Enabled && !cfg.Email.WatchEnabled {
			logger.Info("[Email] Email enabled but watch_enabled is false — watcher not started")
		}
		return nil
	}

	watcher := NewEmailWatcher(cfg, logger, guardian, llmGuardian)
	watcher.internalToken = internalToken
	if len(cheatsheetDBs) > 0 {
		watcher.relaySheet = loadEmailRelayCheatsheet(cheatsheetDBs[0], cfg.Email.RelayCheatsheetID, logger)
	}
	watcher.StartContext(ctx)
	return watcher
}

func loadEmailRelayCheatsheet(db *sql.DB, id string, logger *slog.Logger) EmailRelayCheatsheet {
	id = strings.TrimSpace(id)
	if db == nil || id == "" {
		return EmailRelayCheatsheet{}
	}
	sheet, err := CheatsheetGet(db, id)
	if err != nil {
		if logger != nil {
			logger.Warn("[EmailWatcher] Failed to load relay cheatsheet", "id", id, "error", err)
		}
		return EmailRelayCheatsheet{}
	}
	return EmailRelayCheatsheet{
		ID:      sheet.ID,
		Name:    sheet.Name,
		Content: sheet.Content,
	}
}
