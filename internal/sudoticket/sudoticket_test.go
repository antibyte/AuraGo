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

// fakeSudo stands in for the sudo processes behind the shared ticket.
type fakeSudo struct {
	mu          sync.Mutex
	ticketValid bool
	drops       int
	validateOut string
	validateErr error
	probeErr    error
}

func (f *fakeSudo) state() (valid bool, drops int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ticketValid, f.drops
}

func installFakeSudo(t *testing.T, f *fakeSudo) {
	t.Helper()
	if n := holders(); n != 0 {
		t.Fatalf("sudo ticket holders = %d before the test, want 0", n)
	}
	timeout := validateTimeout
	restore := ReplaceProcessesForTesting(Processes{
		Validate: func(ctx context.Context, dir, password string) ([]byte, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.validateErr != nil {
				return []byte(f.validateOut), f.validateErr
			}
			f.ticketValid = true
			return nil, nil
		},
		Probe: func(ctx context.Context, dir string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.probeErr != nil {
				return f.probeErr
			}
			if !f.ticketValid {
				return errors.New("no ticket")
			}
			return nil
		},
		Drop: func() {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.drops++
			f.ticketValid = false
		},
	})
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

	t.Run("failed validation keeps holders and returns sudo output", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		holder, _, err := Acquire(context.Background(), ".", "pw-existing-holder")
		if err != nil {
			t.Fatalf("acquire existing holder: %v", err)
		}
		defer holder.Release()
		f.mu.Lock()
		f.validateErr = errors.New("exit status 1")
		f.validateOut = "Sorry, try again."
		f.mu.Unlock()

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
		restore := ReplaceProcessesForTesting(Processes{
			Validate: func(ctx context.Context, dir, password string) ([]byte, error) {
				<-ctx.Done()
				return nil, errors.New("signal: killed")
			},
			Probe: func(context.Context, string) error { return nil },
			Drop:  func() { f.mu.Lock(); f.drops++; f.mu.Unlock() },
		})
		defer restore()
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
		restore := ReplaceProcessesForTesting(Processes{
			Validate: func(ctx context.Context, dir, password string) ([]byte, error) {
				<-ctx.Done()
				return nil, errors.New("signal: killed")
			},
			Probe: func(context.Context, string) error { return nil },
			Drop:  func() { f.mu.Lock(); f.drops++; f.mu.Unlock() },
		})
		defer restore()
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

	t.Run("probe failure is advisory and explains only a failed run", func(t *testing.T) {
		// Defaults verifypw=always refuses sudo -n -v even with a fresh
		// ticket, while sudo -n <command> still runs under it.
		f := &fakeSudo{probeErr: errors.New("exit status 1")}
		installFakeSudo(t, f)
		lease, _, err := Acquire(context.Background(), ".", "pw-verifypw-always")
		if err != nil || lease == nil {
			t.Fatalf("Acquire() = (lease %v, err %v), want a lease despite the failed probe", lease != nil, err)
		}
		if valid, drops := f.state(); !valid || drops != 0 || holders() != 1 {
			t.Fatalf("valid=%v drops=%d holders=%d, want the ticket kept for the run", valid, drops, holders())
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

	t.Run("probe failure while another call holds the ticket must not drop it", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		holder, _, err := Acquire(context.Background(), ".", "pw-holder")
		if err != nil {
			t.Fatalf("acquire holder: %v", err)
		}
		defer holder.Release()
		f.mu.Lock()
		f.probeErr = errors.New("exit status 1")
		f.mu.Unlock()

		lease, _, err := Acquire(context.Background(), ".", "pw-second")
		if err != nil {
			t.Fatalf("second Acquire() error = %v, want a lease despite the failed probe", err)
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
