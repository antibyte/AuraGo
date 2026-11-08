package server

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"aurago/internal/agent"
	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
)

type helperDecisionProfile struct {
	consumers.HelperStatus
	identity    [32]byte
	revision    uint64
	lastSuccess time.Time
	retryAfter  time.Time
}

// helperProfileLocked changes only ephemeral qualification state, never runtime state.
func (h *decisionHub) helperProfileLocked(base consumers.Snapshot) *helperDecisionProfile {
	if h.helperProfiles == nil {
		h.helperProfiles = make(map[consumers.ID]*helperDecisionProfile)
	}
	p := h.helperProfiles[base.Consumer]
	if p == nil || p.identity != base.Identity {
		p = &helperDecisionProfile{HelperStatus: consumers.HelperStatus{ID: base.Consumer, Enabled: true, Code: "helper_pending"}, identity: base.Identity}
		h.helperProfiles[base.Consumer] = p
	}
	if p.Available && h.clock().Sub(p.lastSuccess) >= consumers.HelperQualificationTTL {
		p.Available, p.Code = false, "helper_expired"
		p.revision++
	}
	return p
}

func (h *decisionHub) qualifiedHelperSnapshot(base consumers.Snapshot) consumers.Snapshot {
	h.mu.Lock()
	p := h.helperProfileLocked(base)
	available, code, revision := p.Available, p.Code, p.revision
	h.mu.Unlock()
	result := base
	var identity [40]byte
	copy(identity[:], base.Identity[:])
	binary.LittleEndian.PutUint64(identity[32:], revision)
	result.Identity = sha256.Sum256(identity[:])
	result.Available, result.Code = available, code
	result.Current = func() bool {
		if base.Current == nil || !base.Current() {
			return false
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		p := h.helperProfileLocked(base)
		return p.revision == revision && p.Available
	}
	result.RecordUse = func(evaluations, avoided int) {
		if !result.Current() {
			return
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		p := h.helperProfileLocked(base)
		if p.revision == revision {
			p.Evaluations += max(0, evaluations)
			p.AvoidedCalls += max(0, avoided)
		}
	}
	result.Evaluate = func(parent context.Context, request decisions.Request) (decisions.Response, error) {
		if !result.Current() {
			return decisions.Response{}, decisions.NewError("helper_unqualified", nil)
		}
		if !consumers.HelperRequestFits(base.Consumer, request) {
			return decisions.Response{}, decisions.NewError("helper_input_limit", nil)
		}
		start := h.clock()
		ctx, cancel := context.WithTimeout(parent, consumers.HelperTimeout)
		defer cancel()
		response, err := base.Evaluate(ctx, request)
		if parent.Err() != nil {
			return response, decisions.NewError(decisions.CodeCancelled, parent.Err())
		}
		if !base.Current() {
			return response, decisions.NewError(consumers.CodeConfigurationChanged, context.Canceled)
		}
		code := decisions.ErrorCode(err)
		if code == consumers.CodeAdmissionDeferred || code == consumers.CodePreempted || code == decisions.CodeQueueFull || code == decisions.CodeRuntimeBusy {
			return response, err
		}
		if err == nil {
			err = consumers.ValidateHelperResponse(request, response)
		}
		elapsed := h.clock().Sub(start)
		if ctx.Err() != nil || elapsed >= consumers.HelperTimeout {
			err = context.DeadlineExceeded
		}
		if !result.Current() {
			return response, decisions.NewError(consumers.CodeConfigurationChanged, context.Canceled)
		}
		h.mu.Lock()
		p := h.helperProfileLocked(base)
		if p.revision != revision {
			h.mu.Unlock()
			return response, decisions.NewError(consumers.CodeConfigurationChanged, context.Canceled)
		}
		p.LastResponseMS = float64(elapsed) / float64(time.Millisecond)
		p.MaxResponseMS = max(p.MaxResponseMS, p.LastResponseMS)
		if err != nil {
			p.Available, p.Code, p.retryAfter = false, "helper_failed", h.clock().Add(consumers.HelperRetryDelay)
			if errors.Is(err, context.DeadlineExceeded) {
				p.Code = "helper_slow"
			}
			p.revision++
		} else {
			p.lastSuccess = h.clock()
		}
		h.mu.Unlock()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return response, decisions.NewError(decisions.CodeRuntimeScoreTimeout, err)
			}
			return response, decisions.NewError("helper_failed", err)
		}
		return response, nil
	}
	return result
}

