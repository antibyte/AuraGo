package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/security"

	"go.mau.fi/whatsmeow"
)

// Link methods.
const (
	LinkMethodQR    = "qr"
	LinkMethodPhone = "phone"
)

// Link session states. idle means this process has no session.
const (
	LinkStateIdle    = "idle"
	LinkStateWaiting = "waiting"
	LinkStateQR      = "qr"
	LinkStateCode    = "code"
	LinkStateSuccess = "success"
	LinkStateTimeout = "timeout"
	LinkStateError   = "error"
)

const linkTTL = 3 * time.Minute

// LinkRequest starts a link session, either by QR code or by phone pairing
// code.
type LinkRequest struct {
	Method string `json:"method"`
	Phone  string `json:"phone,omitempty"`
}

// Validate checks the method and, for phone linking, the number.
func (r LinkRequest) Validate() error {
	switch strings.ToLower(strings.TrimSpace(r.Method)) {
	case LinkMethodQR:
		return nil
	case LinkMethodPhone:
		if _, err := config.NormalizeE164(r.Phone); err != nil {
			return ErrInvalidPhone
		}
		return nil
	}
	return ErrInvalidLinkMethod
}

// LinkSnapshot is what GET /api/whatsapp/link returns. Error is a fixed code.
type LinkSnapshot struct {
	State       string `json:"state"`
	QRPNG       string `json:"qr_png,omitempty"`
	PairingCode string `json:"pairing_code,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	Error       string `json:"error,omitempty"`
}

type linkDeps struct {
	backend backend
	now     func() time.Time
	render  func(ctx context.Context, payload string) (string, error)
}

func defaultLinkDeps(logger *slog.Logger) linkDeps {
	return linkDeps{backend: sqliteBackend{log: newWALogger(logger, "whatsmeow")}, now: time.Now, render: renderQRDataURI}
}

// LinkSession owns a temporary client on an unlinked store for at most
// linkTTL. On success it disconnects and closes the store before onFinish
// runs, so the next Bot is the only user of the new device.
type LinkSession struct {
	method string
	phone  string
	cancel context.CancelFunc
	done   chan struct{}

	mu       sync.Mutex
	snap     LinkSnapshot
	finished bool
}

// StartLink opens the store, refuses an already linked device, and links in
// the background. onFinish(linked) runs once, on the session goroutine, after
// cleanup. It must not block.
func StartLink(parent context.Context, cfg *config.Config, req LinkRequest, logger *slog.Logger, onFinish func(linked bool)) (*LinkSession, error) {
	if logger == nil {
		logger = slog.Default()
	}
	return startLink(parent, cfg, req, logger, onFinish, defaultLinkDeps(logger))
}

func startLink(parent context.Context, cfg *config.Config, req LinkRequest, logger *slog.Logger, onFinish func(bool), deps linkDeps) (*LinkSession, error) {
	if ConfigState(cfg) != "" {
		return nil, ErrNotConfigured
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	method := strings.ToLower(strings.TrimSpace(req.Method))
	phone := ""
	if method == LinkMethodPhone {
		number, _ := config.NormalizeE164(req.Phone)
		phone = strings.TrimPrefix(number, "+")
	}
	if parent == nil {
		parent = context.Background()
	}
	store, err := deps.backend.open(parent, cfg.Directories.DataDir)
	if err != nil {
		return nil, fmt.Errorf("open whatsapp device store: %w", err)
	}
	if store.Linked() {
		_ = store.Close()
		return nil, ErrAlreadyLinked
	}
	ctx, cancel := context.WithTimeout(parent, linkTTL)
	l := &LinkSession{method: method, phone: phone, cancel: cancel, done: make(chan struct{}),
		snap: LinkSnapshot{State: LinkStateWaiting, ExpiresAt: deps.now().Add(linkTTL).UTC().Format(time.RFC3339)}}
	integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateLinking)
	go l.run(ctx, store, deps, logger, onFinish)
	return l, nil
}

func (l *LinkSession) run(ctx context.Context, store deviceStore, deps linkDeps, logger *slog.Logger, onFinish func(bool)) {
	client := store.NewClient()
	linked := false
	defer func() {
		// Cancel first: whatsmeow's 515 reconnect after pairing uses ctx.
		l.cancel()
		client.Disconnect()
		_ = store.Close()
		l.mu.Lock()
		l.finished = true
		l.mu.Unlock()
		if !linked {
			integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateNotLinked)
		}
		if onFinish != nil {
			onFinish(linked)
		}
		close(l.done)
	}()
	items, err := client.GetQRChannel(ctx)
	if err != nil {
		l.fail(logger, "link_failed", err)
		return
	}
	if err := client.ConnectContext(ctx); err != nil {
		l.fail(logger, "connect_failed", err)
		return
	}
	codeRequested := false
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				l.set(LinkSnapshot{State: LinkStateTimeout})
			} else {
				l.set(LinkSnapshot{State: LinkStateError, Error: "cancelled"})
			}
			return
		case item, ok := <-items:
			if !ok {
				l.set(LinkSnapshot{State: LinkStateTimeout})
				return
			}
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				if l.method == LinkMethodPhone {
					// PairPhone needs the connection whose first QR event just arrived.
					if codeRequested {
						continue
					}
					codeRequested = true
					code, err := client.PairPhone(ctx, l.phone)
					if err != nil {
						l.fail(logger, "pair_failed", err)
						return
					}
					l.set(LinkSnapshot{State: LinkStateCode, PairingCode: code})
					continue
				}
				uri, err := deps.render(ctx, item.Code)
				if err != nil {
					l.fail(logger, "qr_failed", err)
					return
				}
				l.set(LinkSnapshot{State: LinkStateQR, QRPNG: uri})
			case whatsmeow.QRChannelSuccess.Event:
				linked = true
				l.set(LinkSnapshot{State: LinkStateSuccess})
				return
			case whatsmeow.QRChannelTimeout.Event:
				l.set(LinkSnapshot{State: LinkStateTimeout})
				return
			case whatsmeow.QRChannelScannedWithoutMultidevice.Event:
				// The phone can scan the same code again after enabling linked devices.
				continue
			case whatsmeow.QRChannelEventPasskeyRequest, whatsmeow.QRChannelEventPasskeyResponse:
				l.set(LinkSnapshot{State: LinkStateError, Error: "passkey_unsupported"})
				return
			case whatsmeow.QRChannelClientOutdated.Event:
				l.set(LinkSnapshot{State: LinkStateError, Error: "outdated"})
				return
			default:
				l.fail(logger, "pair_failed", item.Error)
				return
			}
		}
	}
}

// set replaces the snapshot but keeps the session expiry.
func (l *LinkSession) set(next LinkSnapshot) {
	l.mu.Lock()
	defer l.mu.Unlock()
	next.ExpiresAt = l.snap.ExpiresAt
	l.snap = next
}

func (l *LinkSession) fail(logger *slog.Logger, code string, err error) {
	if err != nil {
		logger.Warn("WhatsApp link failed", "code", code, "error", maskDigits(security.Scrub(err.Error())))
	}
	l.set(LinkSnapshot{State: LinkStateError, Error: code})
}

// Snapshot returns a copy of the current state.
func (l *LinkSession) Snapshot() LinkSnapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.snap
}

// Active reports whether the session still owns the device store.
func (l *LinkSession) Active() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return !l.finished
}

// Cancel ends the session. Wait on Done for the cleanup.
func (l *LinkSession) Cancel() { l.cancel() }

// Done is closed after cleanup and onFinish.
func (l *LinkSession) Done() <-chan struct{} { return l.done }

// DeviceLinked reports whether <dataDir>/whatsapp/device.db holds a linked
// device.
func DeviceLinked(ctx context.Context, dataDir string) (bool, error) {
	store, err := (sqliteBackend{}).open(ctx, dataDir)
	if err != nil {
		return false, fmt.Errorf("open whatsapp device store: %w", err)
	}
	defer store.Close()
	return store.Linked(), nil
}

// RemoveLocalDevice deletes the linked device's keys without contacting
// WhatsApp. The owner then removes "AuraGo" from the phone's linked devices.
func RemoveLocalDevice(ctx context.Context, dataDir string) error {
	return removeLocalDevice(ctx, sqliteBackend{}, dataDir)
}
