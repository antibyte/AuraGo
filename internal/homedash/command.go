package homedash

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	dedupeTTL         = 30 * time.Second
	maxDedupeEntries  = 4096
	perSourceInFlight = 4
	burstLimit        = 10
	burstWindow       = 10 * time.Second
	entityLimit       = 5
	entityWindow      = 5 * time.Second
	maxRequestIDLen   = 64
)

// commandState is the mutable state of the command pipeline. limiter has its own lock; mu guards
// dedupe, slots, sems and the test hooks. mu is never held across a Source call, never taken
// together with Hub.mu (the hub never holds mu while locking Hub.mu, and vice versa).
type commandState struct {
	limiter rateLimiter
	mu      sync.Mutex
	dedupe  map[string]*dedupeEntry
	slots   map[string]*cmdSlot
	sems    map[SourceID]chan struct{}

	// Test hooks (guarded by mu): queuedHook runs after a command was parked in a latest-wins slot,
	// semWaitHook when a command starts waiting for an in-flight slot, dedupeWaitHook when a
	// duplicate request starts waiting for the first one.
	queuedHook, semWaitHook, dedupeWaitHook func()
}

func (c *commandState) hooks() (queued, semWait, dedupeWait func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.queuedHook, c.semWaitHook, c.dedupeWaitHook
}

type dedupeEntry struct {
	at        time.Time
	done      chan struct{}
	res       CommandResult
	err       error
	abandoned bool // the first caller left before the command ran; waiters claim the id again
}

type cmdSlot struct{ next *queuedCmd }

type queuedCmd struct {
	cmd  Command
	done chan struct{}
	res  CommandResult
	err  error
}

func newCommandState() *commandState {
	return &commandState{dedupe: map[string]*dedupeEntry{}, slots: map[string]*cmdSlot{}, sems: map[SourceID]chan struct{}{}}
}

func (c *commandState) sem(src SourceID) chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sems[src]
	if s == nil {
		s = make(chan struct{}, perSourceInFlight)
		c.sems[src] = s
	}
	return s
}

// claim registers a request id. first is false when another call owns it; full is true when the
// bounded map holds maxDedupeEntries live ids (the caller then refuses).
func (c *commandState) claim(key string, now time.Time) (e *dedupeEntry, first, full bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, old := range c.dedupe {
		if now.Sub(old.at) > dedupeTTL {
			delete(c.dedupe, k)
		}
	}
	if old, ok := c.dedupe[key]; ok {
		return old, false, false
	}
	if len(c.dedupe) >= maxDedupeEntries {
		return nil, false, true
	}
	e = &dedupeEntry{at: now, done: make(chan struct{})}
	c.dedupe[key] = e
	return e, true, false
}

// forget removes a request id again (only while e still owns it).
func (c *commandState) forget(key string, e *dedupeEntry) {
	c.mu.Lock()
	if c.dedupe[key] == e {
		delete(c.dedupe, key)
	}
	c.mu.Unlock()
}

// wait returns the first request's result; retry is true when that request was abandoned.
func (e *dedupeEntry) wait(ctx context.Context) (res CommandResult, retry bool, err error) {
	select {
	case <-e.done:
		return e.res, e.abandoned, e.err
	case <-ctx.Done():
		return CommandResult{}, false, ctx.Err()
	}
}

