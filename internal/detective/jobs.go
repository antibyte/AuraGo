package detective

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Session struct {
	service    *Service
	CaseID     string
	cancel     context.CancelFunc
	last       time.Time
	published  bool
	stopReason string
}

func (s *Service) Start(key, action, effort, requestKey string, answers ...string) (Case, error) {
	if requestKey == "" || len(requestKey) > 128 {
		return Case{}, errors.New("idempotency key is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Case{}, ErrClosed
	}
	if s.runner == nil {
		return Case{}, errors.New("research runner unavailable")
	}
	c, err := s.getLocked(key)
	if err != nil {
		return c, err
	}
	var previous string
	err = s.db.QueryRow("SELECT run_id FROM detective_keys WHERE case_id=? AND key=?", key, requestKey).Scan(&previous)
	if err == nil {
		if previous != c.Run.ID {
			return c, ErrConflict
		}
		return c, nil
	}
	if activeStatus(c.Run.Status) || (s.active != nil && s.active.CaseID == key) {
		return c, ErrConflict
	}
	switch action {
	case "start":
		if c.Run.Status != "draft" {
			return c, ErrConflict
		}
	case "continue":
		if c.Run.Status != "cancelled" && c.Run.Status != "interrupted" && c.Run.Status != "partial" && c.Run.Status != "failed" && c.Run.Status != "waiting_for_user" {
			return c, ErrConflict
		}
		if c.Run.Usage.ActiveMS >= int64(c.Run.Profile.Seconds)*1000 || c.Run.Usage.Iterations >= c.Run.Profile.Iterations {
			return c, ErrBudget
		}
	case "deepen":
		if len(c.Reports) >= 30 {
			return c, errors.New("report revision limit reached")
		}
	default:
		return c, errors.New("unknown research action")
	}
	if action != "continue" {
		if effort == "" {
			effort = c.Request.Effort
		}
		p, ok := s.profiles[effort]
		if !ok {
			return c, errors.New("unknown effort")
		}
		c.Request.Effort = effort
		c.Run = Run{ID: id("run_"), Profile: p, Phase: "research"}
	}
	if len(answers) > 0 && strings.TrimSpace(answers[0]) != "" {
		if action != "continue" || c.Run.Status != "waiting_for_user" || len(answers[0]) > 8000 || len(c.Answers) >= 20 {
			return c, errors.New("invalid clarification answer")
		}
		c.Answers = append(c.Answers, answers[0])
	}
	c.Run.Status = "queued"
	c.Run.Reason = ""
	c.UpdatedAt = s.now().UTC()
	c.Run.UpdatedAt = c.UpdatedAt
	b, err := json.Marshal(c)
	if err != nil {
		return c, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return c, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE detective_cases SET body=?,status=?,updated=? WHERE id=?", b, c.Run.Status, c.UpdatedAt.Format(time.RFC3339Nano), key); err != nil {
		return c, err
	}
	if _, err = tx.Exec("INSERT INTO detective_keys(case_id,key,run_id) VALUES(?,?,?)", key, requestKey, c.Run.ID); err != nil {
		return c, err
	}
	if err = tx.Commit(); err != nil {
		return c, err
	}
	s.eventLocked(&c, "status", "queued")
	s.wake()
	return c, nil
}

func (s *Service) Stop(key string, finish bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.getLocked(key)
	if err != nil {
		return err
	}
	if !activeStatus(c.Run.Status) {
		return ErrConflict
	}
	if finish {
		c.Run.Phase = "writing"
		s.eventLocked(&c, "phase", "writing")
	} else {
		c.Run.Status = "cancelled"
		c.Run.Reason = "user_stopped"
		if s.active != nil && s.active.CaseID == key {
			s.active.stopReason = "user_stopped"
			s.active.cancel()
		}
		s.eventLocked(&c, "status", "cancelled")
	}
	return s.saveLocked(&c)
}

func (s *Service) worker() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.kick:
		}
		for {
			s.mu.Lock()
			if s.closed || s.runner == nil {
				s.mu.Unlock()
				break
			}
			var key string
			err := s.db.QueryRow("SELECT id FROM detective_cases WHERE status='queued' ORDER BY updated LIMIT 1").Scan(&key)
			if err != nil {
				s.mu.Unlock()
				break
			}
			c, err := s.getLocked(key)
			if err != nil {
				s.mu.Unlock()
				break
			}
			ctx, cancel := context.WithCancel(s.ctx)
			session := &Session{service: s, CaseID: key, cancel: cancel}
			s.active = session
			err = s.saveLocked(&c)
			runner := s.runner
			s.mu.Unlock()
			if err == nil {
				err = runSafely(ctx, runner, session)
			}
			cancel()
			session.complete(err)
			s.mu.Lock()
			s.active = nil
			s.mu.Unlock()
		}
	}
}

