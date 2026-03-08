package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/commands"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/media"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sashabaranov/go-openai"
)

// StartLongPolling initializes the Telegram bot in Long Polling mode.
// It runs in a background goroutine and processes incoming messages.
func StartLongPolling(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManager *tools.MissionManager) {
	if cfg.Telegram.BotToken == "" {
		logger.Warn("Telegram Bot Token is missing, skipping Long Polling start.")
		return
	}

	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		logger.Error("Failed to initialize Telegram bot", "error", err)
		return
	}

	// [MANDATORY] Clear any existing Webhook
	_, err = bot.Request(tgbotapi.DeleteWebhookConfig{})
	if err != nil {
		logger.Error("Failed to clear Telegram webhook", "error", err)
	} else {
		logger.Info("Telegram webhook cleared successfully.")
	}

	logger.Info("Telegram Bot started in Long Polling mode", "user", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// Worker pool to limit concurrent message processing
	maxWorkers := cfg.Telegram.MaxConcurrentWorkers
	if maxWorkers <= 0 {
		maxWorkers = 5
	}
	workerSem := make(chan struct{}, maxWorkers)

	go func() {
		for update := range updates {
			if update.Message == nil {
				continue
			}

			senderID := update.Message.From.ID

			// [Silent ID Discovery Mode]
			if cfg.Telegram.UserID == 0 {
				fmt.Printf("\n[SECURITY] Incoming message from unauthorized ID: %d. Add this to config.yaml under telegram_user_id to authorize.\n\n", senderID)
				logger.Warn("Unauthorized Telegram ID discovered", "id", senderID, "username", update.Message.From.UserName)
				continue
			}

			// [Authorization Check]
			if senderID != cfg.Telegram.UserID {
				logger.Warn("Blocked unauthorized Telegram message", "id", senderID)
				continue
			}

			// Acquire worker slot (blocks if all slots are busy)
			workerSem <- struct{}{}
			go func(upd tgbotapi.Update) {
				defer func() { <-workerSem }()
				processUpdate(bot, upd, cfg, logger, client, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB, missionManager)
			}(update)
		}
	}()
}

func processUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManager *tools.MissionManager) {
	// Maintenance check: Inform the user but allow the tool-based interaction
	inMaintenance := tools.IsBusy()
	if inMaintenance {
		logger.Info("Telegram processing in Maintenance Mode")
	}
	msg := update.Message
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

		oggURL := file.Link(cfg.Telegram.BotToken)

		// 2. Download the .ogg file (we can reuse the logic but needs adjustment)
		oggPath, err := downloadFile(oggURL, logger)
		if err != nil {
			logger.Error("Failed to download voice file", "error", err)
			return
		}
		defer os.Remove(oggPath)

		// 3. Convert to .mp3 (better for multimodal APIs)
		mp3Path := oggPath + ".mp3"
		if err := ConvertOggToMp3(oggPath, mp3Path); err != nil {
			logger.Error("Failed to convert voice file to mp3", "error", err)
			return
		}
		defer os.Remove(mp3Path)

		// 4. Transcribe via Multimodal OpenRouter API
		text, err := TranscribeMultimodal(mp3Path, cfg)
		if err != nil {
			logger.Error("Failed to transcribe voice (multimodal)", "error", err)
			return
		}

		logger.Info("Multimodal transcription successful", "text", text)
		inputText = text
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
			logger.Error("Failed to get photo file info", "error", err)
		} else {
			imgURL := file.Link(cfg.Telegram.BotToken)

			// 2. Download the file
			imgPath, err := downloadFile(imgURL, logger)
			if err != nil {
				logger.Error("Failed to download photo file", "error", err)
			} else {
				defer os.Remove(imgPath)

				// 3. Analyze via Vision API
				analysis, err := AnalyzeImage(imgPath, cfg)
				if err != nil {
					logger.Error("Failed to analyze image", "error", err)
					analysis = "[Error analyzing image]"
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
			logger.Error("Failed to get document file info", "error", err)
		} else {
			docURL := file.Link(cfg.Telegram.BotToken)
			attachDir := filepath.Join(cfg.Directories.WorkspaceDir, "attachments")
			savedPath, err := media.SaveAttachment(docURL, msg.Document.FileName, attachDir)
			if err != nil {
				logger.Error("Failed to save Telegram document", "error", err)
			} else {
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

	// Phase: Command Interception
	// Check for slash commands
	if strings.HasPrefix(msg.Text, "/") {
		cmdCtx := commands.Context{
			STM:         shortTermMem,
			HM:          historyManager,
			Vault:       vault,
			InventoryDB: inventoryDB,
			Cfg:         cfg,
			PromptsDir:  cfg.Directories.PromptsDir,
		}
		cmdResult, isCmd, err := commands.Handle(msg.Text, cmdCtx)
		if err != nil {
			logger.Error("Telegram command execution failed", "error", err)
			sendTelegramMessage(bot, msg.From.ID, "⚠️ Fehler beim Ausführen des Befehls.")
			return
		}
		if isCmd {
			if err := sendTelegramMessage(bot, msg.From.ID, cmdResult); err != nil {
				logger.Error("Failed to send Telegram command result", "error", err)
			}
			return
		}
	}

	// Authorized text found (either native or transcribed)
	manifest := tools.NewManifest(cfg.Directories.ToolsDir)
	sessionID := "default"

	// Add the message to history first
	mid, _ := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, inputText, false, false)
	if sessionID == "default" {
		historyManager.Add(openai.ChatMessageRoleUser, inputText, mid, false, false)
	}

	// 1. Build context flags for the prompt builder
	flags := prompts.ContextFlags{
		ActiveProcesses:        agent.GetActiveProcessStatus(registry),
		IsMaintenanceMode:      tools.IsBusy(),
		LifeboatEnabled:        cfg.Maintenance.LifeboatEnabled,
		DiscordEnabled:         cfg.Discord.Enabled,
		EmailEnabled:           cfg.Email.Enabled,
		DockerEnabled:          cfg.Docker.Enabled,
		HomeAssistantEnabled:   cfg.HomeAssistant.Enabled,
		WebDAVEnabled:          cfg.WebDAV.Enabled,
		KoofrEnabled:           cfg.Koofr.Enabled,
		ChromecastEnabled:      cfg.Chromecast.Enabled,
		CoAgentEnabled:         cfg.CoAgents.Enabled,
		GoogleWorkspaceEnabled: cfg.Agent.EnableGoogleWorkspace,
		ProxmoxEnabled:         cfg.Proxmox.Enabled,
		OllamaEnabled:          cfg.Ollama.Enabled,
		IsEgg:                  cfg.EggMode.Enabled,
	}
	coreMem := shortTermMem.ReadCoreMemory()

	sysPrompt := prompts.BuildSystemPrompt(cfg.Directories.PromptsDir, flags, coreMem, logger)

	// 2. Assemble final messages for LLM
	finalMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
	}

	currentSummary := historyManager.GetSummary()
	if currentSummary != "" {
		finalMessages = append(finalMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: "[CONTEXT_RECAP]: The following is a summary of previous relevant discussions for context. DO NOT echo or repeat this recap in your response:\n" + currentSummary,
		})
	}
	finalMessages = append(finalMessages, historyManager.Get()...)

	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: finalMessages,
	}

	// Start typing indicator
	typingCtx, stopTyping := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			bot.Send(tgbotapi.NewChatAction(msg.From.ID, tgbotapi.ChatTyping))
			select {
			case <-ticker.C:
			case <-typingCtx.Done():
				return
			}
		}
	}()

	// Run the loop
	ctx := context.Background()
	runCfg := agent.RunConfig{
		Config:          cfg,
		Logger:          logger,
		LLMClient:       client, // Changed from llmClient to client based on context
		ShortTermMem:    shortTermMem,
		HistoryManager:  historyManager,
		LongTermMem:     longTermMem,
		KG:              nil, // Telegram bot doesn't currently wire KG by default
		InventoryDB:     inventoryDB,
		Vault:           vault,
		Registry:        registry,
		Manifest:        manifest,
		CronManager:     cronManager,
		MissionManager:  missionManager,
		CoAgentRegistry: nil,
		BudgetTracker:   nil, // Assuming budgetTracker is not available or needs to be initialized
		SessionID:       sessionID,
		IsMaintenance:   tools.IsBusy(), // Changed from false to tools.IsBusy() based on original RunSyncAgentLoop
		SurgeryPlan:     "",
	}

	// Use a NoopBroker for telegram, as we don't stream UI events back
	broker := &agent.NoopBroker{}
	resp, err := agent.ExecuteAgentLoop(ctx, req, runCfg, false, broker)
	stopTyping() // Stop the indicator as soon as the agent is done

	if err != nil {
		logger.Error("Telegram agent loop failed", "error", err)
		sendTelegramMessage(bot, msg.From.ID, "⚠️ Sorry, I encountered an error processing your request.")
		return
	}

	// Send result back to Telegram
	if len(resp.Choices) > 0 {
		answer := resp.Choices[0].Message.Content
		// Extract markdown images and send as native Telegram photos
		cleanText, images := media.ExtractMarkdownImages(answer)
		for _, img := range images {
			var localPath string
			if strings.HasPrefix(img.URL, "/files/") {
				// Local workspace file
				localPath = filepath.Join(cfg.Directories.WorkspaceDir, strings.TrimPrefix(img.URL, "/files/"))
			} else if strings.HasPrefix(img.URL, "http://") || strings.HasPrefix(img.URL, "https://") {
				// Remote URL: download and sanitize before sending
				imagesDir := filepath.Join(cfg.Directories.WorkspaceDir, "images")
				sanitized, err := media.DownloadAndSanitizeImage(img.URL, imagesDir)
				if err != nil {
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
		}
		if cleanText != "" {
			if err := sendTelegramMessage(bot, msg.From.ID, cleanText); err != nil {
				logger.Error("Failed to send Telegram message", "error", err)
			}
		}
	}
}

func sendTelegramMessage(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := bot.Send(msg)
	return err
}

// sendTelegramPhoto sends a local image file as a native Telegram photo with optional caption.
func sendTelegramPhoto(bot *tgbotapi.BotAPI, chatID int64, localPath, caption string) error {
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(localPath))
	photo.Caption = caption
	_, err := bot.Send(photo)
	return err
}

// TelegramBroker implements agent.FeedbackProvider for Telegram
type TelegramBroker struct {
	bot    *tgbotapi.BotAPI
	chatID int64
	logger *slog.Logger
}

func (b TelegramBroker) Send(event, message string) {
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
		sendTelegramMessage(b.bot, b.chatID, text)
	}
	if event == "budget_warning" {
		sendTelegramMessage(b.bot, b.chatID, "⚠️ "+message)
	}
	if event == "budget_blocked" {
		sendTelegramMessage(b.bot, b.chatID, "🚫 "+message)
	}
}

func (b TelegramBroker) SendJSON(jsonStr string) {
	// Usually for token usage etc. - skip for Telegram
}

func downloadFile(url string, logger *slog.Logger) (string, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file: %s", resp.Status)
	}

	tempFile, err := os.CreateTemp("", "aura_voice_*.ogg")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}