// Command runs the pipeline of backend section 5 (validate, gates, kiosk rules, confirmation, rate
// limits, request-id dedupe, in-flight control, Source.Command, audit). Agents have no command path.
func (h *Hub) Command(ctx context.Context, cmd Command) (CommandResult, error) {
	spec, err := h.admitCommand(ctx, &cmd)
	if err != nil {
		h.audit(ctx, cmd, CommandResult{}, err)
		return CommandResult{}, err
	}
	if cmd.RequestID == "" {
		res, err := h.dispatch(ctx, cmd, spec)
		h.audit(ctx, cmd, res, err)
		return res, err
	}
	key := string(cmd.Principal.Kind) + "|" + cmd.Principal.Key + "|" + cmd.RequestID
	for {
		entry, first, full := h.cmds.claim(key, h.opts.Now())
		if full {
			err := &Error{Code: CodeRateLimited, Message: "too many commands", RetryAfterMS: 1000}
			h.audit(ctx, cmd, CommandResult{}, err)
			return CommandResult{}, err
		}
		if first {
			return h.runClaimed(ctx, cmd, spec, key, entry)
		}
		if _, _, hook := h.cmds.hooks(); hook != nil {
			hook()
		}
		res, retry, err := entry.wait(ctx)
		if !retry {
			return res, err
		}
		// The first caller left before the command ran: one waiter claims the id and runs it, the
		// others wait for that one.
	}
}

// runClaimed runs the command of the caller that owns a request id. Every outcome except the
// caller's own cancellation stays cached for dedupeTTL as the first result, command_failed
// included: a retry of the same id within the period gets the same answer, not a second attempt.
func (h *Hub) runClaimed(ctx context.Context, cmd Command, spec CapabilitySpec, key string, entry *dedupeEntry) (CommandResult, error) {
	defer close(entry.done) // also on a panic outside the source call, so waiters never hang
	entry.res, entry.err = h.dispatch(ctx, cmd, spec)
	if ctx.Err() != nil && (errors.Is(entry.err, context.Canceled) || errors.Is(entry.err, context.DeadlineExceeded)) {
		entry.abandoned = true
		h.cmds.forget(key, entry)
	}
	h.audit(ctx, cmd, entry.res, entry.err)
	return entry.res, entry.err
}

func (h *Hub) audit(ctx context.Context, cmd Command, res CommandResult, err error) {
	h.mu.Lock()
	fn := h.opts.Audit
	h.mu.Unlock()
	if fn == nil {
		return
	}
	outcome := "ok"
	var he *Error
	switch {
	case err == nil && res.Superseded:
		outcome = "superseded"
	case err == nil:
	case errors.As(err, &he) && he.Code != CodeCommandFailed && he.Code != CodeSourceError:
		outcome = "refused:" + he.Code
	default:
		outcome = "failed"
	}
	fn(ctx, cmd, outcome)
}

func findCap(caps []CapabilitySpec, c Capability) (CapabilitySpec, bool) {
	for _, s := range caps {
		if s.Name == c {
			return s, true
		}
	}
	return CapabilitySpec{}, false
}

func readOnlyErr(blockedBy string) *Error {
	return &Error{Code: CodeReadOnly, Message: "commands are switched off", BlockedBy: blockedBy}
}

// errKioskForbidden is the one refusal a display gets for an entity or command outside its set,
// whether the entity exists or not.
func errKioskForbidden() *Error {
	return newError(CodeForbiddenEntity, "this display may not send this command")
}

func errPrincipal() *Error {
	return &Error{Code: CodeForbiddenEntity, Message: "unknown principal", BlockedBy: "principal"}
}

// The gate helpers below are shared by admitCommand and execute (which re-checks right before the
// source call), so the two cannot drift. All of them need Hub.mu.

// stateGateLocked: hub closed, HomeDash disabled, kiosk switch for a kiosk principal.
func (h *Hub) stateGateLocked(p Principal) error {
	switch {
	case h.closed:
		return ErrHubClosed
	case !h.cfg.active():
		return newError(CodeHomeDashDisabled, "HomeDash is disabled")
	case p.Kind == PrincipalKiosk && !h.cfg.KioskEnabled:
		return newError(CodeKioskDisabled, "wall displays are switched off")
	}
	return nil
}

// sourceGateLocked: the source is registered, has a settings entry (fails closed) and is active.
func (h *Hub) sourceGateLocked(src SourceID) (*sourceRT, error) {
	rt := h.rts[src]
	if rt == nil {
		return nil, ErrEntityNotFound
	}
	if set, ok := h.cfg.Sources[src]; !ok {
		return nil, readOnlyErr(controlBlock(src, set, ok))
	}
	if !rt.active || rt.src == nil {
		return nil, ErrEntityNotFound
	}
	return rt, nil
}

