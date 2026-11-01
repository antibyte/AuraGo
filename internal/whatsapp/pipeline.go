package whatsapp

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/commands"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/media"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/telegram"
	"aurago/internal/tools"
	"aurago/internal/webhooks"

	"github.com/sashabaranov/go-openai"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// AgentDeps are the collaborators the pipeline hands to agent.ExecuteAgentLoop, the same set
// telegram.StartLongPolling receives (internal/telegram/bot.go). The knowledge graph is left
// out: Telegram passes it but runs the agent without it.
type AgentDeps struct {
	LLMClient        llm.ChatClient
	ShortTermMem     *memory.SQLiteMemory
	LongTermMem      memory.VectorDB
	Vault            *security.Vault
	Registry         *tools.ProcessRegistry
	CronManager      *tools.CronManager
	HistoryManager   *memory.HistoryManager
	InventoryDB      *sql.DB
	MissionManagerV2 *tools.MissionManagerV2
	RemoteHub        *remote.RemoteHub
	Guardian         *security.Guardian
	BudgetTracker    func() *budget.Tracker
	WebhookManager   *webhooks.Manager
	Snapshot         func() (*config.Config, llm.ChatClient) // current config and client per message
	RoutingDecisions agent.TaskDecisionService
}

// agentLoopFunc is agent.ExecuteAgentLoop; tests replace it.
type agentLoopFunc func(ctx context.Context, req openai.ChatCompletionRequest, runCfg agent.RunConfig, stream bool, broker agent.FeedbackBroker) (openai.ChatCompletionResponse, error)

// typingRefresh is the gap between two "typing" updates; WhatsApp shows one for about ten seconds.
const typingRefresh = 8 * time.Second

// Notices the owner can get instead of an answer. The status prefixes and wording are
// Telegram's (internal/telegram/bot.go).
const (
	replyPromptText   = "Please reply with one of the listed numbers."
	commandErrorText  = "⚠️ Fehler beim Ausführen des Befehls."
	guardianBlockText = "Your message was blocked by the security guardian."
	agentErrorText    = "⚠️ Sorry, I encountered an error processing your request."
)

// pipeline hands accepted owner messages to the agent, the way telegram.processUpdate does,
// for text. Every reply goes through the sender to in.Owner, an entry of ownerRecipients;
// the chat the message came from is only used for receipts and typing.
type pipeline struct {
	base        *config.Config // the generation's config clone
	deps        AgentDeps
	sender      *sender
	client      func() waClient
	agentLoop   agentLoopFunc // nil: agent.ExecuteAgentLoop
	now         func() time.Time
	typingEvery time.Duration
	logger      *slog.Logger
}