func runSafely(ctx context.Context, runner Runner, session *Session) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("research execution failed")
		}
	}()
	return runner.Run(ctx, session)
}

func (x *Session) Snapshot() (Case, error) { return x.service.Get(x.CaseID) }

// Activate starts the active budget only after the shared agent execution slot
// is acquired. A heartbeat bounds crash-related accounting loss to one second.
func (x *Session) Activate(parent context.Context) (context.Context, context.CancelFunc, error) {
	var remaining time.Duration
	err := x.update(func(c *Case) error {
		if c.Run.Status != "queued" {
			return context.Canceled
		}
		remaining = time.Duration(c.Run.Profile.Seconds)*time.Second - time.Duration(c.Run.Usage.ActiveMS)*time.Millisecond
		if remaining <= 0 {
			return ErrBudget
		}
		c.Run.Status = "running"
		if c.Run.StartedAt.IsZero() {
			c.Run.StartedAt = x.service.now().UTC()
		}
		x.last = x.service.now()
		x.service.eventLocked(c, "status", "running")
		return nil
	})
	if err != nil {
		return parent, func() {}, err
	}
	ctx, cancel := context.WithTimeout(parent, remaining)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = x.update(func(*Case) error { return nil })
			}
		}
	}()
	return ctx, func() { cancel(); <-done }, nil
}
func (x *Session) touch(c *Case) {
	now := x.service.now()
	if x.last.IsZero() {
		return
	}
	delta := now.Sub(x.last).Milliseconds()
	if delta > 0 {
		c.Run.Usage.ActiveMS += delta
	}
	x.last = now
}
func (x *Session) update(fn func(*Case) error) error {
	s := x.service
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.getLocked(x.CaseID)
	if err != nil {
		return err
	}
	x.touch(&c)
	err = fn(&c)
	saveErr := s.saveLocked(&c)
	if err != nil {
		return err
	}
	return saveErr
}
func (x *Session) Iterate() error {
	return x.update(func(c *Case) error {
		if c.Run.Status != "running" {
			return context.Canceled
		}
		c.Run.Usage.Iterations++
		if c.Run.Usage.Iterations > c.Run.Profile.Iterations || c.Run.Usage.ActiveMS >= int64(c.Run.Profile.Seconds)*1000 {
			return ErrBudget
		}
		if c.Run.Usage.ActiveMS >= int64(c.Run.Profile.Seconds)*800 || c.Run.Usage.Tools >= c.Run.Profile.Tools || (c.Run.Profile.Tokens > 0 && c.Run.Usage.PromptTokens+c.Run.Usage.CompletionTokens >= c.Run.Profile.Tokens*8/10) {
			if c.Run.Phase != "writing" {
				c.Run.Phase = "writing"
				x.service.eventLocked(c, "phase", "writing")
			}
		}
		return nil
	})
}
func (x *Session) BeforeTool(name string) error {
	return x.update(func(c *Case) error {
		if c.Run.Status != "running" {
			return context.Canceled
		}
		if name == "detective_report" {
			if x.published {
				return errors.New("report already published")
			}
			return nil
		}
		if c.Run.Phase == "writing" || c.Run.Usage.Tools >= c.Run.Profile.Tools || c.Run.Usage.ActiveMS >= int64(c.Run.Profile.Seconds)*800 {
			c.Run.Phase = "writing"
			return fmt.Errorf("research allowance complete: use detective_report to finish from recorded evidence")
		}
		c.Run.Usage.Tools++
		x.service.eventLocked(c, "tool", name)
		return nil
	})
}
func (x *Session) RecordUsage(prompt, completion, cached int) error {
	return x.update(func(c *Case) error {
		u := &c.Run.Usage
		u.PromptTokens += max(0, prompt)
		u.CompletionTokens += max(0, completion)
		u.CachedTokens += max(0, cached)
		if c.Run.Profile.Tokens > 0 && u.PromptTokens+u.CompletionTokens >= c.Run.Profile.Tokens {
			return ErrBudget
		}
		return nil
	})
}

