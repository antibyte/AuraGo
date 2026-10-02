package bluetooth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"math"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const interactionTimeout = 20 * time.Second

// InteractionKind names one pairing question the operator must see or answer.
type InteractionKind string

const (
	InteractionConfirmPasskey   InteractionKind = "confirm_passkey"
	InteractionEnterPasskey     InteractionKind = "enter_passkey"
	InteractionEnterPIN         InteractionKind = "enter_pin"
	InteractionDisplayPasskey   InteractionKind = "display_passkey"
	InteractionDisplayPIN       InteractionKind = "display_pin"
	InteractionAuthorizePairing InteractionKind = "authorize_pairing"
	InteractionAuthorizeService InteractionKind = "authorize_service"
)

// Interaction is the admin-only view of the open pairing question. Passkey and
// PIN are filled only for kinds that must show them and are dropped on close.
type Interaction struct {
	ID               string          `json:"id"`
	Kind             InteractionKind `json:"kind"`
	DeviceAddress    string          `json:"device_address,omitempty"`
	DeviceName       string          `json:"device_name,omitempty"`
	Passkey          string          `json:"passkey,omitempty"`
	PIN              string          `json:"pin,omitempty"`
	Entered          int             `json:"entered,omitempty"`
	Service          string          `json:"service,omitempty"`
	ExpiresAt        time.Time       `json:"expires_at"`
	RemainingSeconds int             `json:"remaining_seconds"`
}

type interactionAnswer struct {
	accept bool
	value  string
}

type pendingInteraction struct {
	view   Interaction
	answer chan interactionAnswer
	timer  *time.Timer
	closed bool
}

// interactionBroker holds at most one open pairing question, never logs or
// persists its secrets, and rejects unanswered questions after timeout.
type interactionBroker struct {
	mu       sync.Mutex
	now      func() time.Time
	timeout  time.Duration
	open     *pendingInteraction
	recent   []string
	onChange func(id string, isNew bool)
}

var (
	passkeyPattern = regexp.MustCompile(`^[0-9]{1,6}$`)
	pinPattern     = regexp.MustCompile(`^[\x20-\x7e]{1,16}$`)
)

func newInteractionBroker() *interactionBroker {
	return &interactionBroker{now: time.Now, timeout: interactionTimeout}
}

func newInteractionID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

func needsAnswer(kind InteractionKind) bool {
	return kind != InteractionDisplayPasskey && kind != InteractionDisplayPIN
}

func (b *interactionBroker) notify(id string, isNew bool) {
	if b.onChange != nil {
		b.onChange(id, isNew)
	}
}

func (b *interactionBroker) openLocked(view Interaction) *pendingInteraction {
	b.closeLocked(b.open, interactionAnswer{})
	view.ID = newInteractionID()
	view.ExpiresAt = b.now().Add(b.timeout)
	pending := &pendingInteraction{view: view, answer: make(chan interactionAnswer, 1)}
	b.open = pending
	return pending
}

// closeLocked ends pending, drops its secrets and hands result to a waiting ask.
func (b *interactionBroker) closeLocked(pending *pendingInteraction, result interactionAnswer) bool {
	if pending == nil || pending.closed {
		return false
	}
	pending.closed = true
	if pending.timer != nil {
		pending.timer.Stop()
	}
	pending.view.Passkey, pending.view.PIN = "", ""
	select {
	case pending.answer <- result:
	default:
	}
	if b.open == pending {
		b.open = nil
	}
	b.recent = append(b.recent, pending.view.ID)
	if len(b.recent) > 8 {
		b.recent = b.recent[len(b.recent)-8:]
	}
	return true
}

func (b *interactionBroker) wasRecentLocked(id string) bool {
	for _, recent := range b.recent {
		if recent == id {
			return true
		}
	}
	return false
}

