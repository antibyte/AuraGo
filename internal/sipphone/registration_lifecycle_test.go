package sipphone

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emiago/diago"
	"github.com/emiago/sipgo/sip"
)

type scriptedRegistrationTransaction struct {
	options           diago.RegisterOptions
	registerErr       error
	qualifyErr        error
	registered        chan struct{}
	qualifyUntilClose bool
	unregisterStarted chan struct{}
	unregisterRelease <-chan struct{}
	unregisterOnce    sync.Once
}

func (t *scriptedRegistrationTransaction) Register(context.Context) error {
	if t.registerErr != nil {
		return t.registerErr
	}
	if t.options.OnRegistered != nil {
		t.options.OnRegistered()
	}
	if t.registered != nil {
		close(t.registered)
	}
	return nil
}

func (t *scriptedRegistrationTransaction) QualifyLoop(ctx context.Context) error {
	if !t.qualifyUntilClose {
		return t.qualifyErr
	}
	<-ctx.Done()
	return ctx.Err()
}

func (t *scriptedRegistrationTransaction) Unregister(ctx context.Context) error {
	t.unregisterOnce.Do(func() {
		if t.unregisterStarted != nil {
			close(t.unregisterStarted)
		}
	})
	if t.unregisterRelease != nil {
		select {
		case <-t.unregisterRelease:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func TestRegistrationBackoffCountsOnlyConsecutiveFailures(t *testing.T) {
	cfg := validTestSIPConfig()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scripts := []*scriptedRegistrationTransaction{
		{qualifyErr: errors.New("refresh failed")},
		{registerErr: errors.New("register failed")},
		{qualifyErr: errors.New("refresh failed again")},
	}
	var factoryMu sync.Mutex
	manager := &Manager{
		cfg:      cfg,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		endpoint: nil,
		cancel:   cancel,
	}
	manager.registrationFactory = func(_ context.Context, _ sip.Uri, options diago.RegisterOptions) (registrationTransaction, error) {
		factoryMu.Lock()
		defer factoryMu.Unlock()
		if len(scripts) == 0 {
			return nil, errors.New("unexpected registration attempt")
		}
		tx := scripts[0]
		scripts = scripts[1:]
		tx.options = options
		return tx, nil
	}
	var attempts []int
	manager.registrationDelay = func(attempt int) time.Duration {
		attempts = append(attempts, attempt)
		if len(attempts) == 3 {
			cancel()
		}
		return 0
	}
	manager.registrationLoop(ctx, nil, cfg)
	if len(attempts) != 3 || attempts[0] != 1 || attempts[1] != 2 || attempts[2] != 1 {
		t.Fatalf("consecutive failure attempts = %v, want [1 2 1]", attempts)
	}
}

func TestStopWaitsForUnregisterBeforeReturning(t *testing.T) {
	cfg := validTestSIPConfig()
	rootCtx, cancel := context.WithCancel(context.Background())
	registered := make(chan struct{})
	unregisterStarted := make(chan struct{})
	unregisterRelease := make(chan struct{})
	tx := &scriptedRegistrationTransaction{
		registered: registered, qualifyUntilClose: true,
		unregisterStarted: unregisterStarted, unregisterRelease: unregisterRelease,
	}
	manager := &Manager{
		cfg: cfg, logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cancel: cancel, rootCtx: rootCtx, state: StateRegistering,
	}
	manager.registrationFactory = func(_ context.Context, _ sip.Uri, options diago.RegisterOptions) (registrationTransaction, error) {
		tx.options = options
		return tx, nil
	}
	done := make(chan struct{})
	manager.registrationDone = done
	go func() {
		defer close(done)
		manager.registrationLoop(rootCtx, nil, cfg)
	}()
	<-registered

	stopped := make(chan error, 1)
	go func() { stopped <- manager.Stop(context.Background()) }()
	<-unregisterStarted
	select {
	case err := <-stopped:
		t.Fatalf("Stop returned before unregister completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(unregisterRelease)
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
}

func TestStopReportsForcedUnregisterTimeout(t *testing.T) {
	cfg := validTestSIPConfig()
	rootCtx, cancel := context.WithCancel(context.Background())
	registered := make(chan struct{})
	unregisterStarted := make(chan struct{})
	unregisterRelease := make(chan struct{})
	reported := make(chan string, 1)
	tx := &scriptedRegistrationTransaction{
		registered: registered, qualifyUntilClose: true,
		unregisterStarted: unregisterStarted, unregisterRelease: unregisterRelease,
	}
	manager := &Manager{
		cfg: cfg, logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cancel: cancel, rootCtx: rootCtx, state: StateRegistering,
		issueReporter: func(_ context.Context, fingerprint, _ string) { reported <- fingerprint },
	}
	manager.registrationFactory = func(_ context.Context, _ sip.Uri, options diago.RegisterOptions) (registrationTransaction, error) {
		tx.options = options
		return tx, nil
	}
	done := make(chan struct{})
	manager.registrationDone = done
	go func() {
		defer close(done)
		manager.registrationLoop(rootCtx, nil, cfg)
	}()
	<-registered

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	err := manager.Stop(stopCtx)
	stopCancel()
	if err == nil {
		t.Fatal("Stop unexpectedly succeeded while unregister was blocked")
	}
	select {
	case fingerprint := <-reported:
		if fingerprint != "sip_unregister_timeout" {
			t.Fatalf("operational issue = %q", fingerprint)
		}
	case <-time.After(time.Second):
		t.Fatal("unregister timeout was not reported")
	}
	close(unregisterRelease)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("registration goroutine did not exit after forced shutdown")
	}
}

func TestConnectionPassesBoundedContextToRegistration(t *testing.T) {
	cfg := validTestSIPConfig()
	manager := &Manager{cfg: cfg, endpoint: &diago.Diago{}}
	var observedDeadline time.Time
	manager.registrationFactory = func(ctx context.Context, _ sip.Uri, _ diago.RegisterOptions) (registrationTransaction, error) {
		var ok bool
		observedDeadline, ok = ctx.Deadline()
		if !ok {
			t.Fatal("SIP connection test context has no deadline")
		}
		return &scriptedRegistrationTransaction{registerErr: errors.New("stop registration test")}, nil
	}

	err := manager.TestConnection(context.Background())
	if err == nil || !strings.Contains(err.Error(), "stop registration test") {
		t.Fatalf("connection test error = %v", err)
	}
	remaining := time.Until(observedDeadline)
	if remaining <= 0 || remaining > sipConnectionTestTimeout {
		t.Fatalf("connection test deadline has unexpected remaining duration %s", remaining)
	}
}

func TestConnectionRejectsCanceledContextBeforePreparingRegistration(t *testing.T) {
	cfg := validTestSIPConfig()
	manager := &Manager{cfg: cfg, endpoint: &diago.Diago{}}
	created := false
	manager.registrationFactory = func(context.Context, sip.Uri, diago.RegisterOptions) (registrationTransaction, error) {
		created = true
		return &scriptedRegistrationTransaction{}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := manager.TestConnection(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled connection test error = %v, want context.Canceled", err)
	}
	if created {
		t.Fatal("canceled connection test prepared a registration transaction")
	}
}
