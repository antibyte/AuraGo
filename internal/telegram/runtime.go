package telegram

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/security"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// UpdateOffsets survives bot generations, including token removal and rotation.
// Keys come from getMe, never from an unverified token prefix.
type UpdateOffsets struct {
	mu    sync.Mutex
	byBot map[int64]int
	store OffsetStore
}

// SetStore attaches a persistent cursor store. The in-memory cache stays the
// fast path; the store only adds durability across restarts.
func (o *UpdateOffsets) SetStore(store OffsetStore) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.store = store
}

// Store returns the attached cursor store, if any.
func (o *UpdateOffsets) Store() OffsetStore {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.store
}

// Load returns the cursor for botID. The in-memory cache wins; a stored
// cursor is read once and cached afterwards.
func (o *UpdateOffsets) Load(botID int64) (int, bool) {
	if o == nil {
		return 0, false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if offset, ok := o.byBot[botID]; ok {
		return offset, true
	}
	if o.store == nil {
		return 0, false
	}
	offset, ok := o.store.Load(botID)
	if ok {
		if o.byBot == nil {
			o.byBot = make(map[int64]int)
		}
		o.byBot[botID] = offset
	}
	return offset, ok
}

// Save stores the cursor for botID in the cache and, when a store is
// attached, persists it.
func (o *UpdateOffsets) Save(botID int64, offset int) error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.byBot == nil {
		o.byBot = make(map[int64]int)
	}
	o.byBot[botID] = offset
	if o.store == nil {
		return nil
	}
	return o.store.Save(botID, offset)
}

func (o *UpdateOffsets) get(botID int64) int {
	offset, _ := o.Load(botID)
	return offset
}

type Bot struct {
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	initial *config.Config
}

func (b *Bot) MatchesConfig(cfg *config.Config) bool {
	return b != nil && b.ctx.Err() == nil && cfg != nil && !cfg.EggMode.Enabled && b.initial.Telegram == cfg.Telegram
}

// CancelIfConfigChanged revokes credentials and grants at config publication;
// Stop drains the old generation after the server releases its config lock.
func (b *Bot) CancelIfConfigChanged(cfg *config.Config) {
	if b != nil && !b.MatchesConfig(cfg) {
		b.cancel()
	}
}

func (b *Bot) Stop() {
	if b != nil {
		b.cancel()
		<-b.done
	}
}

func startLongPolling(parent context.Context, cfg *config.Config, logger *slog.Logger, plannerDB *sql.DB, offsets *UpdateOffsets, endpoint string, client tgbotapi.HTTPClient, handle func(context.Context, *tgbotapi.BotAPI, tgbotapi.Update)) *Bot {
	if cfg == nil || cfg.EggMode.Enabled || cfg.Telegram.BotToken == "" {
		integrationstatus.SetTelegramConfigured(false, false)
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}
	if parent.Err() != nil {
		return nil
	}
	if offsets == nil {
		offsets = &UpdateOffsets{}
	}
	cfg = cfg.Clone()
	attachTelegramOffsetStore(offsets, cfg, logger)
	security.RegisterSensitive(cfg.Telegram.BotToken)
	integrationstatus.SetTelegramConfigured(true, cfg.Telegram.UserID != 0)
	ctx, cancel := context.WithCancel(parent)
	b := &Bot{ctx: ctx, cancel: cancel, done: make(chan struct{}), initial: cfg}
	go func() {
		defer close(b.done)
		bot, err := tgbotapi.NewBotAPIWithClient(cfg.Telegram.BotToken, endpoint, contextBoundHTTPClient{parent: ctx, client: client})
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			integrationstatus.MarkTelegramUnavailable("telegram_init_failed", time.Now())
			logger.Error("Failed to initialize Telegram bot", "error", err)
			return
		}
		_, err = bot.Request(tgbotapi.DeleteWebhookConfig{})
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			logger.Error("Failed to clear Telegram webhook", "error", err)
		}
		logger.Info("Telegram Bot started in Long Polling mode", "user", bot.Self.UserName)
		maxWorkers := cfg.Telegram.MaxConcurrentWorkers
		if maxWorkers <= 0 {
			maxWorkers = 5
		}
		workerSem := make(chan struct{}, maxWorkers)
		var workers sync.WaitGroup
		defer workers.Wait()
		u := tgbotapi.NewUpdate(offsets.get(bot.Self.ID))
		u.Timeout = 60
		integrationstatus.MarkTelegramPolling()
		offset := runPollingLoop(ctx, bot, u, plannerDB, logger, func(update tgbotapi.Update) bool {
			if update.Message == nil || update.Message.From == nil {
				return true
			}
			senderID := update.Message.From.ID
			if cfg.Telegram.UserID == 0 {
				logger.Warn("Unauthorized Telegram ID discovered", "id", senderID, "username", update.Message.From.UserName)
				return true
			}
			if senderID != cfg.Telegram.UserID {
				logger.Warn("Blocked unauthorized Telegram message", "id", senderID)
				return true
			}
			if !acquireTelegramWorker(ctx, workerSem) {
				return false
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer func() { <-workerSem }()
				handle(ctx, bot, update)
			}()
			return true
		}, func(offset int) {
			persistTelegramOffset(offsets, bot.Self.ID, offset, logger)
		})
		persistTelegramOffset(offsets, bot.Self.ID, offset, logger)
	}()
	return b
}

// persistTelegramOffset writes the cursor to the cache and, when a store is
// attached, to disk. A failing store must never stop the poller.
func persistTelegramOffset(offsets *UpdateOffsets, botID int64, offset int, logger *slog.Logger) {
	if offsets == nil {
		return
	}
	if err := offsets.Save(botID, offset); err != nil && logger != nil {
		logger.Warn("Failed to persist Telegram cursor", "error", security.Scrub(err.Error()))
	}
}

// attachTelegramOffsetStore binds a persistent cursor store when the data
// directory is configured. An empty data directory keeps the in-memory cursor,
// and a failing store only degrades durability.
func attachTelegramOffsetStore(offsets *UpdateOffsets, cfg *config.Config, logger *slog.Logger) {
	if offsets == nil || offsets.Store() != nil || cfg == nil {
		return
	}
	dataDir := strings.TrimSpace(cfg.Directories.DataDir)
	if dataDir == "" {
		return
	}
	store, err := NewFileOffsetStore(dataDir)
	if err != nil {
		if logger != nil {
			logger.Warn("Telegram cursor persistence disabled", "error", security.Scrub(err.Error()))
		}
		return
	}
	if store == nil {
		return
	}
	store.SetLogger(logger)
	offsets.SetStore(store)
}