// ask opens an answerable question and blocks until it is answered, replaced,
// cancelled, expired or ctx ends. Anything but an answer is a rejection.
func (b *interactionBroker) ask(ctx context.Context, view Interaction) (interactionAnswer, error) {
	b.mu.Lock()
	pending := b.openLocked(view)
	id, timeout := pending.view.ID, b.timeout
	b.mu.Unlock()
	b.notify(id, true)

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	var result interactionAnswer
	var err error
	select {
	case result = <-pending.answer:
	case <-timer.C:
		err = codedError(ErrorInteractionExpired, "The pairing request expired.", nil)
	case <-ctx.Done():
		err = ctx.Err()
	}
	b.mu.Lock()
	closed := b.closeLocked(pending, interactionAnswer{})
	b.mu.Unlock()
	if closed {
		b.notify("", false)
	}
	if err != nil {
		return interactionAnswer{}, err
	}
	return result, nil
}

// show opens or updates a display-only question that closes itself on timeout.
func (b *interactionBroker) show(view Interaction) {
	b.mu.Lock()
	if current := b.open; current != nil && !current.closed && current.view.Kind == view.Kind && current.view.DeviceAddress == view.DeviceAddress {
		current.view.Passkey, current.view.PIN, current.view.Entered = view.Passkey, view.PIN, view.Entered
		id := current.view.ID
		b.mu.Unlock()
		b.notify(id, false)
		return
	}
	pending := b.openLocked(view)
	pending.timer = time.AfterFunc(b.timeout, func() {
		b.mu.Lock()
		closed := b.closeLocked(pending, interactionAnswer{})
		b.mu.Unlock()
		if closed {
			b.notify("", false)
		}
	})
	id := pending.view.ID
	b.mu.Unlock()
	b.notify(id, true)
}

func (b *interactionBroker) get(id string) (Interaction, error) {
	if b == nil {
		return Interaction{}, codedError(ErrorInteractionNotFound, "The pairing request is no longer open.", nil)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.open == nil || b.open.closed || b.open.view.ID != id {
		if b.wasRecentLocked(id) {
			return Interaction{}, codedError(ErrorInteractionExpired, "The pairing request has already ended.", nil)
		}
		return Interaction{}, codedError(ErrorInteractionNotFound, "The pairing request is no longer open.", nil)
	}
	view := b.open.view
	view.RemainingSeconds = int(math.Max(0, math.Ceil(view.ExpiresAt.Sub(b.now()).Seconds())))
	return view, nil
}

func (b *interactionBroker) answer(id string, accept bool, value string) error {
	if b == nil {
		return codedError(ErrorInteractionNotFound, "The pairing request is no longer open.", nil)
	}
	b.mu.Lock()
	pending := b.open
	if pending == nil || pending.closed || pending.view.ID != id {
		expired := b.wasRecentLocked(id)
		b.mu.Unlock()
		if expired {
			return codedError(ErrorInteractionExpired, "The pairing request has already ended.", nil)
		}
		return codedError(ErrorInteractionNotFound, "The pairing request is no longer open.", nil)
	}
	if !needsAnswer(pending.view.Kind) {
		b.mu.Unlock()
		return codedError(ErrorInvalidArgument, "This pairing step does not take an answer.", nil)
	}
	if !accept {
		value = ""
	} else {
		switch pending.view.Kind {
		case InteractionEnterPasskey:
			if !passkeyPattern.MatchString(value) {
				b.mu.Unlock()
				return codedError(ErrorInvalidArgument, "Enter 1 to 6 digits.", nil)
			}
		case InteractionEnterPIN:
			if !pinPattern.MatchString(value) {
				b.mu.Unlock()
				return codedError(ErrorInvalidArgument, "Enter 1 to 16 printable characters.", nil)
			}
		default:
			value = ""
		}
	}
	b.closeLocked(pending, interactionAnswer{accept: accept, value: value})
	b.mu.Unlock()
	b.notify("", false)
	return nil
}

func (b *interactionBroker) cancel() {
	if b == nil {
		return
	}
	b.mu.Lock()
	closed := b.closeLocked(b.open, interactionAnswer{})
	b.mu.Unlock()
	if closed {
		b.notify("", false)
	}
}

func (b *interactionBroker) current() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.open == nil || b.open.closed {
		return ""
	}
	return b.open.view.ID
}
