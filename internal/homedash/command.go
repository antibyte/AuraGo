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
// dedupe, slots, sems and queuedHook. mu is never held across a Source call, never taken together
// with Hub.mu (the hub never holds mu while locking Hub.mu, and vice versa).
type commandState struct {
	limiter rateLimiter
	mu      sync.Mutex
	dedupe  map[string]*dedupeEntry
	slots   map[string]*cmdSlot
	sems    map[SourceID]chan struct{}

	// queuedHook is a test hook: it runs after a command was parked in a latest-wins slot.
	queuedHook func()
}

type dedupeEntry struct {
	at   time.Time
	done chan struct{}
	res  CommandResult
	err  error
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

func (e *dedupeEntry) wait(ctx context.Context) (CommandResult, error) {
	select {
	case <-e.done:
		return e.res, e.err
	case <-ctx.Done():
		return CommandResult{}, ctx.Err()
	}
}

// Command runs the pipeline of backend section 5 (validate, gates, kiosk rules, confirmation, rate
// limits, request-id dedupe, in-flight control, Source.Command, audit). Agents have no command path.
func (h *Hub) Command(ctx context.Context, cmd Command) (CommandResult, error) {
	spec, err := h.admitCommand(ctx, &cmd)
	if err != nil {
		h.audit(ctx, cmd, err)
		return CommandResult{}, err
	}
	if cmd.RequestID == "" {
		res, err := h.dispatch(ctx, cmd, spec)
		h.audit(ctx, cmd, err)
		return res, err
	}
	key := string(cmd.Principal.Kind) + "|" + cmd.Principal.Key + "|" + cmd.RequestID
	entry, first, full := h.cmds.claim(key, h.opts.Now())
	if full {
		err := &Error{Code: CodeRateLimited, Message: "too many commands", RetryAfterMS: 1000}
		h.audit(ctx, cmd, err)
		return CommandResult{}, err
	}
	if !first {
		return entry.wait(ctx)
	}
	defer close(entry.done) // also on a panic outside the source call, so waiters never hang
	entry.res, entry.err = h.dispatch(ctx, cmd, spec)
	if errors.Is(entry.err, context.Canceled) || errors.Is(entry.err, context.DeadlineExceeded) {
		h.cmds.forget(key, entry) // the caller gave up: a retry with the same id may run
	}
	h.audit(ctx, cmd, entry.err)
	return entry.res, entry.err
}

func (h *Hub) audit(ctx context.Context, cmd Command, err error) {
	h.mu.Lock()
	fn := h.opts.Audit
	h.mu.Unlock()
	if fn == nil {
		return
	}
	outcome := "ok"
	var he *Error
	switch {
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

// admitCommand applies every gate in order and returns the entity's capability spec. Order:
//
//	principal kind (agent / unknown refused) -> request id -> hub closed / disabled -> kiosk switch ->
//	source registered -> source control settings present -> source active and entity cataloged ->
//	capability and arguments -> integration allowed -> HomeDash read-only -> desktop read-only ->
//	source control switch -> kiosk allowed set -> view-only -> kiosk-denied -> confirmation ->
//	rate limits.
//
// Every gate is evaluated against the configuration current when the second critical section runs.
func (h *Hub) admitCommand(ctx context.Context, cmd *Command) (CapabilitySpec, error) {
	switch cmd.Principal.Kind {
	case PrincipalSession, PrincipalKiosk:
	case PrincipalAgent:
		return CapabilitySpec{}, &Error{Code: CodeForbiddenEntity, Message: "agents send no device commands", BlockedBy: "agent"}
	default:
		return CapabilitySpec{}, &Error{Code: CodeForbiddenEntity, Message: "unknown principal", BlockedBy: "principal"}
	}
	if len(cmd.RequestID) > maxRequestIDLen || ContainsControl(cmd.RequestID) {
		return CapabilitySpec{}, newError(CodeBadRequest, "invalid request_id")
	}
	kiosk := cmd.Principal.Kind == PrincipalKiosk

	h.mu.Lock()
	switch {
	case h.closed:
		h.mu.Unlock()
		return CapabilitySpec{}, ErrHubClosed
	case !h.cfg.active():
		h.mu.Unlock()
		return CapabilitySpec{}, newError(CodeHomeDashDisabled, "HomeDash is disabled")
	case kiosk && !h.cfg.KioskEnabled:
		// Before any entity lookup: a disabled kiosk learns nothing about entities.
		h.mu.Unlock()
		return CapabilitySpec{}, newError(CodeKioskDisabled, "wall displays are switched off")
	}
	h.mu.Unlock()

	src, native, err := ParseEntityID(string(cmd.Entity))
	if err != nil {
		return CapabilitySpec{}, ErrEntityNotFound
	}
	cmd.Native = native

	h.mu.Lock()
	rt := h.rts[src]
	if rt == nil {
		h.mu.Unlock()
		return CapabilitySpec{}, ErrEntityNotFound
	}
	if set, ok := h.cfg.Sources[src]; !ok {
		// A registered source without a settings entry fails closed.
		blocked := controlBlock(src, set, ok)
		h.mu.Unlock()
		return CapabilitySpec{}, readOnlyErr(blocked)
	}
	if !rt.active || rt.src == nil {
		h.mu.Unlock()
		return CapabilitySpec{}, ErrEntityNotFound
	}
	var jobs []catalogJob
	if _, have := rt.catalog[cmd.Entity]; !have {
		jobs = append(jobs, h.catalogJobLocked(rt))
	}
	h.mu.Unlock()
	h.fetchCatalogs(ctx, jobs)

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return CapabilitySpec{}, ErrHubClosed
	}
	rt = h.rts[src]
	if rt == nil || !rt.active {
		return CapabilitySpec{}, ErrEntityNotFound
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
	cfg := h.cfg
	set, haveSet := cfg.Sources[src]
	switch blocked := controlBlock(src, set, haveSet); {
	case !cfg.active():
		return CapabilitySpec{}, newError(CodeHomeDashDisabled, "HomeDash is disabled")
	case !spec.Allowed:
		by := spec.BlockedBy
		if by == "" {
			by = "integration"
		}
		return CapabilitySpec{}, readOnlyErr(by)
	case cfg.ReadOnly:
		return CapabilitySpec{}, readOnlyErr("homedash.readonly")
	case cfg.DesktopReadOnly:
		return CapabilitySpec{}, readOnlyErr("virtual_desktop.readonly")
	case blocked != "":
		return CapabilitySpec{}, readOnlyErr(blocked)
	}
	if kiosk {
		p := cmd.Principal
		switch {
		case !cfg.KioskEnabled:
			return CapabilitySpec{}, newError(CodeKioskDisabled, "wall displays are switched off")
		case !p.Allowed.Allows(cmd.Entity, cmd.Name): // a nil set allows nothing
			return CapabilitySpec{}, newError(CodeForbiddenEntity, "this display may not send this command")
		case p.ViewOnly || !cfg.KioskAllowControl:
			return CapabilitySpec{}, newError(CodeViewOnly, "this display is view-only")
		case info.KioskDenied:
			return CapabilitySpec{}, &Error{Code: CodeForbiddenEntity, Message: "not allowed from a wall display", BlockedBy: "kiosk.denied"}
		}
	}
	if (spec.Sensitive || info.Sensitive) && !cmd.Confirmed {
		return CapabilitySpec{}, newError(CodeConfirmationRequired, "this command needs a confirmation")
	}
	pk := string(cmd.Principal.Kind) + ":" + cmd.Principal.Key
	if ok, wait := h.cmds.limiter.allow(h.opts.Now(), []window{
		{"pm:" + pk, cfg.Limits.CommandsPerMinute, time.Minute},
		{"pb:" + pk, burstLimit, burstWindow},
		{"e:" + string(cmd.Entity), entityLimit, entityWindow},
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
			// The caller left: withdraw the command if it still waits, so it is not sent later.
			c.mu.Lock()
			if slot.next == q {
				slot.next = nil
				c.mu.Unlock()
				return CommandResult{}, ctx.Err()
			}
			c.mu.Unlock()
			select { // already taken by the drain goroutine or superseded: report its outcome
			case <-q.done:
				return q.res, q.err
			default:
			}
			return CommandResult{}, ctx.Err()
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

// execute counts as interest while it runs, holds one of 4 per-source slots and never reports raw
// upstream text. Source.Command runs here, with no hub or command-state lock held.
func (h *Hub) execute(ctx context.Context, cmd Command) (CommandResult, error) {
	srcID, _, _ := ParseEntityID(string(cmd.Entity))
	h.mu.Lock()
	rt := h.rts[srcID]
	if h.closed || rt == nil || !rt.active || rt.src == nil {
		h.mu.Unlock()
		return CommandResult{}, newError(CodeSourceUnavailable, "the source is not available")
	}
	source := rt.src
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
	case <-ctx.Done():
		return CommandResult{}, ctx.Err()
	}
	defer func() { <-sem }()
	cctx, cancel := context.WithTimeout(ctx, h.opts.CommandTimeout)
	defer cancel()
	res, err := commandGuarded(cctx, source, cmd)
	if err != nil {
		var he *Error
		if errors.As(err, &he) {
			return CommandResult{}, he
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
