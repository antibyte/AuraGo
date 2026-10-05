package tools

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

	"aurago/internal/security"
)

func TestNormalizeSudoStderrRemovesLocalizedPrompt(t *testing.T) {
	stderr := "[sudo] Passwort fuer andi: permission denied"
	got := normalizeSudoStderr(stderr)
	if got != "permission denied" {
		t.Fatalf("normalizeSudoStderr() = %q, want %q", got, "permission denied")
	}
}

func TestNormalizeSudoStderrLeavesRegularErrors(t *testing.T) {
	stderr := "permission denied"
	got := normalizeSudoStderr(stderr)
	if got != stderr {
		t.Fatalf("normalizeSudoStderr() = %q, want %q", got, stderr)
	}
}

func TestSudoRunCommandNeverCarriesPasswordOnStdin(t *testing.T) {
	cmd := newSudoRunCommand("id", t.TempDir())
	if cmd.Stdin != nil {
		t.Fatal("sudo run command must not attach stdin")
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, " -n ") || strings.Contains(args, " -S") {
		t.Fatalf("sudo run must be non-interactive without -S, got %q", args)
	}
}

func TestSudoValidateCommandConsumesPasswordWithoutRunningACommand(t *testing.T) {
	cmd := newSudoValidateCommand(context.Background(), t.TempDir(), "hunter2-secret")
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
	onDrop      func()
}

func (f *fakeSudo) state() (valid bool, drops int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ticketValid, f.drops
}

func installFakeSudo(t *testing.T, f *fakeSudo) {
	t.Helper()
	if n := sudoTicketHolders(); n != 0 {
		t.Fatalf("sudo ticket holders = %d before the test, want 0", n)
	}
	validate, probe, drop, timeout := runSudoValidate, runSudoProbe, runSudoDrop, sudoValidateTimeout
	t.Cleanup(func() {
		runSudoValidate, runSudoProbe, runSudoDrop, sudoValidateTimeout = validate, probe, drop, timeout
	})
	runSudoValidate = func(ctx context.Context, dir, password string) ([]byte, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.validateErr != nil {
			return []byte(f.validateOut), f.validateErr
		}
		f.ticketValid = true
		return nil, nil
	}
	runSudoProbe = func(dir string) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.probeErr != nil {
			return f.probeErr
		}
		if !f.ticketValid {
			return errors.New("no ticket")
		}
		return nil
	}
	runSudoDrop = func() {
		f.mu.Lock()
		f.drops++
		f.ticketValid = false
		onDrop := f.onDrop
		f.mu.Unlock()
		if onDrop != nil {
			onDrop()
		}
	}
}

func sudoTicketHolders() int {
	sudoTicket.Lock()
	defer sudoTicket.Unlock()
	return sudoTicket.holders
}

