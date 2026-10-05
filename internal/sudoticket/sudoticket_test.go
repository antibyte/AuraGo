package sudoticket

import (
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateCommandConsumesPasswordWithoutRunningACommand(t *testing.T) {
	cmd := newValidateCommand(context.Background(), t.TempDir(), "hunter2-secret")
	if cmd.Stdin == nil {
		t.Fatal("validate command must feed the password on stdin")
	}
	stdin, err := io.ReadAll(cmd.Stdin)
	if err != nil || string(stdin) != "hunter2-secret\n" {
		t.Fatalf("validate stdin = %q (err %v), want the password line", stdin, err)
	}
	args := strings.Join(cmd.Args, " ")
	for _, flag := range []string{" -S ", " -v"} {
		if !strings.Contains(args, flag) {
			t.Fatalf("validate command missing %q in %q", strings.TrimSpace(flag), args)
		}
	}
	emptyPrompt := false
	for i, arg := range cmd.Args[:len(cmd.Args)-1] {
		if arg == "-p" && cmd.Args[i+1] == "" {
			emptyPrompt = true
		}
	}
	if !emptyPrompt {
		t.Fatalf("validate command must pass an empty prompt (-p \"\"): %q", cmd.Args)
	}
	// sudo -k combined with -v authenticates but never writes the timestamp,
	// so the following sudo -n would always fail with "a password is required".
	if strings.Contains(args, " -k") {
		t.Fatalf("validate command must not pass -k: %q", args)
	}
	if strings.Contains(args, "/bin/sh") {
		t.Fatalf("validate command must not run a shell: %q", args)
	}
}

// fakeSudo stands in for the sudo processes behind the shared ticket. By
// default the host needs a password and keeps a validated ticket.
type fakeSudo struct {
	mu                sync.Mutex
	ticketValid       bool
	nopasswd          bool // a NOPASSWD rule: sudo -n runs without a ticket
	timestampDisabled bool // timestamp_timeout=0: validation leaves no ticket
	verifypwAlways    bool // Defaults verifypw=always: sudo -n -v always fails
	validates         int
	probes            int
	drops             int
	validateOut       string
	validateErr       error
	passwordlessErr   error // forces sudo -n true to fail
	probeErr          error // forces sudo -n -v to fail
	onProbe           func()
}

func (f *fakeSudo) state() (valid bool, drops int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ticketValid, f.drops
}

func (f *fakeSudo) calls() (validates, probes int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.validates, f.probes
}

func (f *fakeSudo) set(update func(*fakeSudo)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	update(f)
}

func (f *fakeSudo) processes() Processes {
	return Processes{
		Passwordless: func(ctx context.Context, dir string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			switch {
			case f.passwordlessErr != nil:
				return f.passwordlessErr
			case f.nopasswd || f.ticketValid:
				return nil
			}
			return errors.New("sudo: a password is required")
		},
		Validate: func(ctx context.Context, dir, password string) ([]byte, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.validates++
			if f.validateErr != nil {
				return []byte(f.validateOut), f.validateErr
			}
			if !f.timestampDisabled {
				f.ticketValid = true
			}
			return nil, nil
		},
		Probe: func(ctx context.Context, dir string) error {
			f.mu.Lock()
			f.probes++
			onProbe := f.onProbe
			f.mu.Unlock()
			if onProbe != nil {
				onProbe()
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			switch {
			case f.probeErr != nil:
				return f.probeErr
			case f.verifypwAlways:
				return errors.New("sudo: a password is required")
			case f.nopasswd || f.ticketValid:
				return nil
			}
			return errors.New("sudo: a password is required")
		},
		Drop: func() {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.drops++
			f.ticketValid = false
		},
	}
}

func installFakeSudo(t *testing.T, f *fakeSudo) {
	t.Helper()
	if n := holders(); n != 0 {
		t.Fatalf("sudo ticket holders = %d before the test, want 0", n)
	}
	timeout := validateTimeout
	restore := ReplaceProcessesForTesting(f.processes())
	t.Cleanup(func() {
		restore()
		validateTimeout = timeout
	})
}

func holders() int {
	ticket.Lock()
	defer ticket.Unlock()
	return ticket.holders
}

func TestSudoTicketRefcount(t *testing.T) {
	t.Run("last holder drops the ticket", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		leaseA, _, errA := Acquire(context.Background(), ".", "pw-holder-a")
		leaseB, _, errB := Acquire(context.Background(), ".", "pw-holder-b")
		if errA != nil || errB != nil {
			t.Fatalf("acquire errors: %v, %v", errA, errB)
		}
		t.Cleanup(leaseA.Release)
		t.Cleanup(leaseB.Release)
		leaseA.Release()
		if valid, drops := f.state(); drops != 0 || !valid {
			t.Fatalf("after first release: drops=%d valid=%v, want 0 drops and a valid ticket", drops, valid)
		}
		leaseB.Release()
		if valid, drops := f.state(); drops != 1 || valid {
			t.Fatalf("after last release: drops=%d valid=%v, want 1 drop", drops, valid)
		}
		leaseB.Release()
		leaseA.Release()
		if _, drops := f.state(); drops != 1 || holders() != 0 {
			t.Fatalf("double release: drops=%d holders=%d, want no-op", drops, holders())
		}
	})

	t.Run("nil lease release is a no-op", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		var lease *Lease
		lease.Release()
		if _, drops := f.state(); drops != 0 || holders() != 0 {
			t.Fatalf("nil release: drops=%d holders=%d, want 0/0", drops, holders())
		}
	})

	t.Run("passwordless sudo skips the validation but counts as a holder", func(t *testing.T) {
		// A NOPASSWD rule: no PAM call, and a stale Vault password is ignored.
		f := &fakeSudo{nopasswd: true, validateErr: errors.New("exit status 1")}
		installFakeSudo(t, f)
		lease, _, err := Acquire(context.Background(), ".", "pw-stale")
		if err != nil || lease == nil {
			t.Fatalf("Acquire() = (lease %v, err %v), want a lease without validation", lease != nil, err)
		}
		if validates, probes := f.calls(); validates != 0 || probes != 0 || holders() != 1 {
			t.Fatalf("validates=%d probes=%d holders=%d, want no validation, no probe and one holder", validates, probes, holders())
		}
		if got := lease.Explain(errors.New("exit status 1")); errors.Is(got, ErrTimestampDisabled) {
			t.Fatalf("Explain() = %v, a passwordless lease carries no timestamp hint", got)
		}
		lease.Release()
		if _, drops := f.state(); drops != 1 || holders() != 0 {
			t.Fatalf("drops=%d holders=%d after release, want the last release to drop", drops, holders())
		}
	})

	t.Run("password required validates as before", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		lease, _, err := Acquire(context.Background(), ".", "pw-required")
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		defer lease.Release()
		if validates, probes := f.calls(); validates != 1 || probes != 1 {
			t.Fatalf("validates=%d probes=%d, want one validation and one probe", validates, probes)
		}
	})

	t.Run("fast-path holder keeps the ticket another caller validated", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		first, _, err := Acquire(context.Background(), ".", "pw-first")
		if err != nil {
			t.Fatalf("first Acquire() error = %v", err)
		}
		t.Cleanup(first.Release)
		second, _, err := Acquire(context.Background(), ".", "pw-second")
		if err != nil {
			t.Fatalf("second Acquire() error = %v", err)
		}
		t.Cleanup(second.Release)
		if validates, _ := f.calls(); validates != 1 || holders() != 2 {
			t.Fatalf("validates=%d holders=%d, want the second caller on the fast path as a holder", validates, holders())
		}
		first.Release()
		if valid, drops := f.state(); !valid || drops != 0 {
			t.Fatalf("after the validating caller released: valid=%v drops=%d, the fast-path holder's ticket must survive", valid, drops)
		}
		second.Release()
		if valid, drops := f.state(); valid || drops != 1 {
			t.Fatalf("after the last release: valid=%v drops=%d, want 1 drop", valid, drops)
		}
	})

	t.Run("failed validation keeps holders and returns sudo output", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		holder, _, err := Acquire(context.Background(), ".", "pw-existing-holder")
		if err != nil {
			t.Fatalf("acquire existing holder: %v", err)
		}
		defer holder.Release()
		// The holder's ticket expired (timestamp_timeout) while it still runs.
		f.set(func(f *fakeSudo) {
			f.ticketValid = false
			f.validateErr = errors.New("exit status 1")
			f.validateOut = "Sorry, try again."
		})

		lease, out, err := Acquire(context.Background(), ".", "pw-wrong")
		if err == nil || lease != nil || !strings.Contains(err.Error(), "sudo authentication failed") {
			t.Fatalf("Acquire() = (lease %v, err %v), want authentication failure", lease != nil, err)
		}
		if out != "Sorry, try again." {
			t.Fatalf("auth output = %q, want the sudo message", out)
		}
		if n := holders(); n != 1 {
			t.Fatalf("holders = %d after failed validation, want 1", n)
		}
		if _, drops := f.state(); drops != 0 {
			t.Fatalf("drops = %d after failed validation, want 0", drops)
		}
	})

	t.Run("validation timeout is reported as a timeout", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		validateTimeout = 20 * time.Millisecond
		p := f.processes()
		p.Validate = func(ctx context.Context, dir, password string) ([]byte, error) {
			<-ctx.Done()
			return nil, errors.New("signal: killed")
		}
		defer ReplaceProcessesForTesting(p)()
		_, _, err := Acquire(context.Background(), ".", "pw-timeout")
		if err == nil || !strings.Contains(err.Error(), "sudo authentication timed out") {
			t.Fatalf("err = %v, want sudo authentication timed out", err)
		}
		if _, drops := f.state(); drops != 0 || holders() != 0 {
			t.Fatalf("drops=%d holders=%d after timeout, want 0/0", drops, holders())
		}
	})

	t.Run("caller cancellation stops the validation", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		p := f.processes()
		p.Validate = func(ctx context.Context, dir, password string) ([]byte, error) {
			<-ctx.Done()
			return nil, errors.New("signal: killed")
		}
		defer ReplaceProcessesForTesting(p)()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, err := Acquire(ctx, ".", "pw-canceled")
		if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "sudo authentication canceled") {
			t.Fatalf("err = %v, want sudo authentication canceled", err)
		}
		if holders() != 0 {
			t.Fatalf("holders = %d after cancellation, want 0", holders())
		}
	})

	t.Run("disabled timestamps explain only a failed run", func(t *testing.T) {
		f := &fakeSudo{timestampDisabled: true}
		installFakeSudo(t, f)
		lease, _, err := Acquire(context.Background(), ".", "pw-no-timestamp")
		if err != nil || lease == nil {
			t.Fatalf("Acquire() = (lease %v, err %v), want a lease despite the failed probe", lease != nil, err)
		}
		if n := holders(); n != 1 {
			t.Fatalf("holders = %d, want the lease counted", n)
		}
		if got := lease.Explain(nil); got != nil {
			t.Fatalf("Explain(nil) = %v, want nil for a successful run", got)
		}
		runErr := errors.New("exit status 1")
		explained := lease.Explain(runErr)
		if !errors.Is(explained, ErrTimestampDisabled) || !errors.Is(explained, runErr) {
			t.Fatalf("Explain(runErr) = %v, want both the run error and ErrTimestampDisabled", explained)
		}
		lease.Release()
		if _, drops := f.state(); drops != 1 || holders() != 0 {
			t.Fatalf("drops=%d holders=%d after release, want the ticket dropped", drops, holders())
		}
	})

	t.Run("verifypw=always probe failure is cleared by sudo -n true", func(t *testing.T) {
		// sudo -n -v is refused although the ticket covers commands.
		f := &fakeSudo{verifypwAlways: true}
		installFakeSudo(t, f)
		lease, _, err := Acquire(context.Background(), ".", "pw-verifypw-always")
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		defer lease.Release()
		if _, probes := f.calls(); probes != 1 {
			t.Fatalf("probes = %d, want the probe to have run", probes)
		}
		runErr := errors.New("exit status 2")
		if got := lease.Explain(runErr); got != runErr {
			t.Fatalf("Explain(runErr) = %v, want the run error unchanged", got)
		}
	})

	t.Run("cancelled probe carries no hint", func(t *testing.T) {
		f := &fakeSudo{timestampDisabled: true}
		installFakeSudo(t, f)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f.set(func(f *fakeSudo) { f.onProbe = cancel })
		lease, _, err := Acquire(ctx, ".", "pw-cancelled-probe")
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		defer lease.Release()
		runErr := context.Canceled
		if got := lease.Explain(runErr); got != runErr {
			t.Fatalf("Explain(runErr) = %v, a cancelled probe must not attach ErrTimestampDisabled", got)
		}
	})

	t.Run("probe failure while another call holds the ticket must not drop it", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		holder, _, err := Acquire(context.Background(), ".", "pw-holder")
		if err != nil {
			t.Fatalf("acquire holder: %v", err)
		}
		defer holder.Release()
		f.set(func(f *fakeSudo) {
			f.passwordlessErr = errors.New("exit status 1")
			f.probeErr = errors.New("exit status 1")
		})

		lease, _, err := Acquire(context.Background(), ".", "pw-second")
		if err != nil {
			t.Fatalf("second Acquire() error = %v, want a lease despite the failed probe", err)
		}
		if !errors.Is(lease.Explain(errors.New("exit status 1")), ErrTimestampDisabled) {
			t.Fatal("the confirmed probe failure must be explained on a failed run")
		}
		if valid, drops := f.state(); !valid || drops != 0 || holders() != 2 {
			t.Fatalf("valid=%v drops=%d holders=%d, the holder's ticket must survive", valid, drops, holders())
		}
		lease.Release()
		if valid, drops := f.state(); !valid || drops != 0 || holders() != 1 {
			t.Fatalf("after second release: valid=%v drops=%d holders=%d, want the holder's ticket kept", valid, drops, holders())
		}
		holder.Release()
		if _, drops := f.state(); drops != 1 || holders() != 0 {
			t.Fatalf("after last release: drops=%d holders=%d, want 1 drop", drops, holders())
		}
	})

	t.Run("successful probe leaves run errors unchanged", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		lease, _, err := Acquire(context.Background(), ".", "pw-probe-ok")
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		defer lease.Release()
		runErr := errors.New("exit status 2")
		if got := lease.Explain(runErr); got != runErr {
			t.Fatalf("Explain(runErr) = %v, want the run error unchanged", got)
		}
		var none *Lease
		if got := none.Explain(runErr); got != runErr {
			t.Fatalf("nil Lease Explain(runErr) = %v, want the run error unchanged", got)
		}
	})

	t.Run("concurrent holders never lose the ticket", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		var wg sync.WaitGroup
		var lost, failed atomic.Int32
		for i := 0; i < 64; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 3; j++ {
					lease, _, err := Acquire(context.Background(), ".", "pw-concurrent")
					if err != nil {
						failed.Add(1)
						continue
					}
					if valid, _ := f.state(); !valid {
						lost.Add(1)
					}
					runtime.Gosched()
					if valid, _ := f.state(); !valid {
						lost.Add(1)
					}
					lease.Release()
				}
			}()
		}
		wg.Wait()
		if lost.Load() != 0 || failed.Load() != 0 {
			t.Fatalf("ticket lost %d times, acquire failed %d times; a holder's ticket must stay valid", lost.Load(), failed.Load())
		}
		if valid, drops := f.state(); valid || drops == 0 || holders() != 0 {
			t.Fatalf("after all releases: valid=%v drops=%d holders=%d, want a dropped ticket", valid, drops, holders())
		}
	})
}
