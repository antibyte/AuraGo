// Package sanotts shares the CPU-only sanoTTS CLI with chat and CYD audio.
package sanotts

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/i18n"
	"aurago/internal/sandbox"
)

const Revision = "de3f71a8603ee979a74e6f2a7592a0ce795d608b"
const WheelName = "sanotts-0.6.0-py3-none-any.whl"

//go:embed runtime/sanotts-0.6.0-py3-none-any.whl
var Wheel []byte

//go:embed runtime/LICENSE.MIT
var License []byte

// Voice selects the smallest published Python voice. The three browser-only
// languages (hi/ne/zh) intentionally use English until Python packs exist.
func Voice(language string) string {
	lang := strings.ToLower(strings.TrimSpace(language))
	lang = strings.Split(strings.ReplaceAll(lang, "_", "-"), "-")[0]
	switch lang {
	case "ar", "id", "vi", "fr":
		return lang
	case "cs", "de", "es", "it", "pt", "ro", "ru", "tr":
		return lang + "-tiny"
	}
	switch lang = i18n.NormalizeLang(lang); lang {
	case "cs", "de", "es", "it", "pt":
		return lang + "-tiny"
	case "fr":
		return lang
	default:
		return "heart-nano"
	}
}

// Synthesize renders a private temporary WAV; cacheDir stores models, never text.
func Synthesize(ctx context.Context, bin, voice, text, cacheDir string) ([]byte, error) {
	if strings.TrimSpace(text) == "" || len([]rune(text)) > 2000 {
		return nil, fmt.Errorf("sanoTTS requires 1–2000 characters")
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "aurago-sanotts-*")
	if err != nil {
		return nil, fmt.Errorf("create sanoTTS output directory: %w", err)
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "speech.wav")
	args := []string{"say", "--voice", voice, "--nano-g2p", "lexicon", "-o", out}
	if cacheDir != "" {
		args = append(args, "--cache-dir", cacheDir)
	}
	// The separator prevents text starting with '-' from becoming a CLI option.
	args = append(args, "--", text)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(sandbox.FilterEnv(os.Environ()), "OPENBLAS_NUM_THREADS=1", "OMP_NUM_THREADS=1")
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("sanoTTS synthesis failed: %w", err)
	}
	info, err := os.Stat(out)
	if err != nil || info.Size() < 44 || info.Size() > 16<<20 {
		return nil, fmt.Errorf("sanoTTS returned missing or invalid audio")
	}
	wav, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("read sanoTTS audio: %w", err)
	}
	if string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, fmt.Errorf("sanoTTS returned invalid WAV")
	}
	return wav, nil
}
