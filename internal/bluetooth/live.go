package bluetooth

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"
)

// bluezBus is the platform seam of the live BlueZ session.
type bluezBus interface {
	// Watch emits objectsReset first and incremental events afterwards. When
	// bluetoothd restarts it emits objectsLost and later objectsReset. It returns
	// when ctx ends or the bus connection fails.
	Watch(ctx context.Context, emit func(objectEvent)) error
	// Call invokes a method without arguments, e.g. "org.bluez.Device1.Connect".
	Call(ctx context.Context, path, method string) error
	RemoveDevice(ctx context.Context, adapterPath, devicePath string) error
	SetProperty(ctx context.Context, path, iface, name string, value interface{}) error
	Close() error
}

type busDialer func() (bluezBus, error)

// Change tells subscribers that the live snapshot moved to Revision.
type Change struct {
	Revision          uint64
	CapabilityChanged bool
	Present           bool
	// InteractionID is set when a new pairing question was opened.
	InteractionID string
}

const (
	liveDebounce       = 250 * time.Millisecond
	liveBackoffInitial = time.Second
	liveBackoffMax     = 30 * time.Second
	reasonUnreachable  = "BlueZ is not reachable on the system D-Bus."
	reasonDisabled     = "Bluetooth is disabled in the AuraGo configuration."
)

type deviceOperation struct {
	Kind  string
	Error string
}

// liveSession keeps a BlueZ object mirror up to date and publishes debounced
// change notifications. It owns no BlueZ policy; the Manager drives operations.
type liveSession struct {
	dial           busDialer
	logger         *slog.Logger
	debounce       time.Duration
	backoffInitial time.Duration
	backoffMax     time.Duration

	mu                 sync.Mutex
	enabled            bool
	started            bool
	observed           bool
	resyncing          bool
	bus                bluezBus
	tree               objectTree
	revision           uint64
	present            bool
	reason             string
	updatedAt          time.Time
	operations         map[string]deviceOperation
	discoveryEnds      time.Time
	discoverableEnds   time.Time
	interactionID      string
	subscribers        map[int]chan Change
	nextSubscriber     int
	flushArmed         bool
	pendingCapability  bool
	pendingInteraction string
	watchCancel        context.CancelFunc
	wake               chan struct{}
	cancel             context.CancelFunc
	done               chan struct{}
}

func newLiveSession(dial busDialer, logger *slog.Logger) *liveSession {
	if logger == nil {
		logger = slog.Default()
	}
	return &liveSession{
		dial:           dial,
		logger:         logger,
		debounce:       liveDebounce,
		backoffInitial: liveBackoffInitial,
		backoffMax:     liveBackoffMax,
		tree:           objectTree{},
		reason:         "Bluetooth has not been probed yet.",
		operations:     map[string]deviceOperation{},
		subscribers:    map[int]chan Change{},
		wake:           make(chan struct{}, 1),
	}
}

func (l *liveSession) start(parent context.Context, enabled bool) {
	l.mu.Lock()
	if l.started {
		l.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	l.started, l.enabled, l.cancel, l.done = true, enabled, cancel, make(chan struct{})
	l.mu.Unlock()
	go l.run(ctx)
}

func (l *liveSession) stop(timeout time.Duration) {
	l.mu.Lock()
	cancel, done := l.cancel, l.done
	l.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

func (l *liveSession) setEnabled(enabled bool) {
	l.mu.Lock()
	changed := l.enabled != enabled
	l.enabled = enabled
	cancelWatch := l.watchCancel
	l.mu.Unlock()
	if !changed {
		return
	}
	if !enabled && cancelWatch != nil {
		cancelWatch()
	}
	l.signalWake()
}

// resync drops the current watch so the loop re-reads BlueZ at once without
// reporting a temporary loss to subscribers.
func (l *liveSession) resync() {
	l.mu.Lock()
	cancelWatch := l.watchCancel
	if cancelWatch != nil {
		l.resyncing = true
	}
	l.mu.Unlock()
	if cancelWatch != nil {
		cancelWatch()
	}
	l.signalWake()
}

func (l *liveSession) signalWake() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *liveSession) isEnabled() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enabled
}

func (l *liveSession) active() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.started && l.observed
}