// handle runs one accepted owner message to its reply. It logs the owner's masked JID and
// message kinds, never the text, and returns without an answer when the generation is
// being replaced.
func (p *pipeline) handle(ctx context.Context, in inboundMessage) {
	logger, now, agentLoop, typingEvery := p.logger, p.now, p.agentLoop, p.typingEvery
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	if agentLoop == nil {
		agentLoop = agent.ExecuteAgentLoop
	}
	if typingEvery <= 0 {
		typingEvery = typingRefresh
	}

	// 1. The generation's config and the current LLM client. A changed WhatsApp section
	// means a newer generation is taking over.
	cfg, llmClient := p.base, p.deps.LLMClient
	if p.deps.Snapshot != nil {
		cfg, llmClient = p.deps.Snapshot()
	}
	if cfg == nil || p.base == nil || cfg.EggMode.Enabled || !cfg.WhatsApp.Equal(p.base.WhatsApp) || ctx.Err() != nil {
		return
	}
	stm := p.deps.ShortTermMem
	if stm == nil {
		logger.Error("WhatsApp message dropped: no short-term memory configured")
		return
	}
	var wa waClient
	if p.client != nil {
		wa = p.client()
	}

	// 2. Read receipt. A failed one loses no content, so it is only logged.
	if wa != nil && cfg.WhatsApp.Mode == config.WhatsAppModeDedicated && cfg.WhatsApp.SendReadReceipts {
		if err := wa.MarkRead(ctx, []types.MessageID{in.ID}, now(), in.Chat, in.Sender); err != nil && ctx.Err() == nil {
			logger.Warn("WhatsApp read receipt failed", "error", maskedError(err))
		}
	}

	// 3. Budget and session.
	var budgetTracker *budget.Tracker
	if p.deps.BudgetTracker != nil {
		budgetTracker = p.deps.BudgetTracker()
	}
	ctx = budget.WithTracking(ctx, budgetTracker, "chat")
	if tools.IsBusy() {
		logger.Info("WhatsApp processing in Maintenance Mode")
	}
	if ctx.Err() != nil {
		return
	}
	sessionID, err := stm.ResolveUserSession(ctx, "default")
	if err != nil {
		logger.Error("Resolve WhatsApp conversation", "error", maskedError(err))
		return
	}
	ctx, releaseSession, err := stm.RegisterSessionRun(ctx, sessionID)
	if err != nil {
		return
	}
	defer releaseSession()
	historyManager := p.deps.HistoryManager
	if sessionID != "default" {
		historyManager = nil
	}

	// 4. Text. Only the top-level text of the unwrapped message counts: a media message, or
	// a wrapper that hides other content, starts no turn.
	text := messageText(in.Message)
	if text == "" {
		logger.Info("WhatsApp message kind not handled yet", "kind", messageKind(in.Message))
		return
	}

	// 5.
	logger.Info("Received authorized WhatsApp message", "sender", maskJID(in.Owner))

	// 6. The platform message ID is hashed into the evidence turn ID, so the number is not
	// stored in clear.
	inputText := text
	routingText := inputText
	humanInput := memory.NewHumanInputEvent(inputText, sessionID, "whatsapp:"+in.Chat.User+":"+string(in.ID), in.Timestamp)

	// 7. A forwarded message arrives from the owner's account, but a third party wrote it.
	// The fixed marker goes inside the isolated block (and before the injection scan), task
	// routing reads the marked text, the text no longer counts as the owner's own words,
	// and it can neither answer a pending question nor run a slash command.
	forwardMarker := ""
	if ci := in.Message.GetExtendedTextMessage().GetContextInfo(); ci.GetIsForwarded() || ci.GetForwardingScore() > 0 {
		forwardMarker = telegram.ForwardedContentMarker(ci.GetForwardedNewsletterMessageInfo().GetNewsletterName())
		inputText = forwardMarker + "\n" + inputText
		routingText = forwardedRoutingText(forwardMarker, routingText)
		humanInput = nil
	}

	// 8. A pending question belongs to the running turn that already owns the turn gate:
	// route its reply before serializing a new turn.
	if tools.HasPendingQuestion(sessionID) {
		if forwardMarker == "" {
			if response, ok := tools.ResolveQuestionReply(sessionID, inputText); ok {
				response.Source = tools.QuestionSourceWhatsApp
				tools.CompleteQuestion(sessionID, response)
				return
			}
		}
		p.deliver(ctx, in.Owner, replyPromptText, "reply prompt")
		return
	}

	// 9.
	releaseTurn, err := stm.LockSessionTurn(ctx, sessionID)
	if err != nil {
		return
	}
	defer releaseTurn()

	// 10. Slash commands. The filter lets only the owner's direct chat through, so operator
	// commands are allowed.
	if forwardMarker == "" && strings.HasPrefix(text, "/") {
		cmdResult, isCmd, err := commands.Handle(text, commands.Context{
			STM:           stm,
			SessionID:     sessionID,
			HM:            historyManager,
			Vault:         p.deps.Vault,
			InventoryDB:   p.deps.InventoryDB,
			Cfg:           cfg,
			PromptsDir:    cfg.Directories.PromptsDir,
			AllowOperator: true,
		})
		if err != nil {
			logger.Error("WhatsApp command execution failed", "error", maskedError(err))
			p.deliver(ctx, in.Owner, commandErrorText, "command error notice")
			return
		}
		if isCmd {
			p.deliver(ctx, in.Owner, cmdResult, "command result")
			return
		}
	}

	// 11.
	if shouldBlockWhatsAppPromptInjection(inputText, p.deps.Guardian, logger, in.Owner) {
		p.deliver(ctx, in.Owner, guardianBlockText, "security block notice")
		return
	}
	inputText = security.IsolateExternalData(inputText)
	if ctx.Err() != nil {
		return
	}

	// 12. History, run configuration and the model's messages.
	mid, err := stm.InsertMessage(sessionID, openai.ChatMessageRoleUser, inputText, false, false)
	if sessionID == "default" && historyManager != nil && agent.ShouldAppendHistoryMessage(mid, err) {
		historyManager.Add(openai.ChatMessageRoleUser, inputText, mid, false, false)
	}
	runCfg := agent.RunConfig{
		RoutingDecisions:  p.deps.RoutingDecisions,
		DecisionConsumers: agent.ConsumersFrom(p.deps.RoutingDecisions),
		TaskRoutingText:   &routingText,
		HumanInput:        humanInput,
		Config:            cfg,
		Logger:            logger,
		LLMClient:         llmClient,
		ShortTermMem:      stm,
		HistoryManager:    historyManager,
		LongTermMem:       p.deps.LongTermMem,
		KG:                nil,
		InventoryDB:       p.deps.InventoryDB,
		RemoteHub:         p.deps.RemoteHub,
		Vault:             p.deps.Vault,
		Registry:          p.deps.Registry,
		Manifest:          tools.NewManifest(cfg.Directories.ToolsDir),
		CronManager:       p.deps.CronManager,
		MissionManagerV2:  p.deps.MissionManagerV2,
		WebhookManager:    p.deps.WebhookManager,
		BudgetTracker:     budgetTracker,
		SessionID:         sessionID,
		MessageSource:     "whatsapp",
		VoiceOutputActive: agent.GetVoiceMode(),
	}
	finalMessages := buildAgentMessages(historyManager)
	if historyManager == nil {
		recent, err := stm.GetRecentMessages(sessionID, 60)
		if err != nil {
			logger.Error("Load WhatsApp conversation", "error", maskedError(err))
			return
		}
		for _, message := range recent {
			if message.Role == openai.ChatMessageRoleUser || message.Role == openai.ChatMessageRoleAssistant {
				finalMessages = append(finalMessages, message)
			}
		}
	}
	req := openai.ChatCompletionRequest{Model: cfg.LLM.Model, Messages: finalMessages}

	// 13. and 14. Typing while the agent works, then the agent itself.
	stopTyping := startTyping(ctx, wa, in.Chat, typingEvery)
	defer stopTyping()
	broker := &whatsAppBroker{ctx: ctx, sender: p.sender, to: in.Owner, now: now}
	resp, err := agentLoop(ctx, req, runCfg, false, broker)
	stopTyping()
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		logger.Error("WhatsApp agent loop failed", "error", maskedError(err))
		p.deliver(ctx, in.Owner, agentErrorText, "error notice")
		return
	}

	// 15. The answer. Images are not sent as pictures yet (stage 3): each becomes a line.
	if len(resp.Choices) > 0 {
		answer := security.StripThinkingTags(resp.Choices[0].Message.Content)
		cleanText, images := media.ExtractMarkdownImages(answer)
		cleanText = appendImageLines(cleanText, images)
		if cleanText != "" {
			p.deliver(ctx, in.Owner, cleanText, "answer")
		}
	}
}

