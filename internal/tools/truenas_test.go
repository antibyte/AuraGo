package tools

import (
	"testing"
	"time"

	"aurago/internal/config"
)

func TestTrueNASRequestContextUsesConfiguredTimeout(t *testing.T) {
	ctx, cancel := truenasRequestContext(config.TrueNASConfig{RequestTimeout: 7})
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on TrueNAS request context")
	}
	remaining := time.Until(deadline)
	if remaining < 6*time.Second || remaining > 8*time.Second {
		t.Fatalf("remaining timeout = %v, want about 7s", remaining)
	}
}

func TestTrueNASRequestContextUsesDefaultTimeout(t *testing.T) {
	ctx, cancel := truenasRequestContext(config.TrueNASConfig{})
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on TrueNAS request context")
	}
	remaining := time.Until(deadline)
	if remaining < defaultTrueNASRequestTimeout-time.Second || remaining > defaultTrueNASRequestTimeout+time.Second {
		t.Fatalf("remaining timeout = %v, want about %v", remaining, defaultTrueNASRequestTimeout)
	}
}

func TestValidateTrueNASDatasetName(t *testing.T) {
	valid := []string{"tank", "tank/share", "tank/share/nested"}
	for _, name := range valid {
		if err := validateTrueNASDatasetName(name); err != nil {
			t.Errorf("validateTrueNASDatasetName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{"../escape", "tank/../escape", "/absolute", "/tank/share"}
	for _, name := range invalid {
		if err := validateTrueNASDatasetName(name); err == nil {
			t.Errorf("validateTrueNASDatasetName(%q) = nil, want error", name)
		}
	}
}

func TestValidateTrueNASSnapshotName(t *testing.T) {
	valid := []string{"aura-20260101-120000", "tank/share@snapshot"}
	for _, name := range valid {
		if err := validateTrueNASSnapshotName(name); err != nil {
			t.Errorf("validateTrueNASSnapshotName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{"../escape", "/absolute"}
	for _, name := range invalid {
		if err := validateTrueNASSnapshotName(name); err == nil {
			t.Errorf("validateTrueNASSnapshotName(%q) = nil, want error", name)
		}
	}
}

func TestValidateTrueNASPath(t *testing.T) {
	valid := []string{"tank/share", "/mnt/tank/share", "tank/share/nested"}
	for _, path := range valid {
		if err := validateTrueNASPath(path); err != nil {
			t.Errorf("validateTrueNASPath(%q) = %v, want nil", path, err)
		}
	}

	invalid := []string{"../escape", "tank/../escape", "../../etc/passwd"}
	for _, path := range invalid {
		if err := validateTrueNASPath(path); err == nil {
			t.Errorf("validateTrueNASPath(%q) = nil, want error", path)
		}
	}
}

func TestValidateTrueNASShareName(t *testing.T) {
	valid := []string{"tank_share", "MyShare", "backup-01"}
	for _, name := range valid {
		if err := validateTrueNASShareName(name); err != nil {
			t.Errorf("validateTrueNASShareName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{"../escape", "/absolute", "share/invalid"}
	for _, name := range invalid {
		if err := validateTrueNASShareName(name); err == nil {
			t.Errorf("validateTrueNASShareName(%q) = nil, want error", name)
		}
	}
}
