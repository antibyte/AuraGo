package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

type emailContentEvaluator interface {
	EvaluateContent(ctx context.Context, contentType string, content string) security.GuardianResult
}

const emailGuardianWorkerLimit = 4

func sanitizeFetchedEmails(ctx context.Context, logger *slog.Logger, guardian *security.Guardian, llmGuardian emailContentEvaluator, scanEmails bool, messages []tools.EmailMessage) []tools.EmailMessage {
	if guardian == nil || len(messages) == 0 {
		return messages
	}

	sanitized := make([]tools.EmailMessage, len(messages))
	workerCount := emailGuardianWorkerLimit
	if len(messages) < workerCount {
		workerCount = len(messages)
	}

	indexCh := make(chan int, len(messages))
	var wg sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexCh {
				msg := messages[idx]
				combined := msg.From + " " + msg.Subject + " " + msg.Body
				scanRes := guardian.ScanForInjection(combined)
				if scanRes.Level >= security.ThreatHigh {
					if logger != nil {
						logger.Warn("[Email] Guardian HIGH threat in message", "uid", msg.UID, "from", msg.From, "threat", scanRes.Level.String())
					}
					msg.Body = security.RedactedText("guardian blocked content after injection detection")
					msg.Subject = security.SanitizedText("guardian scan flagged this message")
					msg.Snippet = security.RedactedText("")
					sanitized[idx] = msg
					continue
				}

				if llmGuardian != nil && scanEmails {
					llmResult := llmGuardian.EvaluateContent(ctx, "email", combined)
					if llmResult.Decision == security.DecisionBlock {
						if logger != nil {
							logger.Warn("[Email] LLM Guardian blocked email content", "uid", msg.UID, "from", msg.From, "reason", llmResult.Reason)
						}
						msg.Body = security.RedactedText("llm guardian blocked content: " + llmResult.Reason)
						msg.Subject = security.SanitizedText("llm guardian blocked this message")
						msg.Snippet = security.RedactedText("")
						sanitized[idx] = msg
						continue
					}
				}

				msg.Body = guardian.SanitizeToolOutput("email", msg.Body)
				sanitized[idx] = msg
			}
		}()
	}

	for idx := range messages {
		indexCh <- idx
	}
	close(indexCh)
	wg.Wait()

	return sanitized
}

func dispatchEmailCases(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	cfg := dc.Cfg
	logger := dc.Logger
	guardian := dc.Guardian
	llmGuardian := dc.LLMGuardian

	switch tc.Action {
	case "fetch_email", "check_email":
		if !cfg.Email.Enabled && len(cfg.EmailAccounts) == 0 {
			return `Tool Output: {"status": "error", "message": "Email is not enabled. Configure the email section in config.yaml or add email_accounts."}`, true
		}
		req := decodeEmailFetchArgs(tc)
		var acct *config.EmailAccount
		if req.Account != "" {
			acct = cfg.FindEmailAccount(req.Account)
			if acct == nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' not found. Use list_email_accounts to see available accounts."}`, req.Account), true
			}
		} else {
			acct = cfg.DefaultEmailAccount()
		}
		if acct == nil {
			return `Tool Output: {"status": "error", "message": "No active email account configured. Enable an account in Settings > Email."}`, true
		}
		if acct.Disabled {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' is disabled. Enable it in Settings > Email."}`, acct.ID), true
		}
		logger.Info("LLM requested email fetch", "account", acct.ID, "folder", req.Folder)
		folder := req.Folder
		if folder == "" {
			folder = acct.WatchFolder
		}
		limit := req.Limit
		if limit <= 0 {
			limit = 10
		}
		messages, err := tools.FetchEmails(
			acct.IMAPHost, acct.IMAPPort,
			acct.Username, acct.Password,
			folder, limit, logger,
		)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "IMAP fetch failed (%s): %v"}`, acct.ID, err), true
		}
		messages = sanitizeFetchedEmails(ctx, logger, guardian, llmGuardian, cfg.LLMGuardian.ScanEmails, messages)
		result := tools.EmailResult{Status: "success", Count: len(messages), Data: messages, Message: fmt.Sprintf("Account: %s", acct.ID)}
		return "Tool Output: " + tools.EncodeEmailResult(result), true

	case "send_email":
		if !cfg.Email.Enabled && len(cfg.EmailAccounts) == 0 {
			return `Tool Output: {"status": "error", "message": "Email is not enabled. Configure the email section in config.yaml or add email_accounts."}`, true
		}
		req := decodeEmailSendArgs(tc)
		var acct *config.EmailAccount
		if req.Account != "" {
			acct = cfg.FindEmailAccount(req.Account)
			if acct == nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' not found. Use list_email_accounts to see available accounts."}`, req.Account), true
			}
		} else {
			acct = cfg.DefaultEmailAccount()
		}
		if acct == nil {
			return `Tool Output: {"status": "error", "message": "No active email account configured. Enable an account in Settings > Email."}`, true
		}
		if acct.Disabled {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' is disabled. Enable it in Settings > Email."}`, acct.ID), true
		}
		if acct.ReadOnly {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' is read-only. Enable sending in Settings > Email."}`, acct.ID), true
		}
		to := req.To
		if to == "" {
			return `Tool Output: {"status": "error", "message": "'to' (recipient address) is required"}`, true
		}
		subject := req.Subject
		if subject == "" {
			subject = "(no subject)"
		}
		body := req.Body
		logger.Info("LLM requested email send", "account", acct.ID, "to", to, "subject", subject)
		var sendErr error
		if acct.SMTPPort == 465 {
			sendErr = tools.SendEmailTLS(acct.SMTPHost, acct.SMTPPort, acct.Username, acct.Password, acct.FromAddress, to, subject, body, logger)
		} else {
			sendErr = tools.SendEmail(acct.SMTPHost, acct.SMTPPort, acct.Username, acct.Password, acct.FromAddress, to, subject, body, logger)
		}
		if sendErr != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "SMTP send failed (%s): %v"}`, acct.ID, sendErr), true
		}
		result := tools.EmailResult{Status: "success", Message: fmt.Sprintf("Email sent to %s via account %s", to, acct.ID)}
		return "Tool Output: " + tools.EncodeEmailResult(result), true

	case "list_email_accounts":
		if len(cfg.EmailAccounts) == 0 {
			return `Tool Output: {"status": "success", "count": 0, "data": [], "message": "No email accounts configured."}`, true
		}
		type acctInfo struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			IMAP      string `json:"imap"`
			SMTP      string `json:"smtp"`
			Watcher   bool   `json:"watcher"`
			Enabled   bool   `json:"enabled"`
			AllowSend bool   `json:"allow_sending"`
		}
		var accts []acctInfo
		for _, a := range cfg.EmailAccounts {
			accts = append(accts, acctInfo{
				ID:        a.ID,
				Name:      a.Name,
				Email:     a.FromAddress,
				IMAP:      fmt.Sprintf("%s:%d", a.IMAPHost, a.IMAPPort),
				SMTP:      fmt.Sprintf("%s:%d", a.SMTPHost, a.SMTPPort),
				Watcher:   a.WatchEnabled,
				Enabled:   !a.Disabled,
				AllowSend: !a.ReadOnly,
			})
		}
		result := tools.EmailResult{Status: "success", Count: len(accts), Data: accts}
		return "Tool Output: " + tools.EncodeEmailResult(result), true
	}
	return "", false
}