func (h *decisionHub) qualifyHelperProfile(ctx context.Context, id consumers.ID) {
	if ctx.Err() != nil {
		return
	}
	cfg := h.s.ConfigSnapshot()
	if cfg == nil || !policyFor(cfg, id).Helper {
		return
	}
	base := h.snapshot(cfg, id, false)
	if !base.Available || base.Evaluate == nil {
		return
	}
	h.mu.Lock()
	p := h.helperProfileLocked(base)
	if p.Available || h.clock().Before(p.retryAfter) {
		h.mu.Unlock()
		return
	}
	p.Code, p.Samples = "helper_testing", 0
	revision := p.revision
	h.mu.Unlock()
	request := agent.HelperQualificationRequest(id)
	for sample := 0; sample <= consumers.HelperSamples; sample++ {
		// Vary the first tokens so qualification cannot depend on an identical prompt cache hit.
		request.State = fmt.Sprintf("Synthetic performance sample %d.\n%s", sample, agent.HelperQualificationRequest(id).State)
		if ctx.Err() != nil || !base.Current() {
			return
		}
		start := h.clock()
		callCtx, cancel := context.WithTimeout(ctx, consumers.HelperTimeout)
		response, err := base.Evaluate(callCtx, request)
		if err == nil {
			err = consumers.ValidateHelperResponse(request, response)
		}
		elapsed := h.clock().Sub(start)
		deadline := callCtx.Err()
		cancel()
		if ctx.Err() != nil || !base.Current() {
			return
		}
		code := decisions.ErrorCode(err)
		if code == consumers.CodeAdmissionDeferred || code == consumers.CodePreempted || code == decisions.CodeQueueFull || code == decisions.CodeRuntimeBusy {
			h.mu.Lock()
			p := h.helperProfileLocked(base)
			if p.revision == revision {
				p.Code = "helper_pending"
			}
			h.mu.Unlock()
			return
		}
		h.mu.Lock()
		p := h.helperProfileLocked(base)
		if p.revision != revision {
			h.mu.Unlock()
			return
		}
		p.ProbeCalls++
		p.LastResponseMS = float64(elapsed) / float64(time.Millisecond)
		p.MaxResponseMS = max(p.MaxResponseMS, p.LastResponseMS)
		if err != nil || deadline != nil || elapsed >= consumers.HelperTimeout {
			p.Available, p.Code, p.retryAfter = false, "helper_failed", h.clock().Add(consumers.HelperRetryDelay)
			if deadline != nil || elapsed >= consumers.HelperTimeout {
				p.Code = "helper_slow"
			}
			p.revision++
			h.mu.Unlock()
			return
		}
		if sample > 0 {
			p.Samples++
		}
		if p.Samples == consumers.HelperSamples {
			p.Available, p.Code, p.lastSuccess = true, consumers.CodeAvailable, h.clock()
		}
		h.mu.Unlock()
	}
}

func (h *decisionHub) startHelperQualifications(ctx context.Context) {
	h.mu.Lock()
	if h.helperCancel != nil {
		h.mu.Unlock()
		return
	}
	ctx, h.helperCancel = context.WithCancel(ctx)
	done := make(chan struct{})
	h.helperDone = done
	h.mu.Unlock()
	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			for _, id := range consumers.HelperProfiles {
				h.qualifyHelperProfile(ctx, id)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (h *decisionHub) stopHelperQualifications(ctx context.Context) {
	h.mu.Lock()
	cancel, done := h.helperCancel, h.helperDone
	h.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (h *decisionHub) helperStatus() []consumers.HelperStatus {
	out := make([]consumers.HelperStatus, 0, len(consumers.HelperProfiles))
	cfg := h.s.ConfigSnapshot()
	for _, id := range consumers.HelperProfiles {
		status := consumers.HelperStatus{ID: id, Code: "helper_disabled"}
		if cfg != nil && policyFor(cfg, id).Helper {
			base := h.snapshot(cfg, id, true)
			status.Enabled, status.Code = policyFor(cfg, id).on(id), base.Code
			h.mu.Lock()
			if p := h.helperProfiles[id]; p != nil {
				status = p.HelperStatus
				status.Enabled, status.Available, status.Code = policyFor(cfg, id).on(id), base.Available, base.Code
			}
			h.mu.Unlock()
		}
		out = append(out, status)
	}
	return out
}
