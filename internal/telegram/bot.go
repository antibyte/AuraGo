package telegram

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/commands"
	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/llm"
	"aurago/internal/media"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/tools"
	"aurago/internal/webhooks"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sashabaranov/go-openai"
)

func buildTelegramAgentMessages(historyManager *memory.HistoryManager) []openai.ChatCompletionMessage {
	finalMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem},
	}
	if historyManager == nil {
		return finalMessages
	}
	currentSummary := historyManager.GetSummary()
	if currentSummary != "" {
		finalMessages = append(finalMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: agent.FormatContextRecapForPrompt(currentSummary),
		})
	}
	return append(finalMessages, historyManager.Get()...)
}

// telegramHTTPTimeout bounds a single Telegram request. It exceeds the 60 s
// long-poll window so a healthy poll is never cut short, while a stuck
// connection can no longer block the polling loop forever.
const telegramHTTPTimeout = 90 * time.Second

// newTelegramHTTPClient builds the HTTP client used for Telegram requests.
func newTelegramHTTPClient() *http.Client {
	return &http.Client{Timeout: telegramHTTPTimeout}
}

// StartLongPolling starts one cancellable generation without waiting for network I/O.
func StartLongPolling(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, plannerDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, remoteHub *remote.RemoteHub, guardian *security.Guardian, budgetTrackerSnapshot func() *budget.Tracker, webhookManager *webhooks.Manager, snapshot func() (*config.Config, llm.ChatClient), routingDecisions agent.TaskDecisionService, offsets *UpdateOffsets) *Bot {
	if cfg != nil {
		cfg = cfg.Clone()
	}
	return startLongPolling(ctx, cfg, logger, plannerDB, offsets, tgbotapi.APIEndpoint, newTelegramHTTPClient(), func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) {
		current, currentClient := cfg, client
		if snapshot != nil {
			current, currentClient = snapshot()
		}
		if current == nil || current.EggMode.Enabled || current.Telegram != cfg.Telegram || ctx.Err() != nil {
			return
		}
		processUpdate(ctx, bot, update, current, logger, currentClient, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB, missionManagerV2, remoteHub, guardian, budgetTrackerSnapshot, webhookManager, routingDecisions)
	})
}

const telegramPollingIssueFingerprint = "telegram|long_polling"

func acquireTelegramWorker(ctx context.Context, slots chan struct{}) bool {
	if ctx.Err() != nil {
		return false
	}
	select {
	case slots <- struct{}{}:
		if ctx.Err() != nil {
			<-slots
			return false
		}
		return true
	case <-ctx.Done():
		return false
	}
}

var telegramPollingInitialBackoff = time.Second

const (
	// telegramOffsetPersistEveryUpdates and telegramOffsetPersistInterval
	// bound how long a handed-out cursor may stay unpersisted.
	telegramOffsetPersistEveryUpdates = 10
	telegramOffsetPersistInterval     = 30 * time.Second
	// telegramMaxRetryAfterSeconds caps the flood-control delay Telegram asks
	// for, so a hostile or buggy response cannot stall the poller.
	telegramMaxRetryAfterSeconds = 60
	// telegramPollingBackoffJitter spreads retries by ±20 %.
	telegramPollingBackoffJitter = 0.2
)

type telegramUpdatePoller interface {
	GetUpdates(config tgbotapi.UpdateConfig) ([]tgbotapi.Update, error)
}