func (l *liveSession) run(ctx context.Context) {
	defer close(l.done)
	backoff := l.backoffInitial
	for ctx.Err() == nil {
		if !l.isEnabled() {
			l.markLost(reasonDisabled)
			select {
			case <-ctx.Done():
				return
			case <-l.wake:
				continue
			}
		}
		started := time.Now()
		err := l.watchOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			l.logger.Debug("[Bluetooth] Live session ended", "error", err)
		}
		if time.Since(started) >= l.backoffMax {
			backoff = l.backoffInitial
		}
		select {
		case <-ctx.Done():
			return
		case <-l.wake:
			backoff = l.backoffInitial
		case <-time.After(backoff):
			backoff *= 2
			if backoff > l.backoffMax {
				backoff = l.backoffMax
			}
		}
	}
}

func (l *liveSession) watchOnce(ctx context.Context) error {
	bus, err := l.dial()
	if err != nil {
		l.markLost(reasonUnreachable)
		return err
	}
	watchCtx, cancel := context.WithCancel(ctx)
	l.mu.Lock()
	l.bus, l.watchCancel = bus, cancel
	l.mu.Unlock()
	err = bus.Watch(watchCtx, l.apply)
	cancel()
	l.mu.Lock()
	l.bus, l.watchCancel = nil, nil
	resyncing := l.resyncing
	l.resyncing = false
	l.mu.Unlock()
	_ = bus.Close()
	if !resyncing {
		l.markLost(reasonUnreachable)
	}
	return err
}

func (l *liveSession) apply(event objectEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tree = l.tree.apply(event)
	l.observed = true
	l.updatedAt = time.Now().UTC()
	lost := ""
	if event.Kind == objectsLost {
		lost = event.Reason
		if lost == "" {
			lost = reasonUnreachable
		}
		l.failOperationsLocked()
	}
	l.recomputeLocked(lost)
}

func (l *liveSession) markLost(reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.observed && !l.present && len(l.tree) == 0 && l.reason == reason {
		return
	}
	l.tree = objectTree{}
	l.observed = true
	l.updatedAt = time.Now().UTC()
	l.discoveryEnds = time.Time{}
	l.failOperationsLocked()
	l.recomputeLocked(reason)
}

func (l *liveSession) failOperationsLocked() {
	for address, operation := range l.operations {
		if operation.Kind != "" {
			l.operations[address] = deviceOperation{Error: ErrorUnavailable}
		}
	}
}

func (l *liveSession) recomputeLocked(lostReason string) {
	wasPresent := l.present
	path, properties := l.tree.selectAdapter()
	switch {
	case path == "" && lostReason != "":
		l.present, l.reason = false, lostReason
	case path == "":
		l.present, l.reason = false, "BlueZ is running, but no Bluetooth adapter was found."
	case propString(properties, "PowerState") == powerStateOffBlocked:
		l.present, l.reason = false, "The Bluetooth adapter is blocked by a hardware switch."
	case !propBool(properties, "Powered"):
		l.present, l.reason = true, "The Bluetooth adapter is turned off."
	default:
		l.present, l.reason = true, ""
	}
	l.revision++
	l.scheduleLocked(wasPresent != l.present, "")
}

func (l *liveSession) scheduleLocked(capabilityChanged bool, interactionID string) {
	l.pendingCapability = l.pendingCapability || capabilityChanged
	if interactionID != "" {
		l.pendingInteraction = interactionID
	}
	if l.flushArmed {
		return
	}
	l.flushArmed = true
	time.AfterFunc(l.debounce, l.flush)
}

func (l *liveSession) flush() {
	l.mu.Lock()
	change := Change{Revision: l.revision, CapabilityChanged: l.pendingCapability, Present: l.present, InteractionID: l.pendingInteraction}
	l.flushArmed, l.pendingCapability, l.pendingInteraction = false, false, ""
	subscribers := make([]chan Change, 0, len(l.subscribers))
	for _, ch := range l.subscribers {
		subscribers = append(subscribers, ch)
	}
	l.mu.Unlock()
	for _, ch := range subscribers {
		deliverLatest(ch, change)
	}
}

// deliverLatest keeps only the newest change in a one-slot channel while
// preserving the capability and interaction flags of a replaced change.
func deliverLatest(ch chan Change, change Change) {
	for {
		select {
		case ch <- change:
			return
		default:
		}
		select {
		case old := <-ch:
			change.CapabilityChanged = change.CapabilityChanged || old.CapabilityChanged
			if change.InteractionID == "" {
				change.InteractionID = old.InteractionID
			}
		default:
		}
	}
}