func TestSudoTicketRefcount(t *testing.T) {
	t.Run("last holder drops the ticket", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		releaseA, _, errA := acquireSudoTicket(".", "pw-holder-a")
		releaseB, _, errB := acquireSudoTicket(".", "pw-holder-b")
		if errA != nil || errB != nil {
			t.Fatalf("acquire errors: %v, %v", errA, errB)
		}
		t.Cleanup(releaseA)
		t.Cleanup(releaseB)
		releaseA()
		if valid, drops := f.state(); drops != 0 || !valid {
			t.Fatalf("after first release: drops=%d valid=%v, want 0 drops and a valid ticket", drops, valid)
		}
		releaseB()
		if valid, drops := f.state(); drops != 1 || valid {
			t.Fatalf("after last release: drops=%d valid=%v, want 1 drop", drops, valid)
		}
		releaseB()
		releaseA()
		if _, drops := f.state(); drops != 1 || sudoTicketHolders() != 0 {
			t.Fatalf("double release: drops=%d holders=%d, want no-op", drops, sudoTicketHolders())
		}
	})

	t.Run("failed validation keeps holders and returns scrubbed output", func(t *testing.T) {
		const password = "refcount-secret-pw"
		f := &fakeSudo{}
		installFakeSudo(t, f)
		holder, _, err := acquireSudoTicket(".", "pw-existing-holder")
		if err != nil {
			t.Fatalf("acquire existing holder: %v", err)
		}
		defer holder()
		f.mu.Lock()
		f.validateErr = errors.New("exit status 1")
		f.validateOut = "Sorry, try again. " + password
		f.mu.Unlock()

		release, out, err := withSudoTicket(password, ".")
		if err == nil || release != nil || !strings.Contains(err.Error(), "sudo authentication failed") {
			t.Fatalf("withSudoTicket() = (release %v, err %v), want authentication failure", release != nil, err)
		}
		if strings.Contains(out, password) || !strings.Contains(out, "Sorry, try again.") {
			t.Fatalf("auth output = %q, want the sudo message with the password scrubbed", out)
		}
		if n := sudoTicketHolders(); n != 1 {
			t.Fatalf("holders = %d after failed validation, want 1", n)
		}
		if _, drops := f.state(); drops != 0 {
			t.Fatalf("drops = %d after failed validation, want 0", drops)
		}
	})

	t.Run("validation timeout is reported as a timeout", func(t *testing.T) {
		f := &fakeSudo{}
		installFakeSudo(t, f)
		sudoValidateTimeout = 20 * time.Millisecond
		runSudoValidate = func(ctx context.Context, dir, password string) ([]byte, error) {
			<-ctx.Done()
			return nil, errors.New("signal: killed")
		}
		_, _, err := acquireSudoTicket(".", "pw-timeout")
		if err == nil || !strings.Contains(err.Error(), "sudo authentication timed out") {
			t.Fatalf("err = %v, want sudo authentication timed out", err)
		}
		if _, drops := f.state(); drops != 0 || sudoTicketHolders() != 0 {
			t.Fatalf("drops=%d holders=%d after timeout, want 0/0", drops, sudoTicketHolders())
		}
	})

	t.Run("missing timestamp is reported before any command runs", func(t *testing.T) {
		f := &fakeSudo{probeErr: errors.New("exit status 1")}
		installFakeSudo(t, f)
		_, _, err := acquireSudoTicket(".", "pw-no-timestamp")
		if !errors.Is(err, errSudoTimestampDisabled) {
			t.Fatalf("err = %v, want errSudoTimestampDisabled", err)
		}
		if _, drops := f.state(); drops != 1 || sudoTicketHolders() != 0 {
			t.Fatalf("drops=%d holders=%d, want the unused ticket dropped and no holder", drops, sudoTicketHolders())
		}
	})

	t.Run("release drops the ticket before unregistering the password", func(t *testing.T) {
		const password = "ordering-secret-pw"
		scrubbedDuringDrop := false
		f := &fakeSudo{onDrop: func() {
			scrubbedDuringDrop = !strings.Contains(security.Scrub("out "+password), password)
		}}
		installFakeSudo(t, f)
		release, _, err := withSudoTicket(password, ".")
		if err != nil {
			t.Fatalf("withSudoTicket() error = %v", err)
		}
		if strings.Contains(security.Scrub(password), password) {
			t.Fatal("password must be registered with the scrubber while the ticket is held")
		}
		release()
		if !scrubbedDuringDrop {
			t.Fatal("password was unregistered before the ticket was dropped")
		}
		if !strings.Contains(security.Scrub(password), password) {
			t.Fatal("password must be unregistered after release")
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
					release, _, err := acquireSudoTicket(".", "pw-concurrent")
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
					release()
				}
			}()
		}
		wg.Wait()
		if lost.Load() != 0 || failed.Load() != 0 {
			t.Fatalf("ticket lost %d times, acquire failed %d times; a holder's ticket must stay valid", lost.Load(), failed.Load())
		}
		if valid, drops := f.state(); valid || drops == 0 || sudoTicketHolders() != 0 {
			t.Fatalf("after all releases: valid=%v drops=%d holders=%d, want a dropped ticket", valid, drops, sudoTicketHolders())
		}
	})
}