func runPollingLoop(ctx context.Context, poller telegramUpdatePoller, updateCfg tgbotapi.UpdateConfig, plannerDB *sql.DB, logger *slog.Logger, handle func(tgbotapi.Update) bool, persist func(offset int)) int {
	backoff := telegramPollingInitialBackoff
	pending := 0
	lastPersist := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return updateCfg.Offset
		}
		updates, err := poller.GetUpdates(updateCfg)
		if ctx.Err() != nil {
			return updateCfg.Offset
		}
		now := time.Now()
		if err != nil {
			code := telegramPollErrorCode(err)
			failures := integrationstatus.MarkTelegramPollFailure(code, now)
			if failures >= 3 && plannerDB != nil {
				_, recordErr := planner.RecordOperationalIssue(plannerDB, planner.OperationalIssue{
					Source: "telegram", Context: "long_polling", Title: "Telegram polling is repeatedly failing",
					Detail:   "Telegram long polling failed repeatedly. Runtime diagnostics expose only the sanitized error code.",
					Severity: "warning", Kind: planner.OperationalIssueKindRuntimeFailure,
					Reference: code, Fingerprint: telegramPollingIssueFingerprint, OccurredAt: now,
				})
				if recordErr != nil && logger != nil {
					logger.Warn("Failed to record Telegram polling issue", "error", recordErr)
				}
			}
			wait := telegramPollBackoff(backoff)
			if retryAfter, ok := telegramRetryAfter(err); ok {
				// Flood control defines its own delay; the local backoff would
				// only retry too early.
				wait = retryAfter
			}
			if logger != nil {
				logger.Warn("Telegram long poll failed", "error_code", code, "consecutive_failures", failures, "retry_in", wait)
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return updateCfg.Offset
			case <-timer.C:
			}
			if backoff < 30*time.Second {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
			continue
		}
		integrationstatus.MarkTelegramPollSuccess(now)
		if plannerDB != nil {
			_, _ = planner.ResolveOperationalIssue(plannerDB, telegramPollingIssueFingerprint, "Telegram polling completed successfully.", now)
		}
		backoff = telegramPollingInitialBackoff
		for _, update := range updates {
			if ctx.Err() != nil {
				return updateCfg.Offset
			}
			if update.UpdateID < updateCfg.Offset {
				continue
			}
			if handle != nil && !handle(update) {
				return updateCfg.Offset
			}
			// The cursor advances only after the update was handed to the
			// worker, so a crash may replay at most this one update.
			updateCfg.Offset = update.UpdateID + 1
			pending++
			if persist != nil && (pending >= telegramOffsetPersistEveryUpdates || time.Since(lastPersist) >= telegramOffsetPersistInterval) {
				persist(updateCfg.Offset)
				pending = 0
				lastPersist = time.Now()
			}
		}
	}
}

// telegramPollBackoff spreads the retry delay by ±20 % so parallel pollers do
// not reconnect in lockstep.
func telegramPollBackoff(base time.Duration) time.Duration {
	if base <= 0 {
		return base
	}
	factor := 1 - telegramPollingBackoffJitter + rand.Float64()*(2*telegramPollingBackoffJitter)
	return time.Duration(float64(base) * factor)
}

// telegramRetryAfter reports the flood-control delay Telegram advertised with
// a 429 response.
func telegramRetryAfter(err error) (time.Duration, bool) {
	var tgErr *tgbotapi.Error
	if !errors.As(err, &tgErr) || tgErr.Code != 429 || tgErr.RetryAfter <= 0 {
		return 0, false
	}
	seconds := tgErr.RetryAfter
	if seconds > telegramMaxRetryAfterSeconds {
		seconds = telegramMaxRetryAfterSeconds
	}
	return time.Duration(seconds) * time.Second, true
}

func telegramPollErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var tgErr *tgbotapi.Error
	if errors.As(err, &tgErr) && tgErr.Code == 429 {
		return "telegram_poll_rate_limited"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "429"), strings.Contains(message, "rate limit"):
		return "telegram_poll_rate_limited"
	case strings.Contains(message, "timeout"), strings.Contains(message, "deadline"):
		return "telegram_poll_timeout"
	default:
		return "telegram_poll_failed"
	}
}

// SendTestMessage sends exactly one fixed, manually triggered test message.
func SendTestMessage(cfg *config.Config, now time.Time) error {
	if cfg == nil || strings.TrimSpace(cfg.Telegram.BotToken) == "" || cfg.Telegram.UserID == 0 {
		return fmt.Errorf("telegram is not configured")
	}
	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return fmt.Errorf("initialize telegram test client: %w", err)
	}
	text := telegramTestMessage(now)
	if _, err := bot.Send(tgbotapi.NewMessage(cfg.Telegram.UserID, text)); err != nil {
		return fmt.Errorf("send telegram test message: %w", err)
	}
	integrationstatus.MarkTelegramTest(now)
	return nil
}