// deliver hands text to the sender, which refuses non-owners and counts and logs a failed
// delivery itself.
func (p *pipeline) deliver(ctx context.Context, to types.JID, text, what string) {
	_ = p.sender.deliverText(ctx, to, text, what)
}

// maskedError is the only form in which an error reaches a log line.
func maskedError(err error) string {
	return maskDigits(security.Scrub(err.Error()))
}

// messageText returns the text of a plain or extended text message, and "" for everything
// else. It reads the top level of the already unwrapped message only.
func messageText(m *waE2E.Message) string {
	if text := m.GetConversation(); text != "" {
		return text
	}
	return m.GetExtendedTextMessage().GetText()
}

// messageKind names the content of a message without text, for the "not handled yet" line.
func messageKind(m *waE2E.Message) string {
	switch {
	case m.GetImageMessage() != nil:
		return "image"
	case m.GetAudioMessage() != nil:
		return "audio"
	case m.GetVideoMessage() != nil:
		return "video"
	case m.GetDocumentMessage() != nil:
		return "document"
	case m.GetStickerMessage() != nil:
		return "sticker"
	case m.GetLocationMessage() != nil || m.GetLiveLocationMessage() != nil:
		return "location"
	case m.GetContactMessage() != nil || m.GetContactsArrayMessage() != nil:
		return "contact"
	}
	return "other"
}

