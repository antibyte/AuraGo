package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"aurago/internal/config"
	"aurago/internal/sanotts"
)

// ponytail: one CPU synthesis at a time; use a bounded pool if concurrent calls become necessary.
var sanoTTSSlot = make(chan struct{}, 1)

func ttsSano(cfg TTSConfig, text string) ([]byte, error) {
	ctx := ttsRequestContext(cfg)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case sanoTTSSlot <- struct{}{}:
		defer func() { <-sanoTTSSlot }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	root := filepath.Join(cfg.DataDir, "sanotts")
	bin, err := ensureSanoTTS(ctx, root)
	if err != nil {
		return nil, err
	}
	return sanotts.Synthesize(ctx, bin, sanotts.Voice(cfg.Language), text, filepath.Join(root, "voices"))
}

// Keep this runtime separate from agent-editable skills and their dependencies.
func ensureSanoTTS(ctx context.Context, root string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	bin := filepath.Join(filepath.Dir(GetPythonBin(root)), "sanotts")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	marker := filepath.Join(root, "revision")
	if data, err := os.ReadFile(marker); err == nil && string(data) == sanotts.Revision {
		if _, err := os.Stat(bin); err == nil {
			return bin, nil
		}
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", fmt.Errorf("create sanoTTS runtime: %w", err)
	}
	if err := EnsureVenv(root, slog.Default()); err != nil {
		return "", fmt.Errorf("sanoTTS needs Python 3.10+ with venv and pip: %w", err)
	}
	if err := config.WriteFileAtomic(filepath.Join(root, "LICENSE.MIT"), sanotts.License, 0o600); err != nil {
		return "", fmt.Errorf("write sanoTTS license: %w", err)
	}
	wheel := filepath.Join(root, sanotts.WheelName)
	if err := config.WriteFileAtomic(wheel, sanotts.Wheel, 0o600); err != nil {
		return "", fmt.Errorf("stage sanoTTS runtime: %w", err)
	}
	defer os.Remove(wheel)
	slog.Info("[sanoTTS] Installing local CPU runtime", "revision", sanotts.Revision)
	cmd := exec.CommandContext(ctx, GetPythonBin(root), "-I", "-m", "pip", "install", "--disable-pip-version-check", "--no-input", "--force-reinstall", wheel)
	ensureFilteredEnv(cmd)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("install sanoTTS dependencies (internet required on first use): %w", err)
	}
	if err := config.WriteFileAtomic(marker, []byte(sanotts.Revision), 0o600); err != nil {
		return "", fmt.Errorf("record sanoTTS runtime revision: %w", err)
	}
	return bin, nil
}