func telegramTestMessage(now time.Time) string {
	return "AuraGo Telegram test (manual) — " + now.UTC().Format(time.RFC3339)
}

func processUpdate(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, remoteHub *remote.RemoteHub, guardian *security.Guardian, budgetTrackerSnapshot func() *budget.Tracker, webhookManager *webhooks.Manager, routingDecisions ...agent.TaskDecisionService) {
	var budgetTracker *budget.Tracker
	if budgetTrackerSnapshot != nil {
		budgetTracker = budgetTrackerSnapshot()
	}
	ctx = budget.WithTracking(ctx, budgetTracker, "chat")
	// Maintenance check: Inform the user but allow the tool-based interaction
	inMaintenance := tools.IsBusy()
	if inMaintenance {
		logger.Info("Telegram processing in Maintenance Mode")
	}
	if ctx.Err() != nil {
		return
	}
	msg := update.Message
	sessionID, err := shortTermMem.ResolveUserSession(ctx, "default")
	if err != nil {
		logger.Error("Resolve Telegram conversation", "error", err)
		return
	}
	ctx, releaseSession, err := shortTermMem.RegisterSessionRun(ctx, sessionID)
	if err != nil {
		return
	}
	defer releaseSession()
	if sessionID != "default" {
		historyManager = nil
	}
	logger.Info("Received authorized Telegram message", "id", msg.From.ID, "hasText", msg.Text != "", "hasVoice", msg.Voice != nil, "hasPhoto", len(msg.Photo) > 0)

	inputText := msg.Text
	if msg.Caption != "" {
		if inputText == "" {
			inputText = msg.Caption
		} else {
			inputText = inputText + "\n" + msg.Caption
		}
	}

	// If it's a voice message, process it
	if msg.Voice != nil {
		logger.Info("Attempting voice transcription", "file_id", msg.Voice.FileID)

		// 1. Get File URL
		fileConfig := tgbotapi.FileConfig{FileID: msg.Voice.FileID}
		file, err := bot.GetFile(fileConfig)
		if err != nil {
			logger.Error("Failed to get voice file info", "error", err)
			return
		}
		if ctx.Err() != nil {
			return
		}

		oggURL := file.Link(cfg.Telegram.BotToken)

		// 2. Download the .ogg file (we can reuse the logic but needs adjustment)
		oggPath, err := downloadFile(ctx, oggURL, logger)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("Failed to download voice file", "error", err)
			return
		}
		defer os.Remove(oggPath)

		// 3. Convert to .mp3 (better for multimodal APIs)
		mp3Path := oggPath + ".mp3"
		defer os.Remove(mp3Path)
		if err := ConvertOggToMp3Context(ctx, oggPath, mp3Path); err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("Failed to convert voice file to mp3", "error", err)
			return
		}
		if ctx.Err() != nil {
			return
		}

		// 4. Transcribe the application-owned temporary audio in memory. The
		// agent-facing file API intentionally accepts workspace paths only.
		audioData, err := os.ReadFile(mp3Path)
		if err != nil {
			logger.Error("Failed to read converted voice file", "error", err)
			return
		}
		text, _, err := tools.TranscribeAudio(ctx, filepath.Base(mp3Path), audioData, cfg)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("Failed to transcribe voice", "error", err)
			return
		}
		if ctx.Err() != nil {
			return
		}

		logger.Info("Voice transcription successful", "text_len", len(text))
		inputText = text
	}

	// Pin the caption or voice transcription before adding image/file contents.
	routingText := inputText
	humanInput := memory.NewHumanInputEvent(inputText, sessionID, fmt.Sprintf("telegram:%d:%d", msg.Chat.ID, msg.MessageID), time.Time{})
	if msg.Date > 0 {
		humanInput.OccurredAt = time.Unix(int64(msg.Date), 0).UTC()
	}
	// If it's a photo, process it
	if len(msg.Photo) > 0 {
		// Get the largest photo (usually the last one)
		photo := msg.Photo[len(msg.Photo)-1]
		logger.Info("Attempting image analysis", "file_id", photo.FileID)

		// 1. Get File URL
		fileConfig := tgbotapi.FileConfig{FileID: photo.FileID}
		file, err := bot.GetFile(fileConfig)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("Failed to get photo file info", "error", err)
		} else {
			if ctx.Err() != nil {
				return
			}
			imgURL := file.Link(cfg.Telegram.BotToken)

			// 2. Download the file
			imgPath, err := downloadFile(ctx, imgURL, logger)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Error("Failed to download photo file", "error", err)
			} else {
				defer os.Remove(imgPath)

				// 3. Analyze via Vision API
				analysis, err := AnalyzeImageContext(ctx, imgPath, cfg)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					logger.Error("Failed to analyze image", "error", err)
					if errors.Is(err, tools.ErrVisionPublicURLRequired) {
						analysis = "[Image analysis unavailable: " + tools.VisionPublicURLRequiredMessage + "]"
					} else {
						analysis = "[Error analyzing image]"
					}
				}

				logger.Info("Image analysis successful", "length", len(analysis))
				if inputText != "" {
					inputText = "[USER SENT AN IMAGE]\n" + analysis + "\n\n[USER CAPTION/TEXT]:\n" + inputText
				} else {
					inputText = "[USER SENT AN IMAGE]\n" + analysis
				}
			}
		}
	}

	// If it's a document/file attachment, save it to the agent's workdir
	if msg.Document != nil {
		logger.Info("Received Telegram document", "filename", msg.Document.FileName, "mime", msg.Document.MimeType)

		fileConfig := tgbotapi.FileConfig{FileID: msg.Document.FileID}
		file, err := bot.GetFile(fileConfig)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("Failed to get document file info", "error", err)
		} else {
			if ctx.Err() != nil {
				return
			}
			docURL := file.Link(cfg.Telegram.BotToken)
			attachDir := filepath.Join(cfg.Directories.WorkspaceDir, "attachments")
			savedPath, err := media.SaveAttachmentContext(ctx, docURL, msg.Document.FileName, attachDir)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Error("Failed to save Telegram document", "error", err)
			} else {
				if ctx.Err() != nil {
					_ = os.Remove(savedPath)
					return
				}
				agentPath := "agent_workspace/workdir/attachments/" + filepath.Base(savedPath)
				fileNote := "[DATEI ANGEHÄNGT]: " + agentPath
				if msg.Document.MimeType != "" {
					fileNote += " (" + msg.Document.MimeType + ")"
				}
				if inputText != "" {
					inputText += "\n\n" + fileNote
				} else {
					inputText = fileNote
				}
				logger.Info("Telegram document saved", "agent_path", agentPath)
			}
		}
	}

	if inputText == "" {
		return
	}
	if ctx.Err() != nil {
		return
	}

	if tools.HasPendingQuestion(sessionID) {
		if response, ok := tools.ResolveQuestionReply(sessionID, inputText); ok {
			response.Source = tools.QuestionSourceTelegram
			tools.CompleteQuestion(sessionID, response)
			return
		}
		deliverMessage(bot, msg.From.ID, "Please reply with one of the listed numbers.", logger)
		return
	}

	// A pending question belongs to the running turn that already owns this
	// gate. Deliver its reply above before serializing a new history turn.
	releaseTurn, err := shortTermMem.LockSessionTurn(ctx, sessionID)
	if err != nil {
		return
	}
	defer releaseTurn()

	// Phase: Command Interception
	// Check for slash commands
	if strings.HasPrefix(msg.Text, "/") {
		cmdCtx := commands.Context{
			STM:           shortTermMem,
			SessionID:     sessionID,
			HM:            historyManager,
			Vault:         vault,
			InventoryDB:   inventoryDB,
			Cfg:           cfg,
			PromptsDir:    cfg.Directories.PromptsDir,
			AllowOperator: telegramChatAllowsOperator(msg),
		}
		cmdResult, isCmd, err := commands.Handle(msg.Text, cmdCtx)
		if err != nil {
			logger.Error("Telegram command execution failed", "error", err)
			deliverMessage(bot, msg.From.ID, "⚠️ Fehler beim Ausführen des Befehls.", logger)
			return
		}
		if isCmd {
			deliverMessage(bot, msg.From.ID, cmdResult, logger)
			return
		}
	}

	if shouldBlockTelegramPromptInjection(inputText, guardian, logger, msg.From.ID) {
		deliverMessage(bot, msg.From.ID, "Your message was blocked by the security guardian.", logger)
		return
	}
	inputText = security.IsolateExternalData(inputText)
	if ctx.Err() != nil {
		return
	}

	// Authorized text found (either native or transcribed)
	manifest := tools.NewManifest(cfg.Directories.ToolsDir)

	// Add the message to history first
	mid, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, inputText, false, false)
	if sessionID == "default" && historyManager != nil && agent.ShouldAppendHistoryMessage(mid, err) {
		historyManager.Add(openai.ChatMessageRoleUser, inputText, mid, false, false)
	}

	// 1. Build RunConfig first so it can be used for prompt flag derivation
	var decisionService agent.TaskDecisionService
	if len(routingDecisions) > 0 {
		decisionService = routingDecisions[0]
	}
	runCfg := agent.RunConfig{
		RoutingDecisions:   decisionService,
		TaskRoutingText:    &routingText,
		HumanInput:         humanInput,
		Config:             cfg,
		Logger:             logger,
		LLMClient:          client,
		ShortTermMem:       shortTermMem,
		HistoryManager:     historyManager,
		LongTermMem:        longTermMem,
		KG:                 nil,
		InventoryDB:        inventoryDB,
		RemoteHub:          remoteHub,
		Vault:              vault,
		Registry:           registry,
		Manifest:           manifest,
		CronManager:        cronManager,
		MissionManagerV2:   missionManagerV2,
		WebhookManager:     webhookManager,
		CoAgentRegistry:    nil,
		BudgetTracker:      budgetTracker,
		PreparationService: nil,
		SessionID:          sessionID,
		MessageSource:      "telegram",
		VoiceOutputActive:  agent.GetVoiceMode(),
	}

	// 2. Assemble final messages for LLM
	finalMessages := buildTelegramAgentMessages(historyManager)
	if historyManager == nil {
		recent, err := shortTermMem.GetRecentMessages(sessionID, 60)
		if err != nil {
			logger.Error("Load Telegram conversation", "error", err)
			return
		}
		for _, message := range recent {
			if message.Role == openai.ChatMessageRoleUser || message.Role == openai.ChatMessageRoleAssistant {
				finalMessages = append(finalMessages, message)
			}
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: finalMessages,
	}

	// Start typing indicator
	typingCtx, stopTyping := context.WithCancel(ctx)
	defer stopTyping()
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			if typingCtx.Err() != nil {
				return
			}
			bot.Send(tgbotapi.NewChatAction(msg.From.ID, tgbotapi.ChatTyping))
			select {
			case <-ticker.C:
			case <-typingCtx.Done():
				return
			}
		}
	}()

	// Run the loop with the polling owner's cancellation context.

	// Use TelegramBroker to capture audio events for native sending
	broker := &TelegramBroker{bot: bot, chatID: msg.From.ID, logger: logger}
	resp, err := agent.ExecuteAgentLoop(ctx, req, runCfg, false, broker)
	stopTyping() // Stop the indicator as soon as the agent is done
	if ctx.Err() != nil {
		return
	}

	if err != nil {
		logger.Error("Telegram agent loop failed", "error", err)
		deliverMessage(bot, msg.From.ID, "⚠️ Sorry, I encountered an error processing your request.", logger)
		return
	}

	// Send captured audio files as native Telegram audio/voice messages
	for _, af := range broker.AudioFiles {
		if agent.GetVoiceMode() {
			// Voice mode: send as voice note (OGG/Opus) for inline playback
			oggPath, convErr := convertToOGGContext(ctx, af.FilePath, logger)
			if convErr == nil && oggPath != "" {
				defer os.Remove(oggPath)
			}
			if ctx.Err() != nil {
				return
			}
			if convErr != nil {
				logger.Warn("[Telegram] OGG conversion failed, falling back to audio", "error", convErr)
				if err := sendTelegramAudio(bot, msg.From.ID, af.FilePath, af.Title); err != nil {
					logger.Warn("[Telegram] Failed to send audio", "path", af.FilePath, "error", err)
				}
			} else {
				if err := sendTelegramVoice(bot, msg.From.ID, oggPath); err != nil {
					logger.Warn("[Telegram] Failed to send voice note", "path", oggPath, "error", err)
				}
			}
		} else {
			if err := sendTelegramAudio(bot, msg.From.ID, af.FilePath, af.Title); err != nil {
				logger.Warn("[Telegram] Failed to send audio", "path", af.FilePath, "error", err)
			}
		}
	}

	// Send result back to Telegram
	if len(resp.Choices) > 0 {
		answer := security.StripThinkingTags(resp.Choices[0].Message.Content)
		// Extract markdown images and send as native Telegram photos
		cleanText, images := media.ExtractMarkdownImages(answer)
		for _, img := range images {
			var localPath string
			staged := false
			if strings.HasPrefix(img.URL, "/files/") {
				// Local workspace file
				var err error
				localPath, err = media.StageWorkspaceImage(cfg.Directories.WorkspaceDir, img.URL)
				if err != nil {
					logger.Warn("Workspace image rejected", "error", err)
					continue
				}
				staged = true
			} else if strings.HasPrefix(img.URL, "http://") || strings.HasPrefix(img.URL, "https://") {
				// Remote URL: download and sanitize before sending
				imagesDir := filepath.Join(cfg.Directories.WorkspaceDir, "images")
				sanitized, err := media.DownloadAndSanitizeImageContext(ctx, img.URL, imagesDir)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					logger.Warn("[Telegram] Failed to download/sanitize image URL", "url", img.URL, "error", err)
					continue
				}
				localPath = sanitized
			} else {
				continue
			}
			if err := sendTelegramPhoto(bot, msg.From.ID, localPath, img.Caption); err != nil {
				logger.Warn("Failed to send Telegram photo", "path", localPath, "error", err)
			}
			if staged {
				_ = os.Remove(localPath)
			}
		}
		cleanText = AppendMissingYouTubeLinks(cleanText, broker.YouTubeVideos)
		if cleanText != "" {
			deliverMessage(bot, msg.From.ID, cleanText, logger)
		}
	}
}