// forwardedRoutingText heads the pinned routing text of a forwarded message with the marker
// the model reads, so the task classifier does not take the third party's words for the
// owner's request. An empty routing text stays empty. It copies telegramForwardedRoutingText.
func forwardedRoutingText(marker, routingText string) string {
	if strings.TrimSpace(routingText) == "" {
		return routingText
	}
	return marker + "\n" + routingText
}

// shouldBlockWhatsAppPromptInjection is shouldBlockTelegramPromptInjection with a masked JID.
func shouldBlockWhatsAppPromptInjection(inputText string, guardian *security.Guardian, logger *slog.Logger, owner types.JID) bool {
	if guardian == nil {
		return false
	}
	scan := guardian.ScanForInjection(inputText)
	if scan.Level < security.ThreatHigh {
		return false
	}
	if logger != nil {
		logger.Warn("[WhatsApp] Prompt injection BLOCKED - message discarded",
			"sender", maskJID(owner), "level", scan.Level, "patterns", scan.Patterns)
	}
	return true
}

// buildAgentMessages is buildTelegramAgentMessages: the system placeholder, the context
// recap when there is one, then the history.
func buildAgentMessages(historyManager *memory.HistoryManager) []openai.ChatCompletionMessage {
	finalMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem},
	}
	if historyManager == nil {
		return finalMessages
	}
	if summary := historyManager.GetSummary(); summary != "" {
		finalMessages = append(finalMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: agent.FormatContextRecapForPrompt(summary),
		})
	}
	return append(finalMessages, historyManager.Get()...)
}

// appendImageLines appends one "[Image: caption]" line per Markdown image to text. The
// caption is the model's alt text, folded onto one line.
func appendImageLines(text string, images []media.ImageRef) string {
	for _, img := range images {
		caption := strings.Join(strings.Fields(img.Caption), " ")
		if caption == "" {
			caption = "image"
		}
		line := "[Image: " + caption + "]"
		if text != "" {
			text += "\n"
		}
		text += line
	}
	return text
}

// startTyping shows "typing" in chat and refreshes it every `every` until the returned stop
// runs. stop waits for the refresh goroutine and then sends "paused" once, unless ctx is
// done. A failed update is ignored: it loses nothing. stop may run more than once.
func startTyping(ctx context.Context, wa waClient, chat types.JID, every time.Duration) (stop func()) {
	if wa == nil {
		return func() {}
	}
	typingCtx, cancel := context.WithCancel(ctx)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for typingCtx.Err() == nil {
			_ = wa.SendChatPresence(typingCtx, chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
			select {
			case <-ticker.C:
			case <-typingCtx.Done():
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-finished
			if ctx.Err() == nil {
				_ = wa.SendChatPresence(ctx, chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
			}
		})
	}
}
