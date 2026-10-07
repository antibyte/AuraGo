package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

type emailContentEvaluator interface {
	EvaluateContent(ctx context.Context, contentType string, content string) security.GuardianResult
}

const emailGuardianWorkerLimit = 4

// logTextRunes bounds the model-supplied text (account, recipient, subject, title, file
// names) that a send_email or send_telegram log line or error repeats.
const logTextRunes = 200

// emailLoggedAttachments is how many attachment names a delivered send_email logs.
const emailLoggedAttachments = 6

// emailSentResult is the result of a delivered send_email. A mail with attachments says how
// many went with it, and its log line names them, so that an incident can be traced.
func emailSentResult(logger *slog.Logger, accountID, to string, files []tools.EmailAttachment) string {
	message := fmt.Sprintf("Email sent to %s via account %s", to, accountID)
	if len(files) > 0 {
		message += fmt.Sprintf(" with %d attachment(s)", len(files))
		logger.Info("send_email delivered", "account", accountID, "attachments", len(files), "files", emailAttachmentNames(files))
	}
	return "Tool Output: " + tools.EncodeEmailResult(tools.EmailResult{Status: "success", Message: message})
}

// emailAttachmentNames lists the base names of sent attachments for the log, at most
// emailLoggedAttachments of them, each cut to logTextRunes, so a file that left AuraGo can be
// traced to its name.
func emailAttachmentNames(files []tools.EmailAttachment) string {
	names := make([]string, 0, emailLoggedAttachments)
	for _, f := range files[:min(len(files), emailLoggedAttachments)] {
		names = append(names, boundedRunes(f.Name, logTextRunes))
	}
	if len(files) > emailLoggedAttachments {
		names = append(names, fmt.Sprintf("+%d more", len(files)-emailLoggedAttachments))
	}
	return strings.Join(names, ", ")
}

// boundedRunes returns s cut to at most maxRunes runes, with an ellipsis when it was cut.
// It walks s once and allocates nothing for the cut, so a huge argument costs no copy.
func boundedRunes(s string, maxRunes int) string {
	if len(s) <= maxRunes {
		return s
	}
	n := 0
	for i := range s {
		if n == maxRunes {
			return s[:i] + "…"
		}
		n++
	}
	return s
}

func sanitizeFetchedEmails(ctx context.Context, logger *slog.Logger, guardian *security.Guardian, llmGuardian emailContentEvaluator, scanEmails bool, messages []tools.EmailMessage) []tools.EmailMessage {
	if len(messages) == 0 {
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
				combined := msg.From + " " + msg.To + " " + msg.Subject + " " + msg.Date + " " + msg.Snippet + " " + msg.Body
				var quarantine *security.GuardianResult
				if guardian != nil {
					scanRes := guardian.ScanForInjectionLocal(combined)
					if scanRes.Level >= security.ThreatHigh {
						result := security.ContentScanQuarantine(security.QuarantineSuspicious)
						quarantine = &result
					}
				}
				if quarantine == nil && scanEmails {
					if guardian == nil || llmGuardian == nil {
						result := security.ContentScanQuarantine(security.QuarantineUnavailable)
						quarantine = &result
					} else {
						llmResult := llmGuardian.EvaluateContent(ctx, "email", combined)
						if llmResult.Decision != security.DecisionAllow {
							reason := llmResult.QuarantineReason
							if reason == "" {
								reason = security.QuarantineSuspicious
							}
							result := security.ContentScanQuarantine(reason)
							quarantine = &result
						}
					}
				}
				if quarantine != nil {
					if logger != nil {
						logger.Warn("[Email] Guardian quarantined message", "uid", msg.UID, "category", quarantine.QuarantineReason)
					}
					msg.From = ""
					msg.To = ""
					msg.Date = ""
					msg.Subject = "Quarantined email"
					msg.Snippet = ""
					msg.Body = security.QuarantineNotice("email-fetch", fmt.Sprint(msg.UID), *quarantine)
					sanitized[idx] = msg
					continue
				}

				if guardian != nil {
					msg.Body = guardian.SanitizeToolOutput("email", msg.Body)
					msg.Snippet = guardian.SanitizeToolOutput("email", msg.Snippet)
				}
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
				return toolErrorf("Email account '%s' not found. Use list_email_accounts to see available accounts.", boundedRunes(req.Account, logTextRunes)), true
			}
		} else {
			acct = cfg.DefaultEmailAccount()
		}
		if acct == nil {
			return `Tool Output: {"status": "error", "message": "No active email account configured. Enable an account in Settings > Email."}`, true
		}
		if acct.Disabled {
			return toolErrorf("Email account '%s' is disabled. Enable it in Settings > Email.", acct.ID), true
		}
		if acct.ReadOnly {
			return toolErrorf("Email account '%s' is read-only. Enable sending in Settings > Email.", acct.ID), true
		}
		to := req.To
		if to == "" {
			return `Tool Output: {"status": "error", "message": "'to' (recipient address) is required"}`, true
		}
		// Every gate above has passed. Only now are the recipients validated, an attachment
		// argument judged and a file opened, so a refused call never touches the file system and
		// a recipient the senders would refuse costs no attachment read.
		if err := tools.CheckEmailRecipients(to); err != nil {
			return toolErrorJSON(err.Error()), true
		}
		if req.AttachmentsErr != nil {
			return toolErrorJSON(req.AttachmentsErr.Error()), true
		}
		subject := req.Subject
		if subject == "" {
			subject = "(no subject)"
		}
		body := req.Body
		logger.Info("LLM requested email send", "account", acct.ID, "to", boundedRunes(to, logTextRunes),
			"subject", boundedRunes(subject, logTextRunes), "attachments", len(req.Attachments))
		var sendErr error
		var files []tools.EmailAttachment
		if len(req.Attachments) > 0 {
			var err error
			files, err = tools.LoadEmailAttachments(cfg, req.Attachments)
			if err != nil {
				return "Tool Output: " + tools.EncodeEmailResult(tools.EmailResult{Status: "error", Message: err.Error()}), true
			}
			if acct.SMTPPort == 465 {
				sendErr = tools.SendEmailTLSWithAttachments(acct.SMTPHost, acct.SMTPPort, acct.Username, acct.Password, acct.FromAddress, to, subject, body, files, logger)
			} else {
				sendErr = tools.SendEmailWithAttachments(acct.SMTPHost, acct.SMTPPort, acct.Username, acct.Password, acct.FromAddress, to, subject, body, files, logger)
			}
		} else if acct.SMTPPort == 465 {
			sendErr = tools.SendEmailTLS(acct.SMTPHost, acct.SMTPPort, acct.Username, acct.Password, acct.FromAddress, to, subject, body, logger)
		} else {
			sendErr = tools.SendEmail(acct.SMTPHost, acct.SMTPPort, acct.Username, acct.Password, acct.FromAddress, to, subject, body, logger)
		}
		if sendErr != nil {
			return toolErrorf("SMTP send failed (%s): %v", acct.ID, sendErr), true
		}
		return emailSentResult(logger, acct.ID, to, files), true

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