// deliverMessage sends a message to the user and records a failed delivery. A
// lost answer is invisible to the user, so it is logged and counted in the
// runtime status.
func deliverMessage(bot *tgbotapi.BotAPI, chatID int64, text string, logger *slog.Logger) error {
	err := sendTelegramMessage(bot, chatID, text)
	if err != nil {
		integrationstatus.MarkTelegramDeliveryFailure(time.Now())
		if logger != nil {
			logger.Error("Failed to send Telegram message", "error", security.Scrub(err.Error()))
		}
	}
	return err
}

// telegramChatAllowsOperator reports whether operator slash commands may run:
// only in a private chat with the owner, never in groups or channels.
func telegramChatAllowsOperator(msg *tgbotapi.Message) bool {
	return msg != nil && msg.Chat != nil && msg.Chat.IsPrivate()
}

func shouldBlockTelegramPromptInjection(inputText string, guardian *security.Guardian, logger *slog.Logger, userID int64) bool {
	if guardian == nil {
		return false
	}
	scan := guardian.ScanForInjection(inputText)
	if scan.Level < security.ThreatHigh {
		return false
	}
	if logger != nil {
		logger.Warn("[Telegram] Prompt injection BLOCKED - message discarded",
			"user_id", userID, "level", scan.Level, "patterns", scan.Patterns)
	}
	return true
}