func (x *Session) BeforeRequest(estimatedPrompt, requestedOutput int) (int, error) {
	output := requestedOutput
	err := x.update(func(c *Case) error {
		if c.Run.Status != "running" {
			return context.Canceled
		}
		if c.Run.Profile.Tokens > 0 {
			remaining := c.Run.Profile.Tokens - c.Run.Usage.PromptTokens - c.Run.Usage.CompletionTokens
			// Reserve a margin for tokenizer differences and provider protocol fields.
			output = min(output, remaining-estimatedPrompt-estimatedPrompt/10-256)
			if output < 256 {
				return ErrBudget
			}
		}
		c.Run.Usage.Requests++
		return nil
	})
	return output, err
}
func (x *Session) SetPlan(plan []string) error {
	if len(plan) > 12 {
		return errors.New("at most 12 research questions")
	}
	for _, p := range plan {
		if len(p) > 800 {
			return errors.New("research question too long")
		}
	}
	return x.update(func(c *Case) error {
		c.Plan = plan
		x.service.eventLocked(c, "plan", strings.Join(plan, " · "))
		return nil
	})
}
func (x *Session) Event(kind, text string) {
	_ = x.update(func(c *Case) error { x.service.eventLocked(c, kind, text); return nil })
}
func (x *Session) Continuation() (Continuation, error) { return x.service.Continuation(x.CaseID) }
func (x *Session) Checkpoint(v Continuation) error     { return x.service.SaveContinuation(x.CaseID, v) }
func (x *Session) Done() bool                          { s := x.service; s.mu.Lock(); defer s.mu.Unlock(); return x.published }

func (x *Session) Wait(question string) error {
	return x.update(func(c *Case) error {
		c.Run.Status = "waiting_for_user"
		c.Run.Reason = bounded(question, 600)
		x.service.eventLocked(c, "question", question)
		x.cancel()
		return nil
	})
}

func (x *Session) complete(runErr error) {
	_ = x.update(func(c *Case) error {
		if x.published {
			c.Run.Status = "completed"
			if c.Reports[len(c.Reports)-1].Partial {
				c.Run.Status = "partial"
			}
			return nil
		}
		if c.Run.Status == "waiting_for_user" {
			return nil
		}
		reason := "incomplete_response"
		if runErr != nil {
			reason = "provider_or_tool_failure"
		}
		if x.service.closed {
			reason = "server_shutdown"
			c.Run.Status = "interrupted"
		} else if x.stopReason != "" {
			reason = x.stopReason
			c.Run.Status = "cancelled"
		} else if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runErr, ErrBudget) || c.Run.Usage.ActiveMS >= int64(c.Run.Profile.Seconds)*1000 {
			reason = "budget_exhausted"
			c.Run.Status = "partial"
		} else {
			c.Run.Status = "partial"
			if runErr != nil && len(c.Findings) == 0 {
				c.Run.Status = "failed"
			}
		}
		c.Run.Reason = reason
		if len(c.Findings) > 0 && len(c.Reports) < 30 {
			report := Report{Title: c.Request.Topic, Summary: "Partial research findings", Partial: true, Limitations: reason}
			for _, f := range c.Findings {
				report.Blocks = append(report.Blocks, Block{Type: "paragraph", Text: f.Text, Evidence: []string{f.ID}})
			}
			sealReport(c, &report, x.service.now())
			c.Reports = append(c.Reports, report)
		}
		x.service.eventLocked(c, "status", reason)
		return nil
	})
}
