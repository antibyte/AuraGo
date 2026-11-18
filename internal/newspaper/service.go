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
	Cast        bool
	WhatsApp    bool
	Budget      BudgetConfig
}

type Progress struct {
	Phase    string
	Sources  int
	Stories  int
	Message  string
	Source   *Source
	Research *ResearchStats
}

type ResearchFunc func(context.Context, Profile, time.Time, func(Progress)) (Draft, error)
type DeliveryFunc func(context.Context, Edition, Profile, string) (string, error)
type DestinationFunc func(context.Context, Profile, string) (string, error)

type Options struct {
	Path         string
	Policy       func() Policy
	Research     ResearchFunc
	Deliver      DeliveryFunc
	Narrate      func(context.Context, Edition) (AudioFile, error)
	DeliverAudio func(context.Context, Edition, Profile, string, bool, AudioFile) (string, error)
	Cast         func(context.Context, string, AudioFile, func() error) error
	Destination  DestinationFunc
	Now          func() time.Time
}

type Service struct {
	store        *Store
	policy       func() Policy
	research     ResearchFunc
	narrate      func(context.Context, Edition) (AudioFile, error)
	deliverAudio func(context.Context, Edition, Profile, string, bool, AudioFile) (string, error)
	cast         func(context.Context, string, AudioFile, func() error) error
	audioJobs    map[string]*audioJob
	audioSlot    chan struct{}
	deliver      DeliveryFunc
	destination  DestinationFunc
	now          func() time.Time
	ctx          context.Context
	cancel       context.CancelFunc
	wake         chan struct{}
	mu           sync.Mutex
	running      context.CancelFunc
	wg           sync.WaitGroup
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
	s := &Service{store: store, narrate: opts.Narrate, deliverAudio: opts.DeliverAudio, cast: opts.Cast, audioJobs: make(map[string]*audioJob), audioSlot: make(chan struct{}, 1), policy: opts.Policy, research: opts.Research, deliver: opts.Deliver, destination: opts.Destination, now: now, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1)}
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

func (p Policy) budgetConfig() BudgetConfig {
	cfg := p.Budget
	if cfg.MaxMinutes == 0 {
		cfg.MaxMinutes = p.MaxMinutes
	}
	return cfg
}

func (s *Service) Start(ctx context.Context, newRevision bool) (Run, error) {
	return s.StartWithCorrection(ctx, newRevision, "")
}

func (s *Service) StartWithCorrection(ctx context.Context, newRevision bool, correction string) (Run, error) {
	return s.StartEdition(ctx, newRevision, correction, Creation{})
}