func sendTelegramMessage(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	text = security.Scrub(text)
	for _, part := range telegramTextChunks(text) {
		if _, err := bot.Send(tgbotapi.NewMessage(chatID, part)); err != nil {
			return err
		}
	}
	return nil
}

// AppendMissingYouTubeLinks appends captured YouTube links to text channels when
// the final assistant message did not already include the canonical link.
func AppendMissingYouTubeLinks(text string, videos []CapturedYouTubeVideo) string {
	result := strings.TrimSpace(text)
	for _, video := range videos {
		url := strings.TrimSpace(video.URL)
		if url == "" || strings.Contains(result, url) {
			continue
		}
		line := url
		if title := strings.TrimSpace(video.Title); title != "" {
			line = title + ": " + url
		}
		if result != "" {
			result += "\n\n"
		}
		result += line
	}
	return result
}

// sendTelegramPhoto sends a local image file as a native Telegram photo with optional caption.
func sendTelegramPhoto(bot *tgbotapi.BotAPI, chatID int64, localPath, caption string) error {
	return sendTelegramPhotoFile(bot, chatID, tgbotapi.FilePath(localPath), caption)
}

// sendTelegramPhotoFile sends file (a local path or in-memory bytes) as a native Telegram
// photo with optional caption. sendTelegramPhoto and SendPhoto share it.
func sendTelegramPhotoFile(bot *tgbotapi.BotAPI, chatID int64, file tgbotapi.RequestFileData, caption string) error {
	photo := tgbotapi.NewPhoto(chatID, file)
	photo.Caption = caption
	_, err := bot.Send(photo)
	return err
}

