package tools

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestPackageManagerMutationsHonorRunPermissionsAndCancellation(t *testing.T) {
	// Even a regressed permission check must never find a real package manager.
	t.Setenv("PATH", t.TempDir())
	installFakeTicketSudo(t, &fakeTicketSudo{validateErr: errors.New("unexpected sudo authentication")})
	server := defaultRuntimePermissionsForTests()
	server.PackageManagerEnabled = true
	server.PackageManagerAllowInstall = true
	server.PackageManagerAllowRemove = true
	server.PackageManagerAllowUpgrade = true
	ConfigureRuntimePermissions(server)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })

	calls := map[string]func(context.Context, bool) (string, string, error){
		"install": func(ctx context.Context, sudo bool) (string, string, error) {
			return PackageManagerInstall(ctx, "brew", "test-only", sudo, "test-only")
		},
		"remove": func(ctx context.Context, sudo bool) (string, string, error) {
			return PackageManagerRemove(ctx, "brew", "test-only", sudo, "test-only")
		},
		"update": func(ctx context.Context, sudo bool) (string, string, error) {
			return PackageManagerUpdate(ctx, "brew", sudo, "test-only")
		},
		"upgrade": func(ctx context.Context, sudo bool) (string, string, error) {
			return PackageManagerUpgrade(ctx, "brew", "test-only", sudo, "test-only")
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			for _, restriction := range []string{"disabled", "readonly", "operation"} {
				run := server
				switch restriction {
				case "disabled":
					run.PackageManagerEnabled = false
				case "readonly":
					run.PackageManagerReadOnly = true
				case "operation":
					run.PackageManagerAllowInstall = false
					run.PackageManagerAllowRemove = false
					run.PackageManagerAllowUpgrade = false
				}
				ctx := WithRuntimePermissions(context.Background(), run)
				if _, _, err := call(ctx, false); err == nil || !strings.Contains(err.Error(), "runtime permissions") {
					t.Fatalf("%s: error=%v, want permission denial before starting a command", restriction, err)
				}
			}
			ctx, cancel := context.WithCancel(WithRuntimePermissions(context.Background(), server))
			cancel()
			if _, _, err := call(ctx, false); !errors.Is(err, context.Canceled) {
				t.Fatalf("error=%v, want cancelled", err)
			}
			run := server
			run.AllowShell = false
			ctx = WithRuntimePermissions(context.Background(), run)
			if _, _, err := call(ctx, true); err == nil || !strings.Contains(err.Error(), "shell execution is disabled") {
				t.Fatalf("sudo error=%v, want the run's shell denial", err)
			}
		})
	}
}

func TestPackageManagerCommandHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cmd := exec.Command(os.Args[0], "-test.run=TestBackgroundProcessSupervisorHelper", "--", "sleep")
	cmd.Env = append(os.Environ(), "GO_WANT_BACKGROUND_HELPER=1")
	if _, _, err := runPackageManagerCommand(ctx, cmd); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want the caller's deadline", err)
	}
}