func (s *Service) StartEdition(ctx context.Context, newRevision bool, correction string, choice Creation) (Run, error) {
	if !s.canWrite() {
		return Run{}, ErrDisabled
	}
	if err := validateFormats(choice.Channels, true); err != nil {
		return Run{}, err
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
	return s.startForDate(ctx, date, newRevision, profile, correction, false, choice)
}

func (s *Service) startForDate(ctx context.Context, date string, newRevision bool, profile Profile, correction string, scheduled bool, choice Creation) (Run, error) {
	if !s.canWrite() {
		return Run{}, ErrDisabled
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return Run{}, ErrDisabled
	}
	if s.running != nil {
		return Run{}, ErrBusy
	}
	var run Run
	var err error
	if scheduled {
		run, err = s.store.StartScheduled(ctx, date, s.now())
	} else {
		run, err = s.store.StartCorrected(ctx, date, newRevision, correction, s.now())
	}
	if err != nil {
		return Run{}, err
	}
	policy := s.policy()
	budget := ResolveBudget(profile, policy.budgetConfig())
	work, cancel := context.WithCancel(s.ctx)
	researchCtx, cancelResearch := context.WithTimeout(work, time.Duration(budget.Minutes)*time.Minute)
	researchCtx = WithBudget(researchCtx, budget)
	s.running = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		defer cancelResearch()
		s.execute(work, researchCtx, run, profile, choice)
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

func (s *Service) execute(ctx, researchCtx context.Context, r Run, p Profile, choice Creation) {
	started := s.now().UTC()
	budget, hasBudget := BudgetFromContext(researchCtx)
	if hasBudget {
		stats := ResearchStats{Budget: &budget}
		if r.Research != nil {
			stats = *r.Research
			stats.Budget = &budget
		}
		r.Research = &stats
	}
	sourceTruncation := 0
	progress := func(v Progress) {
		progressCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		truncatedSource := false
		if v.Source != nil {
			limit := maxResearchSources
			if hasBudget {
				limit = budget.Pages
			}
			_, truncatedSource, _ = s.store.RecordRunSource(progressCtx, r.ID, *v.Source, limit)
		}
		r.Phase = v.Phase
		r.Sources = v.Sources
		r.Stories = v.Stories
		if v.Research != nil {
			stats := *v.Research
			if hasBudget {
				stats.Budget = &budget
			}
			r.Research = &stats
		}
		if truncatedSource {
			sourceTruncation++
		}
		if sourceTruncation > 0 {
			if r.Research == nil {
				r.Research = &ResearchStats{}
			}
			if r.Research.SourceTruncation < sourceTruncation {
				stats := *r.Research
				stats.SourceTruncation = sourceTruncation
				r.Research = &stats
			}
		}
		_ = s.store.UpdateRun(progressCtx, r, v.Message)
	}
	draft, err := s.research(researchCtx, p, started, progress)
	if errors.Is(ctx.Err(), context.Canceled) {
		r.Status, r.Phase, r.Reason = "cancelled", "finished", "Research cancelled"
		_ = s.recordFinalRun(r, "Publication stopped after cancellation")
		return
	}
	if !s.canWrite() {
		r.Status, r.Phase, r.Reason = "cancelled", "finished", "Newspaper permission revoked"
		_ = s.recordFinalRun(r, "Publication stopped after permissions changed")
		return
	}
	partial := err != nil || draft.Partial || errors.Is(researchCtx.Err(), context.DeadlineExceeded)
	cutoff := s.now().UTC()
	if vErr := ValidateDraft(draft, p, cutoff); vErr != nil {
		if err == nil {
			err = vErr
		}
		r.Status = "failed"
		if errors.Is(ctx.Err(), context.Canceled) {
			r.Status = "cancelled"
		}
		if errors.Is(researchCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			r.Status = "partial"
		}
		r.Phase = "finished"
		r.Reason = err.Error()
		_ = s.recordFinalRun(r, "No verified edition could be published")
		return
	}
	place := strings.TrimSpace(strings.Join([]string{p.City, p.Region, p.Country}, ", "))
	e := Edition{ID: r.ID, LocalDate: r.LocalDate, Revision: r.Revision, Title: p.Name, Language: p.Language, Place: place, CreatedAt: s.now().UTC(), CutoffAt: cutoff, Partial: partial, Stories: draft.Stories, Sources: draft.Sources}
	if r.CorrectionNote != "" {
		e.Corrections = []string{r.CorrectionNote}
	}
	if err = e.Seal(); err != nil {
		r.Status = "failed"
		r.Reason = err.Error()
		_ = s.recordFinalRun(r, "Edition sealing failed")
		return
	}
	// The research deadline permits a partial edition; the separate run context
	// still carries an explicit stop through publication and delivery.
	publicationCtx, cancelPublication := context.WithTimeout(ctx, 30*time.Second)
	defer cancelPublication()
	stopShutdown := context.AfterFunc(s.ctx, cancelPublication)
	defer stopShutdown()
	if err = s.store.Publish(publicationCtx, r, e); err != nil {
		r.Status = "failed"
		r.Reason = err.Error()
		_ = s.recordFinalRun(r, "Edition publication failed")
		return
	}
	maxEditions := s.policy().MaxEditions
	if maxEditions == 0 {
		maxEditions = 365
	}
	if pruneErr := s.store.Prune(publicationCtx, maxEditions); pruneErr != nil {
		// Publish committed this status; retention must never put it back in running.
		r.Status, r.Phase = "published", "published"
		retentionCtx, retentionCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		_ = s.store.UpdateRun(retentionCtx, r, "Archive retention failed")
		retentionCancel()
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return
	}
	s.afterPublication(ctx, e, p, choice)
}

func (s *Service) recordFinalRun(r Run, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.store.UpdateRun(ctx, r, message)
}

func (s *Service) Deliver(ctx context.Context, id, channel, kind, key string) (Delivery, error) {
	return s.deliverFormat(ctx, id, channel, kind, key, "edition", false)
}

// SendFormat claims an audio delivery before returning, so polling and client disconnects do not replay it.
func (s *Service) SendFormat(ctx context.Context, id, channel, kind, key, format string) (Delivery, error) {
	if format == "" {
		format = "edition"
	}
	return s.deliverFormat(ctx, id, channel, kind, key, format, true)
}

func (s *Service) deliverFormat(ctx context.Context, id, channel, kind, key, format string, retryAudio bool) (Delivery, error) {
	if !validFormat(format) {
		return Delivery{}, errors.New("invalid delivery format")
	}
	if !s.canWrite() || s.deliver == nil || s.destination == nil {
		return Delivery{}, ErrDisabled
	}
	policy := s.policy()
	if channel == "email" && !policy.Email || channel == "telegram" && !policy.Telegram || channel == "whatsapp" && !policy.WhatsApp {
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
	d, claimed, err := s.store.ClaimDelivery(ctx, e, channel, kind, hex.EncodeToString(digest[:]), key, format)
	if err != nil {
		return Delivery{}, err
	}
	if !claimed {
		return d, nil
	}
	if format != "edition" {
		s.mu.Lock()
		if s.ctx.Err() != nil {
			s.mu.Unlock()
			_ = s.store.FinishDelivery(context.Background(), d, "failed", "Preparation interrupted before send", "")
			return d, ErrDisabled
		}
		s.wg.Add(1)
		s.mu.Unlock()
		go func() {
			defer s.wg.Done()
			file, audioErr := s.awaitRecording(s.ctx, e, retryAudio)
			if audioErr != nil && format == "audio" {
				_ = s.store.FinishDelivery(context.Background(), d, "failed", "Audio unavailable before send", "")
				return
			}
			current, err := s.store.Profile(s.ctx)
			if err == nil {
				var currentTarget string
				currentTarget, err = s.destination(s.ctx, current, channel)
				if currentTarget != target || channel == "email" && !current.EmailVerified || kind == "daily" && !current.dailyChannel(channel) {
					err = ErrDisabled
				}
			}
			if err != nil || !s.canWrite() || !s.channelAllowed(channel) {
				_ = s.store.FinishDelivery(context.Background(), d, "failed", "Delivery permission or destination changed", "")
				return
			}
			// Only the transition to sending can represent an ambiguous external side effect.
			result, err := s.store.db.ExecContext(s.ctx, `UPDATE newspaper_deliveries SET status='sending' WHERE id=? AND status='preparing'`, d.ID)
			if err != nil {
				return
			}
			n, _ := result.RowsAffected()
			if n != 1 {
				return
			}
			work, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
			defer cancel()
			var provider string
			if audioErr != nil {
				provider, err = s.deliver(work, e, current, channel)
			} else if s.deliverAudio == nil {
				err = SafeDelivery(ErrDisabled)
			} else {
				provider, err = s.deliverAudio(work, e, current, channel, format == "both", file)
			}
			status, reason := deliveryResult(err)
			if err == nil && audioErr != nil {
				status, reason = "audio_pending", "Edition sent; audio is pending"
			}
			_ = s.store.FinishDelivery(context.Background(), d, status, reason, provider)
		}()
		return d, nil
	}
	// The adapter cannot always know whether a provider accepted a timed-out send.
	// Treat ambiguous errors as uncertain, never replay them automatically.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	providerID, err := s.deliver(ctx, e, p, channel)
	d.Status, d.Reason = deliveryResult(err)
	d.ProviderID = providerID
	finishErr := s.store.FinishDelivery(context.Background(), d, d.Status, d.Reason, providerID)
	return d, errors.Join(err, finishErr)
}

func deliveryResult(err error) (string, string) {
	if err == nil {
		return "sent", ""
	}
	if errors.Is(err, ErrAudioPending) {
		return "audio_pending", "Edition sent; audio is pending"
	}
	var safe *SafeDeliveryError
	if errors.As(err, &safe) {
		return "failed", "No edition was sent; check the channel setup before retrying"
	}
	return "uncertain", "Delivery outcome unknown; inspect the destination before retrying"
}

func (s *Service) channelAllowed(channel string) bool {
	p := s.policy()
	return channel == "email" && p.Email || channel == "telegram" && p.Telegram || channel == "whatsapp" && p.WhatsApp
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
		timer.Reset(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)))
	}
}

func (s *Service) tick() {
	if !s.canWrite() {
		return
	}
	p, err := s.store.Profile(s.ctx)
	if err != nil {
		return
	}
	s.processCasts(p)
	if !p.Daily && !(p.CastDaily && s.policy().Cast) {
		return
	}
	loc, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		return
	}
	now := s.now()
	budget := ResolveBudget(p, s.policy().budgetConfig())
	for offset := 0; offset <= 1; offset++ {
		date := now.In(loc).AddDate(0, 0, offset).Format("2006-01-02")
		for _, window := range s.publicationWindows(p) {
			ready, err := readyInstant(date, window.clock, loc)
			if err != nil || now.Before(ready.Add(-time.Duration(budget.Minutes+10)*time.Minute)) || now.After(ready.Add(window.grace)) {
				continue
			}
			_, err = s.startForDate(s.ctx, date, false, p, "", true, Creation{})
			if err == nil || errors.Is(err, ErrBusy) {
				return
			}
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
		var next time.Time
		for _, window := range s.publicationWindows(p) {
			ready, err := readyInstant(date, window.clock, loc)
			if err == nil && ready.After(now) && (next.IsZero() || ready.Before(next)) {
				next = ready
			}
		}
		if !next.IsZero() {
			return next, nil
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