// sendTelegramAudio sends a local audio file as a native Telegram audio attachment.
func sendTelegramAudio(bot *tgbotapi.BotAPI, chatID int64, localPath, title string) error {
	audio := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(localPath))
	audio.Title = title
	_, err := bot.Send(audio)
	return err
}

// sendTelegramVoice sends a local OGG/Opus file as a Telegram voice note (inline playback).
func sendTelegramVoice(bot *tgbotapi.BotAPI, chatID int64, oggPath string) error {
	voice := tgbotapi.NewVoice(chatID, tgbotapi.FilePath(oggPath))
	_, err := bot.Send(voice)
	return err
}

func convertToOGGContext(ctx context.Context, inputPath string, logger *slog.Logger) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	outFile, err := os.CreateTemp("", "aura_telegram_voice_*.ogg")
	if err != nil {
		return "", fmt.Errorf("create temporary OGG output: %w", err)
	}
	outPath := outFile.Name()
	if err := outFile.Close(); err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("close temporary OGG output: %w", err)
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", inputPath, "-c:a", "libopus", "-b:a", "64k", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(outPath)
		if ctx.Err() != nil {
			return "", fmt.Errorf("ffmpeg conversion canceled: %w", ctx.Err())
		}
		logger.Debug("[Telegram] ffmpeg OGG conversion output", "output", string(out))
		return "", fmt.Errorf("ffmpeg conversion failed: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = os.Remove(outPath)
		return "", err
	}
	return outPath, nil
}

