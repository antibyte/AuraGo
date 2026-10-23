package phonon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/sandbox"
	"aurago/internal/speechlab"
)

// shortcut: serialize CPU synthesis, use a bounded pool if measured concurrency requires it.
var synthesisSlot = make(chan struct{}, 1)

func ValidateText(text string) error {
	if strings.TrimSpace(text) == "" || len([]rune(text)) > 2000 {
		return errors.New("Phonon requires 1–2000 characters")
	}
	return nil
}

func Synthesize(ctx context.Context, root, dataDir, language, voiceID, text string) ([]byte, error) {
	if err := ValidateText(text); err != nil {
		return nil, err
	}
	lang := Language(language)
	if lang == "" {
		return nil, errors.New("unsupported Phonon language")
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	select {
	case synthesisSlot <- struct{}{}:
		defer func() { <-synthesisSlot }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := Ready(root); err != nil {
		return nil, err
	}
	voice, err := VoicePath(root, dataDir, voiceID)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "aurago-phonon-*")
	if err != nil {
		return nil, fmt.Errorf("create Phonon output: %w", err)
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "speech.wav")
	bin, err := filepath.Abs(Binary(root))
	if err != nil {
		return nil, err
	}
	model, err := filepath.Abs(filepath.Join(root, "model"))
	if err != nil {
		return nil, err
	}
	catalog, err := filepath.Abs(filepath.Join(root, "voices.jsonl"))
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, "--model", model, "--quant", "q8", "--device", "cpu", "--lang", lang,
		"--voice", voice, "--voice-catalog", catalog, "--threads", "2", "-o", out, "--", text)
	cmd.Dir = dir
	cmd.Env = append(sandbox.FilterEnv(os.Environ()), "HF_HUB_OFFLINE=1", "OMP_NUM_THREADS=2")
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("Phonon CPU synthesis failed: %w", err)
	}
	info, err := os.Lstat(out)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 44 || info.Size() > 16<<20 {
		return nil, errors.New("Phonon returned missing or invalid audio")
	}
	wav, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("read Phonon audio: %w", err)
	}
	if err := speechlab.ValidateWAV(wav); err != nil {
		return nil, fmt.Errorf("Phonon returned invalid WAV: %w", err)
	}
	return wav, nil
}
