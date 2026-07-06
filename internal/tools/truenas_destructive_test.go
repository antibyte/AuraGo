package tools

import (
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestTrueNASPoolScrubRequiresAllowDestructive(t *testing.T) {
	cfg := config.TrueNASConfig{Enabled: true, ReadOnly: false, AllowDestructive: false}
	got := TrueNASPoolScrub(cfg, 1, slog.Default())
	if !strings.Contains(got, "allow_destructive: false") {
		t.Fatalf("TrueNASPoolScrub = %s, want allow_destructive denial", got)
	}
}

func TestTrueNASSMBDeleteRequiresAllowDestructive(t *testing.T) {
	cfg := config.TrueNASConfig{Enabled: true, ReadOnly: false, AllowDestructive: false}
	got := TrueNASSMBDelete(cfg, 1, slog.Default())
	if !strings.Contains(got, "allow_destructive: false") {
		t.Fatalf("TrueNASSMBDelete = %s, want allow_destructive denial", got)
	}
}

func TestTrueNASNFSDeleteRequiresAllowDestructive(t *testing.T) {
	cfg := config.TrueNASConfig{Enabled: true, ReadOnly: false, AllowDestructive: false}
	got := TrueNASNFSDelete(cfg, 1, slog.Default())
	if !strings.Contains(got, "allow_destructive: false") {
		t.Fatalf("TrueNASNFSDelete = %s, want allow_destructive denial", got)
	}
}

func TestDispatchTrueNASToolRoutesNFSDelete(t *testing.T) {
	cfg := &config.Config{}
	cfg.TrueNAS.Enabled = true
	cfg.TrueNAS.AllowDestructive = false

	got := DispatchTrueNASTool("truenas_nfs_delete", map[string]string{"share_id": "1"}, cfg, slog.Default())
	if !strings.Contains(got, "allow_destructive: false") {
		t.Fatalf("DispatchTrueNASTool(truenas_nfs_delete) = %s, want allow_destructive denial", got)
	}
}