// CapturedAudio represents an audio file captured during the agent loop for native sending.
type CapturedAudio struct {
	FilePath string
	Title    string
	MimeType string
	Filename string
}

// CapturedYouTubeVideo represents a YouTube link captured during the agent loop
// for text-channel delivery.
type CapturedYouTubeVideo struct {
	URL     string
	Title   string
	VideoID string
}

// TelegramBroker implements agent.FeedbackProvider for Telegram
type TelegramBroker struct {
	bot           *tgbotapi.BotAPI
	chatID        int64
	logger        *slog.Logger
	AudioFiles    []CapturedAudio
	YouTubeVideos []CapturedYouTubeVideo
}

func (b *TelegramBroker) Send(event, message string) {
	if event == "question_user" {
		b.deliver(message)
		return
	}
	// Capture audio events for native sending after the loop
	if event == "audio" {
		var audio struct {
			FilePath string `json:"file_path"`
			Title    string `json:"title"`
			MimeType string `json:"mime_type"`
			Filename string `json:"filename"`
		}
		if json.Unmarshal([]byte(message), &audio) == nil && audio.FilePath != "" {
			b.AudioFiles = append(b.AudioFiles, CapturedAudio{
				FilePath: audio.FilePath,
				Title:    audio.Title,
				MimeType: audio.MimeType,
				Filename: audio.Filename,
			})
		}
		return
	}
	if event == "youtube_video" {
		var video struct {
			URL     string `json:"url"`
			Title   string `json:"title"`
			VideoID string `json:"video_id"`
		}
		if json.Unmarshal([]byte(message), &video) == nil && video.URL != "" {
			b.YouTubeVideos = append(b.YouTubeVideos, CapturedYouTubeVideo{
				URL:     video.URL,
				Title:   video.Title,
				VideoID: video.VideoID,
			})
		}
		return
	}
	// For now, we only send high-level events to avoid spamming the user
	if event == "tool_start" || event == "error_recovery" || event == "api_retry" || event == "progress" {
		b.logger.Info("[Telegram Status]", "event", event, "message", message)
		prefix := "⚙️ "
		if event == "progress" {
			prefix = "⏳ "
		}
		text := fmt.Sprintf("%s%s: %s", prefix, strings.ToUpper(event), message)
		if event == "progress" {
			text = fmt.Sprintf("⏳ %s", message) // Cleaner for progress
		}
		b.deliver(text)
	}
	if event == "budget_warning" {
		b.deliver("⚠️ " + message)
	}
	if event == "budget_blocked" {
		b.deliver("🚫 " + message)
	}
}

