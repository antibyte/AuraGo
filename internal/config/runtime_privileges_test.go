package config

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestApplyRuntimePrivilegeProbes(t *testing.T) {
	tests := []struct {
		name                string
		noNewPrivileges     bool
		protectSystemStrict bool
	}{
		{name: "unrestricted", noNewPrivileges: false, protectSystemStrict: false},
		{name: "both restrictions", noNewPrivileges: true, protectSystemStrict: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := Runtime{}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			applyRuntimePrivilegeProbes(&rt, logger, runtimePrivilegeProbes{
				noNewPrivileges:     func() bool { return tt.noNewPrivileges },
				protectSystemStrict: func() bool { return tt.protectSystemStrict },
			})

			if rt.NoNewPrivileges != tt.noNewPrivileges {
				t.Fatalf("NoNewPrivileges = %v, want %v", rt.NoNewPrivileges, tt.noNewPrivileges)
			}
			if rt.ProtectSystemStrict != tt.protectSystemStrict {
				t.Fatalf("ProtectSystemStrict = %v, want %v", rt.ProtectSystemStrict, tt.protectSystemStrict)
			}
		})
	}
}

func TestComputeFeatureAvailabilityBlocksUnrestrictedSudoWhenProtectSystemStrict(t *testing.T) {
	availability := ComputeFeatureAvailability(Runtime{ProtectSystemStrict: true}, true)
	sudoUnrestricted := availability["sudo_unrestricted"]

	if sudoUnrestricted.Available {
		t.Fatal("sudo_unrestricted should be unavailable while ProtectSystem=strict is active")
	}
	if !strings.Contains(sudoUnrestricted.Reason, "ProtectSystem=strict") {
		t.Fatalf("sudo_unrestricted reason = %q, want ProtectSystem guidance", sudoUnrestricted.Reason)
	}
}

func TestIsReadOnlyFilesystemError(t *testing.T) {
	if !isReadOnlyFilesystemError(&os.PathError{Op: "open", Path: "/etc/test", Err: syscall.EROFS}) {
		t.Fatal("EROFS should be recognized as a read-only filesystem error")
	}
	if isReadOnlyFilesystemError(errors.New("permission denied")) {
		t.Fatal("permission denied should not be treated as a read-only filesystem")
	}
}