// hubSwitchesLocked: HomeDash read-only, desktop read-only, the source's control switch.
func (h *Hub) hubSwitchesLocked(src SourceID) error {
	set, ok := h.cfg.Sources[src]
	switch blocked := controlBlock(src, set, ok); {
	case h.cfg.ReadOnly:
		return readOnlyErr("homedash.readonly")
	case h.cfg.DesktopReadOnly:
		return readOnlyErr("virtual_desktop.readonly")
	case blocked != "":
		return readOnlyErr(blocked)
	}
	return nil
}

// kioskControlLocked: a view-only display, or kiosk control switched off.
func (h *Hub) kioskControlLocked(p Principal) error {
	if p.ViewOnly || !h.cfg.KioskAllowControl {
		return newError(CodeViewOnly, "this display is view-only")
	}
	return nil
}

// executeGateLocked is every switch that can change while a command waits.
func (h *Hub) executeGateLocked(src SourceID, p Principal) (*sourceRT, error) {
	if err := h.stateGateLocked(p); err != nil {
		return nil, err
	}
	rt, err := h.sourceGateLocked(src)
	if err != nil {
		return nil, err
	}
	if err := h.hubSwitchesLocked(src); err != nil {
		return nil, err
	}
	if p.Kind == PrincipalKiosk {
		if err := h.kioskControlLocked(p); err != nil {
			return nil, err
		}
	}
	return rt, nil
}