// deliver sends a message and records a failed delivery. A lost message is
// invisible to the user, so it is logged and counted in the runtime status.
func (b *TelegramBroker) deliver(text string) {
	if err := sendTelegramMessage(b.bot, b.chatID, text); err != nil {
		integrationstatus.MarkTelegramDeliveryFailure(time.Now())
		if b.logger != nil {
			b.logger.Error("[Telegram] Message could not be delivered", "error", security.Scrub(err.Error()))
		}
	}
}

func (b *TelegramBroker) SendJSON(jsonStr string) {
	// Usually for token usage etc. - skip for Telegram
}

func (b *TelegramBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
}

func (b *TelegramBroker) SendLLMStreamDone(finishReason string) {}

func (b *TelegramBroker) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
}

func (b *TelegramBroker) SendThinkingBlock(provider, content, state string) {
}

func downloadFile(ctx context.Context, url string, logger *slog.Logger) (string, error) {
	const maxTelegramDownloadBytes = 50 * 1024 * 1024
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create Telegram media download request: %s", security.Scrub(err.Error()))
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("download Telegram media: %s", security.Scrub(err.Error()))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file: %s", resp.Status)
	}
	if resp.ContentLength > maxTelegramDownloadBytes {
		return "", fmt.Errorf("telegram download exceeds max size: %d", resp.ContentLength)
	}

	tempFile, err := os.CreateTemp("", "aura_voice_*.ogg")
	if err != nil {
		return "", err
	}
	tempPath := tempFile.Name()
	keepTempFile := false
	defer func() {
		_ = tempFile.Close()
		if !keepTempFile {
			_ = os.Remove(tempPath)
		}
	}()

	// Read one byte beyond the limit so an oversized transfer fails loudly
	// instead of being truncated silently.
	written, err := io.Copy(tempFile, io.LimitReader(resp.Body, maxTelegramDownloadBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	if written > maxTelegramDownloadBytes {
		return "", fmt.Errorf("telegram download exceeds max size: %d bytes", maxTelegramDownloadBytes)
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	keepTempFile = true
	return tempPath, nil
}