// subscribe returns a one-slot channel that always holds the newest change.
// The channel is never closed; stop reading when the caller's context ends.
func (l *liveSession) subscribe() (<-chan Change, func()) {
	ch := make(chan Change, 1)
	l.mu.Lock()
	id := l.nextSubscriber
	l.nextSubscriber++
	l.subscribers[id] = ch
	l.mu.Unlock()
	return ch, func() {
		l.mu.Lock()
		delete(l.subscribers, id)
		l.mu.Unlock()
	}
}

func (l *liveSession) notifyInteraction(id string, isNew bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.interactionID = id
	l.revision++
	announce := ""
	if isNew {
		announce = id
	}
	l.scheduleLocked(false, announce)
}

func (l *liveSession) currentBus() (bluezBus, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.bus == nil || !l.present {
		return nil, codedError(ErrorUnavailable, firstNonEmpty(l.reason, reasonUnreachable), nil)
	}
	return l.bus, nil
}

func (l *liveSession) adapterPath() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	path, _ := l.tree.selectAdapter()
	return path
}

func (l *liveSession) adapter() AdapterStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	path, properties := l.tree.selectAdapter()
	if path == "" {
		return AdapterStatus{}
	}
	return adapterStatusFromProperties(path, properties)
}

func (l *liveSession) devicePath(address string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	path := l.tree.devicePathByAddress(address)
	if path == "" {
		return "", codedError(ErrorDeviceNotFound, "The Bluetooth device is no longer known to BlueZ.", nil)
	}
	return path, nil
}

func (l *liveSession) deviceByPath(path string) (Device, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	properties := l.tree[path][bluezDeviceInterface]
	if properties == nil {
		return Device{}, false
	}
	return deviceFromProperties(properties, l.tree[path][bluezBatteryInterface]), true
}

// status overlays the live adapter state onto base, which carries the probed
// audio backend.
func (l *liveSession) status(base Status) Status {
	l.mu.Lock()
	defer l.mu.Unlock()
	status := base
	status.Supported = true
	status.Present = l.present
	status.Reason = l.reason
	status.Adapter = AdapterStatus{}
	if path, properties := l.tree.selectAdapter(); path != "" {
		status.Adapter = adapterStatusFromProperties(path, properties)
	}
	status.Usable = l.present && status.Adapter.Powered
	if !status.Usable {
		status.Audio = AudioStatus{Reason: "Bluetooth audio requires a powered adapter."}
		if status.Reason == "" {
			status.Reason = "No powered Bluetooth adapter is available."
		}
	}
	if !l.updatedAt.IsZero() {
		status.LastProbedAt = l.updatedAt
	}
	return status
}

func (l *liveSession) snapshot(now time.Time) Snapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	snapshot := Snapshot{Revision: l.revision, Present: l.present, Reason: l.reason, Devices: []Device{}, InteractionID: l.interactionID}
	if path, properties := l.tree.selectAdapter(); path != "" {
		snapshot.Adapter = adapterStatusFromProperties(path, properties)
		snapshot.Devices = l.tree.devicesOf(path)
	}
	for i := range snapshot.Devices {
		if operation, ok := l.operations[snapshot.Devices[i].Address]; ok {
			snapshot.Devices[i].Operation = operation.Kind
			snapshot.Devices[i].Error = operation.Error
		}
	}
	snapshot.Discovery = timedState(l.discoveryEnds, now)
	snapshot.Discoverable = timedState(l.discoverableEnds, now)
	return snapshot
}

func timedState(ends, now time.Time) TimedState {
	if ends.IsZero() || !ends.After(now) {
		return TimedState{}
	}
	return TimedState{Active: true, EndsAt: ends, RemainingSeconds: int(math.Ceil(ends.Sub(now).Seconds()))}
}

func (l *liveSession) beginOperation(address, kind string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.operations[address].Kind != "" {
		return codedError(ErrorOperationBusy, "Another Bluetooth operation is already running for this device.", nil)
	}
	l.operations[address] = deviceOperation{Kind: kind}
	l.revision++
	l.scheduleLocked(false, "")
	return nil
}

func (l *liveSession) endOperation(address string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err == nil {
		delete(l.operations, address)
	} else {
		code := ErrorCode(err)
		if code == "" {
			code = ErrorGeneric
		}
		l.operations[address] = deviceOperation{Error: code}
	}
	l.revision++
	l.scheduleLocked(false, "")
}

func (l *liveSession) setDiscovery(ends time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.discoveryEnds = ends
	l.revision++
	l.scheduleLocked(false, "")
}

func (l *liveSession) discoveryEnd() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.discoveryEnds
}

func (l *liveSession) setDiscoverable(ends time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.discoverableEnds = ends
	l.revision++
	l.scheduleLocked(false, "")
}
