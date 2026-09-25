package newspaper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Policy struct {
	Enabled     bool
	ReadOnly    bool
	MaxMinutes  int
	MaxEditions int
	Email       bool
	Telegram    bool
}

type Progress struct {
	Phase   string
	Sources int
	Stories int
	Message string
	Source  *Source
}

type ResearchFunc func(context.Context, Profile, time.Time, func(Progress)) (Draft, error)
type DeliveryFunc func(context.Context, Edition, Profile, string) (string, error)
type DestinationFunc func(context.Context, Profile, string) (string, error)

type Options struct {
	Path        string
	Policy      func() Policy
	Research    ResearchFunc
	Deliver     DeliveryFunc
	Destination DestinationFunc
	Now         func() time.Time
}

type Service struct {
	store       *Store
	policy      func() Policy
	research    ResearchFunc
	deliver     DeliveryFunc
	destination DestinationFunc
	now         func() time.Time
	ctx         context.Context
	cancel      context.CancelFunc
	wake        chan struct{}
	mu          sync.Mutex
	running     context.CancelFunc
	wg          sync.WaitGroup
}

func New(opts Options) (*Service, error) {
	store, err := Open(opts.Path)
	if err != nil {
		return nil, err
	}
	if opts.Policy == nil || opts.Research == nil {
		store.Close()
		return nil, errors.New("newspaper policy and research are required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{store: store, policy: opts.Policy, research: opts.Research, deliver: opts.Deliver, destination: opts.Destination, now: now, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1)}
	s.wg.Add(1)
	go s.schedule()
	return s, nil
}

func (s *Service) Close() error {
	s.cancel()
	s.mu.Lock()
	if s.running != nil {
		s.running()
	}
	s.mu.Unlock()
	s.wg.Wait()
	return s.store.Close()
}

func (s *Service) Profile(ctx context.Context) (Profile, error) { return s.store.Profile(ctx) }
func (s *Service) SaveProfile(ctx context.Context, p Profile) (Profile, error) {
	if !s.canWrite() {
		return Profile{}, ErrDisabled
	}
	result, err := s.store.SaveProfile(ctx, p)
	if err == nil {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
	return result, err
}
func (s *Service) Get(ctx context.Context, id string) (Edition, error) { return s.store.Get(ctx, id) }
func (s *Service) List(ctx context.Context, limit int) ([]Edition, error) {
	return s.store.List(ctx, limit)
}
func (s *Service) LatestRun(ctx context.Context) (Run, error)      { return s.store.LatestRun(ctx) }
func (s *Service) Run(ctx context.Context, id string) (Run, error) { return s.store.Run(ctx, id) }
func (s *Service) Events(ctx context.Context, id string, after int64) ([]Event, error) {
	return s.store.Events(ctx, id, after)
}
func (s *Service) RunSources(ctx context.Context, id string) ([]Source, error) {
	return s.store.RunSources(ctx, id)
}
func (s *Service) Deliveries(ctx context.Context, id string) ([]Delivery, error) {
	return s.store.Deliveries(ctx, id)
}
func (s *Service) Store() *Store { return s.store }

func (s *Service) canWrite() bool { p := s.policy(); return p.Enabled && !p.ReadOnly }

func (s *Service) Start(ctx context.Context, newRevision bool) (Run, error) {
	if !s.canWrite() {
		return Run{}, ErrDisabled
	}
	profile, err := s.store.Profile(ctx)
	if err != nil {
		return Run{}, err
	}
	if err = profile.Validate(); err != nil {
		return Run{}, err
	}
	loc, _ := time.LoadLocation(profile.TimeZone)
	now := s.now()
	date := now.In(loc).Format("2006-01-02")
	return s.startForDate(ctx, date, newRevision, profile)
}

func (s *Service) startForDate(ctx context.Context, date string, newRevision bool, profile Profile) (Run, error) {
	if !s.canWrite() {
		return Run{}, ErrDisabled
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running != nil {
		return Run{}, ErrBusy
	}
	run, err := s.store.Start(ctx, date, newRevision, s.now())
	if err != nil {
		return Run{}, err
	}
	policy := s.policy()
	minutes := policy.MaxMinutes
	if minutes < 1 || minutes > 60 {
		minutes = 30
	}
	work, cancel := context.WithTimeout(s.ctx, time.Duration(minutes)*time.Minute)
	s.running = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		s.execute(work, run, profile)
		s.mu.Lock()
		s.running = nil
		s.mu.Unlock()
	}()
	return run, nil
}

func (s *Service) Stop(ctx context.Context, id string) error {
	if !s.canWrite() {
		return ErrDisabled
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running == nil {
		return ErrNotFound
	}
	r, err := s.store.Run(ctx, id)
	if err != nil {
		return err
	}
	if r.Status != "running" {
		return ErrConflict
	}
	s.running()
	return nil
}

func (s *Service) execute(ctx context.Context, r Run, p Profile) {
	started := s.now().UTC()
	progress := func(v Progress) {
		if v.Source != nil {
			_ = s.store.RecordRunSource(context.Background(), r.ID, *v.Source)
		}
		r.Phase = v.Phase
		r.Sources = v.Sources
		r.Stories = v.Stories
		_ = s.store.UpdateRun(context.Background(), r, v.Message)
	}
	draft, err := s.research(ctx, p, started, progress)
	partial := err != nil || draft.Partial
	cutoff := s.now().UTC()
	if vErr := ValidateDraft(draft, p, cutoff); vErr != nil {
		if err == nil {
			err = vErr
		}
		r.Status = "failed"
		if errors.Is(ctx.Err(), context.Canceled) {
			r.Status = "cancelled"
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			r.Status = "partial"
		}
		r.Phase = "finished"
		r.Reason = err.Error()
		_ = s.store.UpdateRun(context.Background(), r, "No verified edition could be published")
		return
	}
	place := strings.TrimSpace(strings.Join([]string{p.City, p.Region, p.Country}, ", "))
	e := Edition{ID: r.ID, LocalDate: r.LocalDate, Revision: r.Revision, Title: p.Name, Language: p.Language, Place: place, CreatedAt: s.now().UTC(), CutoffAt: cutoff, Partial: partial, Stories: draft.Stories, Sources: draft.Sources}
	if err = e.Seal(); err != nil {
		r.Status = "failed"
		r.Reason = err.Error()
		_ = s.store.UpdateRun(context.Background(), r, "Edition sealing failed")
		return
	}
	if err = s.store.Publish(context.Background(), r, e); err != nil {
		r.Status = "failed"
		r.Reason = err.Error()
		_ = s.store.UpdateRun(context.Background(), r, "Edition publication failed")
		return
	}
	maxEditions := s.policy().MaxEditions
	if maxEditions == 0 {
		maxEditions = 365
	}
	_ = s.store.Prune(context.Background(), maxEditions)
	if p.EmailDaily && s.policy().Email {
		_, _ = s.Deliver(context.Background(), e.ID, "email", "daily", "daily")
	}
	if p.TelegramDaily && s.policy().Telegram {
		_, _ = s.Deliver(context.Background(), e.ID, "telegram", "daily", "daily")
	}
}

func (s *Service) Deliver(ctx context.Context, id, channel, kind, key string) (Delivery, error) {
	if !s.canWrite() || s.deliver == nil || s.destination == nil {
		return Delivery{}, ErrDisabled
	}
	policy := s.policy()
	if channel == "email" && !policy.Email || channel == "telegram" && !policy.Telegram {
		return Delivery{}, ErrDisabled
	}
	e, err := s.store.Get(ctx, id)
	if err != nil {
		return Delivery{}, err
	}
	p, err := s.store.Profile(ctx)
	if err != nil {
		return Delivery{}, err
	}
	if channel == "email" && !p.EmailVerified {
		return Delivery{}, errors.New("email destination is not verified")
	}
	target, err := s.destination(ctx, p, channel)
	if err != nil {
		return Delivery{}, err
	}
	digest := sha256.Sum256([]byte(channel + "\x00" + target))
	d, claimed, err := s.store.ClaimDelivery(ctx, e, channel, kind, hex.EncodeToString(digest[:]), key)
	if err != nil {
		return Delivery{}, err
	}
	if !claimed {
		return d, nil
	}
	// The adapter cannot always know whether a provider accepted a timed-out send.
	// Treat ambiguous errors as uncertain, never replay them automatically.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	providerID, err := s.deliver(ctx, e, p, channel)
	if err != nil {
		var safe *SafeDeliveryError
		if errors.As(err, &safe) {
			d.Status = "failed"
			d.Reason = "No edition was sent; check the channel setup before retrying"
		} else {
			d.Status = "uncertain"
			d.Reason = "Delivery outcome unknown; inspect the destination before retrying"
		}
		_ = s.store.FinishDelivery(context.Background(), d, d.Status, d.Reason, providerID)
		return d, err
	}
	d.Status = "sent"
	d.ProviderID = providerID
	_ = s.store.FinishDelivery(context.Background(), d, "sent", "", providerID)
	return d, nil
}

func (s *Service) schedule() {
	defer s.wg.Done()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.wake:
		case <-timer.C:
		}
		s.tick()
		timer.Reset(time.Minute)
	}
}

func (s *Service) tick() {
	if !s.canWrite() {
		return
	}
	p, err := s.store.Profile(s.ctx)
	if err != nil || !p.Daily {
		return
	}
	loc, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		return
	}
	now := s.now()
	for _, date := range []time.Time{now.In(loc), now.In(loc).AddDate(0, 0, 1)} {
		ready, err := readyInstant(date.Format("2006-01-02"), p.ReadyTime, loc)
		if err != nil {
			continue
		}
		minutes := s.policy().MaxMinutes
		if minutes < 1 || minutes > 60 {
			minutes = 30
		}
		due := ready.Add(-time.Duration(minutes+10) * time.Minute)
		if now.Before(due) || now.After(ready.Add(3*time.Hour)) {
			continue
		}
		_, err = s.startForDate(s.ctx, date.Format("2006-01-02"), false, p)
		if err == nil || errors.Is(err, ErrBusy) {
			return
		}
	}
}

func (s *Service) NextRun(ctx context.Context) (time.Time, error) {
	p, err := s.store.Profile(ctx)
	if err != nil {
		return time.Time{}, err
	}
	loc, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		return time.Time{}, err
	}
	now := s.now()
	for offset := 0; offset < 3; offset++ {
		date := now.In(loc).AddDate(0, 0, offset).Format("2006-01-02")
		ready, e := readyInstant(date, p.ReadyTime, loc)
		if e == nil && ready.After(now) {
			return ready, nil
		}
	}
	return time.Time{}, fmt.Errorf("could not calculate next issue time")
}

// readyInstant selects the first real occurrence of a local minute. A DST gap
// advances to the next valid minute on the date; a repeated minute uses the first.
func readyInstant(date, clock string, loc *time.Location) (time.Time, error) {
	if !clockRE.MatchString(clock) {
		return time.Time{}, errors.New("invalid ready time")
	}
	day, err := time.Parse("2006-01-02", date)
	if err != nil {
		return time.Time{}, err
	}
	wanted := clock[:2] + clock[3:]
	start := day.Add(-14 * time.Hour)
	end := day.Add(38 * time.Hour)
	for at := start; at.Before(end); at = at.Add(time.Minute) {
		local := at.In(loc)
		if local.Format("2006-01-02") == date && local.Format("1504") >= wanted {
			return at, nil
		}
	}
	return time.Time{}, errors.New("local date has no valid minute")
}
