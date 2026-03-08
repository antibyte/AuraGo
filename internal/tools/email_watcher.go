package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// EmailWatcher polls IMAP folders for unseen messages across all configured
// email accounts and wakes the agent via a loopback HTTP request when new
// mail arrives.
type EmailWatcher struct {
	cfg      *config.Config
	logger   *slog.Logger
	guardian *security.Guardian
	stopCh   chan struct{}
	mu       sync.Mutex
	running  bool
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

func NewEmailWatcher(cfg *config.Config, logger *slog.Logger, guardian *security.Guardian) *EmailWatcher {
	return &EmailWatcher{
		cfg:      cfg,
		logger:   logger,
		guardian: guardian,
		stopCh:   make(chan struct{}),
		lastUIDs: make(map[string]map[uint32]bool),
	}
}

// Start begins the polling loop in a background goroutine.
func (ew *EmailWatcher) Start() {
	ew.mu.Lock()
	defer ew.mu.Unlock()
	if ew.running {
		return
	}
	ew.running = true

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
		// Initial seed: record current unseen UIDs without triggering
		ew.seedAllAccounts()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ew.stopCh:
				ew.logger.Info("[EmailWatcher] Stopped")
				return
			case <-ticker.C:
				ew.pollAllAccounts()
			}
		}
	}()
}

func (ew *EmailWatcher) Stop() {
	ew.mu.Lock()
	defer ew.mu.Unlock()
	if !ew.running {
		return
	}
	ew.running = false
	close(ew.stopCh)
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
func (ew *EmailWatcher) seedAllAccounts() {
	for _, acct := range ew.cfg.EmailAccounts {
		if !acct.WatchEnabled || acct.IMAPHost == "" || acct.Username == "" || acct.Password == "" {
			continue
		}
		uids, err := SearchUnseenUIDs(
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

func (ew *EmailWatcher) pollAllAccounts() {
	for _, acct := range ew.cfg.EmailAccounts {
		if !acct.WatchEnabled || acct.IMAPHost == "" || acct.Username == "" || acct.Password == "" {
			continue
		}
		ew.pollAccount(acct)
	}
}

func (ew *EmailWatcher) pollAccount(acct config.EmailAccount) {
	uids, err := SearchUnseenUIDs(
		acct.IMAPHost, acct.IMAPPort,
		acct.Username, acct.Password,
		acct.WatchFolder,
	)
	if err != nil {
		ew.logger.Warn("[EmailWatcher] Poll failed", "account", acct.ID, "error", err)
		return
	}

	if ew.lastUIDs[acct.ID] == nil {
		ew.lastUIDs[acct.ID] = make(map[uint32]bool)
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

	// Fetch the new messages for a summary
	messages, err := FetchEmails(
		acct.IMAPHost, acct.IMAPPort,
		acct.Username, acct.Password,
		acct.WatchFolder, len(newUIDs),
		ew.logger,
	)
	if err != nil {
		ew.logger.Warn("[EmailWatcher] Fetch for notification failed", "account", acct.ID, "error", err)
		ew.notifyAgent(fmt.Sprintf("[EMAIL NOTIFICATION] Account: %s — %d new email(s) in %s. Fetch details with fetch_email (account: \"%s\").", acct.Name, len(newUIDs), acct.WatchFolder, acct.ID))
		return
	}

	// Build summary and run through Guardian; fire mission triggers
	var summary string
	for i, msg := range messages {
		content := fmt.Sprintf("From: %s | Subject: %s | Snippet: %s", msg.From, msg.Subject, msg.Snippet)
		// Guardian scan on email content
		if ew.guardian != nil {
			scanResult := ew.guardian.ScanForInjection(content)
			if scanResult.Level >= security.ThreatHigh {
				ew.logger.Warn("[EmailWatcher] HIGH threat detected in email, sanitizing", "account", acct.ID, "from", msg.From, "subject", msg.Subject, "threat", scanResult.Level.String())
				content = fmt.Sprintf("From: %s | Subject: [SANITIZED - injection detected] | Snippet: [REDACTED]", msg.From)
			}
		}
		summary += fmt.Sprintf("\n%d. %s", i+1, content)

		// Fire registered mission triggers
		ew.mu.Lock()
		for _, mt := range ew.missionCallbacks {
			if mt.folder != "" && mt.folder != acct.WatchFolder {
				continue
			}
			if mt.subjectContains != "" && !containsCI(msg.Subject, mt.subjectContains) {
				continue
			}
			if mt.fromContains != "" && !containsCI(msg.From, mt.fromContains) {
				continue
			}
			go mt.callback(msg.Subject, msg.From, msg.Body)
		}
		ew.mu.Unlock()
	}

	prompt := fmt.Sprintf("[EMAIL NOTIFICATION] Account: %s (%s) — %d new email(s) in %s:%s\n\nYou can use fetch_email with account \"%s\" for full content or send_email to reply.", acct.Name, acct.FromAddress, len(messages), acct.WatchFolder, summary, acct.ID)
	ew.notifyAgent(prompt)
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

// notifyAgent sends a loopback HTTP request to wake the agent.
func (ew *EmailWatcher) notifyAgent(prompt string) {
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", ew.cfg.Server.Port)

	msg := map[string]interface{}{
		"model": ew.cfg.LLM.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	payload, _ := json.Marshal(msg)

	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		ew.logger.Error("[EmailWatcher] Loopback notification failed", "error", err)
		return
	}
	resp.Body.Close()
	ew.logger.Info("[EmailWatcher] Agent notified", "status", resp.Status)
}

// StartEmailWatcher creates and starts an email watcher if any account has
// watch_enabled=true (or the legacy email.enabled + watch_enabled is set).
func StartEmailWatcher(cfg *config.Config, logger *slog.Logger, guardian *security.Guardian) *EmailWatcher {
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

	watcher := NewEmailWatcher(cfg, logger, guardian)
	watcher.Start()
	return watcher
}