// admitCommand applies every gate in order and returns the entity's capability spec. Order:
//
//	principal kind and key (agent, unknown kind, empty key refused) -> request id -> hub closed /
//	HomeDash disabled / kiosk switch -> kiosk: entity in the allowed set (same refusal for unknown
//	and foreign entities) -> id parse -> source registered -> settings entry present -> source active ->
//	entity cataloged (catalog fetched only when stale) -> capability and arguments -> integration
//	allowed -> HomeDash read-only -> desktop read-only -> source control switch -> kiosk allowed
//	command -> view-only -> kiosk-denied -> confirmation -> rate limits.
//
// execute repeats the switch checks right before the source call.
func (h *Hub) admitCommand(ctx context.Context, cmd *Command) (CapabilitySpec, error) {
	switch cmd.Principal.Kind {
	case PrincipalSession, PrincipalKiosk:
		if cmd.Principal.Key == "" {
			return CapabilitySpec{}, errPrincipal() // rate-limit and dedupe namespaces must never merge
		}
	case PrincipalAgent:
		return CapabilitySpec{}, &Error{Code: CodeForbiddenEntity, Message: "agents send no device commands", BlockedBy: "agent"}
	default:
		return CapabilitySpec{}, errPrincipal()
	}
	if len(cmd.RequestID) > maxRequestIDLen || ContainsControl(cmd.RequestID) {
		return CapabilitySpec{}, newError(CodeBadRequest, "invalid request_id")
	}
	kiosk := cmd.Principal.Kind == PrincipalKiosk

	h.mu.Lock()
	err := h.stateGateLocked(cmd.Principal)
	h.mu.Unlock()
	if err != nil {
		return CapabilitySpec{}, err
	}
	// Before any lookup or catalog fetch: a display cannot tell an unknown entity from a foreign one.
	if kiosk && !cmd.Principal.Allowed.Has(cmd.Entity) {
		return CapabilitySpec{}, errKioskForbidden()
	}

	src, native, err := ParseEntityID(string(cmd.Entity))
	if err != nil {
		return CapabilitySpec{}, ErrEntityNotFound
	}
	cmd.Native = native

	h.mu.Lock()
	rt, err := h.sourceGateLocked(src)
	if err != nil {
		h.mu.Unlock()
		return CapabilitySpec{}, err
	}
	var jobs []catalogJob
	if _, have := rt.catalog[cmd.Entity]; !have && h.catalogStale(rt, h.opts.Now()) {
		jobs = append(jobs, h.catalogJobLocked(rt))
	}
	h.mu.Unlock()
	h.fetchCatalogs(ctx, jobs)

	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.stateGateLocked(cmd.Principal); err != nil {
		return CapabilitySpec{}, err
	}
	rt, err = h.sourceGateLocked(src)
	if err != nil {
		return CapabilitySpec{}, err
	}
	e, ok := rt.catalog[cmd.Entity]
	if !ok {
		return CapabilitySpec{}, ErrEntityNotFound
	}
	spec, ok := findCap(e.Capabilities, cmd.Name)
	info, known := LookupCapability(cmd.Name)
	if !ok || !known || !info.Command {
		return CapabilitySpec{}, newError(CodeUnsupportedCommand, "the entity does not offer this command")
	}
	if err := ValidateArgs(spec, cmd.Args); err != nil {
		return CapabilitySpec{}, err
	}
	if !spec.Allowed {
		by := spec.BlockedBy
		if by == "" {
			by = "integration"
		}
		return CapabilitySpec{}, readOnlyErr(by)
	}
	if err := h.hubSwitchesLocked(src); err != nil {
		return CapabilitySpec{}, err
	}
	if kiosk {
		p := cmd.Principal
		if !p.Allowed.Allows(cmd.Entity, cmd.Name) { // a nil set allows nothing
			return CapabilitySpec{}, errKioskForbidden()
		}
		if err := h.kioskControlLocked(p); err != nil {
			return CapabilitySpec{}, err
		}
		if info.KioskDenied {
			return CapabilitySpec{}, &Error{Code: CodeForbiddenEntity, Message: "not allowed from a wall display", BlockedBy: "kiosk.denied"}
		}
	}
	if (spec.Sensitive || info.Sensitive) && !cmd.Confirmed {
		return CapabilitySpec{}, newError(CodeConfirmationRequired, "this command needs a confirmation")
	}
	cfg := h.cfg
	pk := string(cmd.Principal.Kind) + ":" + cmd.Principal.Key
	if ok, wait := h.cmds.limiter.allow(h.opts.Now(), []window{
		{"pm:" + pk, cfg.Limits.CommandsPerMinute, time.Minute},
		{"pb:" + pk, burstLimit, burstWindow},
		// Keyed by kind too: wall displays cannot use up the entity window of desktop sessions.
		{"e:" + string(cmd.Principal.Kind) + ":" + string(cmd.Entity), entityLimit, entityWindow},
	}); !ok {
		return CapabilitySpec{}, &Error{Code: CodeRateLimited, Message: "too many commands", RetryAfterMS: max(1, wait.Milliseconds())}
	}
	return spec, nil
}

// dispatch keeps one queued command per (entity, capability) for continuous controls: a newer
// value replaces a waiting one (the replaced caller gets Superseded) and the queue drains after the
// running command, on a hub goroutine.
func (h *Hub) dispatch(ctx context.Context, cmd Command, _ CapabilitySpec) (CommandResult, error) {
	info, _ := LookupCapability(cmd.Name)
	if !info.Continuous {
		return h.execute(ctx, cmd)
	}
	key := string(cmd.Entity) + "|" + string(cmd.Name)
	c := h.cmds
	c.mu.Lock()
	if slot, busy := c.slots[key]; busy {
		q := &queuedCmd{cmd: cmd, done: make(chan struct{})}
		if prev := slot.next; prev != nil {
			prev.res = CommandResult{Superseded: true}
			close(prev.done)
		}
		slot.next = q
		hook := c.queuedHook
		c.mu.Unlock()
		if hook != nil {
			hook()
		}
		select {
		case <-q.done:
			return q.res, q.err
		case <-ctx.Done():
			// Still parked: withdraw it so it is not sent later.
			c.mu.Lock()
			if slot.next == q {
				slot.next = nil
				c.mu.Unlock()
				return CommandResult{}, ctx.Err()
			}
			c.mu.Unlock()
			// Already taken by the drain (it will run) or superseded: report what happened, bounded
			// by the command timeout.
			t := time.NewTimer(h.opts.CommandTimeout)
			defer t.Stop()
			select {
			case <-q.done:
				return q.res, q.err
			case <-t.C:
				return CommandResult{}, ctx.Err()
			}
		}
	}
	slot := &cmdSlot{}
	c.slots[key] = slot
	c.mu.Unlock()
	res, err := h.execute(ctx, cmd)
	h.releaseSlot(key, slot)
	return res, err
}

func (h *Hub) releaseSlot(key string, slot *cmdSlot) {
	c := h.cmds
	c.mu.Lock()
	q := slot.next
	slot.next = nil
	if q == nil {
		delete(c.slots, key)
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	// wg.Add happens under Hub.mu after the closed check: Close sets closed under the same lock
	// before it waits, so Add never races Wait.
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		q.err = ErrHubClosed
		close(q.done)
		c.mu.Lock()
		delete(c.slots, key)
		c.mu.Unlock()
		return
	}
	h.wg.Add(1)
	h.mu.Unlock()
	go func() {
		defer h.wg.Done()
		q.res, q.err = h.execute(h.ctx, q.cmd)
		close(q.done)
		h.releaseSlot(key, slot)
	}()
}

func commandGuarded(ctx context.Context, src Source, cmd Command) (res CommandResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			res, err = CommandResult{}, fmt.Errorf("command panic: %v", r)
		}
	}()
	return src.Command(ctx, cmd)
}

// execute counts as interest while it runs, holds one of 4 per-source slots, re-checks the switches
// under Hub.mu right before the source call and never reports raw upstream text. Source.Command
// runs here, with no hub or command-state lock held.
func (h *Hub) execute(ctx context.Context, cmd Command) (CommandResult, error) {
	srcID, _, _ := ParseEntityID(string(cmd.Entity))
	h.mu.Lock()
	rt, err := h.executeGateLocked(srcID, cmd.Principal)
	if err != nil {
		h.mu.Unlock()
		return CommandResult{}, err
	}
	rt.inflight++
	note := h.reconcileLocked(rt)
	h.mu.Unlock()
	h.deliver([]*interestNote{note})
	defer func() {
		h.mu.Lock()
		if rt.inflight > 0 {
			rt.inflight--
		}
		note := h.reconcileLocked(rt)
		h.mu.Unlock()
		h.deliver([]*interestNote{note})
	}()
	sem := h.cmds.sem(srcID)
	select {
	case sem <- struct{}{}:
	default:
		if _, hook, _ := h.cmds.hooks(); hook != nil {
			hook()
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return CommandResult{}, ctx.Err()
		}
	}
	defer func() { <-sem }()

	// The command may have waited for a slot or sat in the latest-wins queue: the owner's switches
	// are read again now, and the current source instance is the one that gets the call.
	h.mu.Lock()
	cur, err := h.executeGateLocked(srcID, cmd.Principal)
	var source Source
	if err == nil {
		source = cur.src
	}
	h.mu.Unlock()
	if err != nil {
		return CommandResult{}, err
	}
	cctx, cancel := context.WithTimeout(ctx, h.opts.CommandTimeout)
	defer cancel()
	res, err := commandGuarded(cctx, source, cmd)
	if err != nil {
		var he *Error
		if errors.As(err, &he) {
			return CommandResult{}, he
		}
		if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
			return CommandResult{}, ctx.Err() // the caller left: report that, not a device failure
		}
		h.log.Warn("homedash: command failed", "source", srcID, "command", cmd.Name, "error", err)
		return CommandResult{}, newError(CodeCommandFailed, "the device did not accept the command")
	}
	if res.Expected != nil {
		st := sanitizeState(*res.Expected, h.opts.Now())
		res.Expected = &st
	}
	return res, nil
}
