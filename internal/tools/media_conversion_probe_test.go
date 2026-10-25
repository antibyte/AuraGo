package tools

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestMediaConversionOutputProcess(t *testing.T) {
	size, err := strconv.Atoi(os.Getenv("AURAGO_TEST_CONVERTER_OUTPUT_SIZE"))
	if err != nil {
		return
	}
	prefix := `{"format":{"format_name":"wav","tags":{"padding":"`
	suffix := `"}},"streams":[{"codec_type":"audio","codec_name":"pcm_s16le"}]}`
	fmt.Print(prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix)
	if os.Getenv("AURAGO_TEST_CONVERTER_FAIL") == "1" {
		fmt.Fprint(os.Stderr, "converter failed")
		os.Exit(1)
	}
	os.Exit(0)
}

func TestProbeMediaMetadataOutputLimit(t *testing.T) {
	run := mediaConversionRunCommand
	lookPath := mediaConversionLookPath
	t.Cleanup(func() { mediaConversionRunCommand, mediaConversionLookPath = run, lookPath })
	mediaConversionLookPath = func(string) (string, error) { return os.Args[0], nil }
	mediaConversionRunCommand = func(ctx context.Context, _ string, args []string) ([]byte, error) {
		return run(ctx, os.Args[0], []string{"-test.run=^TestMediaConversionOutputProcess$"})
	}
	for _, size := range []int{(1 << 20) + 1, mediaConversionOutputLimit, mediaConversionOutputLimit + 1} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			t.Setenv("AURAGO_TEST_CONVERTER_OUTPUT_SIZE", strconv.Itoa(size))
			result, err := probeMediaMetadata(context.Background(), nil, "test.wav")
			if size > mediaConversionOutputLimit {
				if err == nil || !strings.Contains(err.Error(), "output exceeds 4194304 bytes") {
					t.Fatalf("expected explicit output limit error, got %v", err)
				}
			} else if err != nil || result.InputFormat != "wav" || result.AudioCodec != "pcm_s16le" {
				t.Fatalf("valid JSON was not retained in full: %+v: %v", result, err)
			}
		})
	}
}

func TestMediaConversionRunCommandPreservesErrors(t *testing.T) {
	args := []string{"-test.run=^TestMediaConversionOutputProcess$"}
	t.Setenv("AURAGO_TEST_CONVERTER_OUTPUT_SIZE", strconv.Itoa(mediaConversionOutputLimit+1))
	t.Setenv("AURAGO_TEST_CONVERTER_FAIL", "1")
	if _, err := mediaConversionRunCommand(context.Background(), os.Args[0], args); err == nil || !strings.Contains(err.Error(), "converter failed") {
		t.Fatalf("process failure was masked by the output limit: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := mediaConversionRunCommand(ctx, os.Args[0], args); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := mediaConversionRunCommand(ctx, os.Args[0], args); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline lost: %v", err)
	}
}

func TestProbeMediaMetadataArguments(t *testing.T) {
	run := mediaConversionRunCommand
	lookPath := mediaConversionLookPath
	t.Cleanup(func() { mediaConversionRunCommand, mediaConversionLookPath = run, lookPath })
	mediaConversionLookPath = func(string) (string, error) { return os.Args[0], nil }
	mediaConversionRunCommand = func(_ context.Context, _ string, args []string) ([]byte, error) {
		if slicesContains(args, "-nostdin") {
			t.Error("ffprobe does not support -nostdin")
		}
		if mediaConversionArgValue(args, "-protocol_whitelist") != "file,pipe" || mediaConversionArgValue(args, "-format_whitelist") != "wav" {
			t.Errorf("missing local-input restrictions: %v", args)
		}
		return []byte(`{"format":{"format_name":"wav"}}`), nil
	}
	if _, err := probeMediaMetadata(context.Background(), nil, "test.wav"); err != nil {
		t.Fatal(err)
	}
}

func TestProbeMediaMetadataRealFFprobe(t *testing.T) {
	binary, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is not installed on PATH")
	}
	wav, err := hex.DecodeString("524946462600000057415645666d74201000000001000100401f0000803e00000200100064617461020000000000")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sample.wav")
	if err := os.WriteFile(path, wav, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := probeMediaMetadata(context.Background(), &config.MediaConversionConfig{FFmpegPath: filepath.Join(filepath.Dir(binary), "ffmpeg"+filepath.Ext(binary))}, path)
	if err != nil || result.InputFormat != "wav" || result.AudioCodec != "pcm_s16le" || result.SampleRate != 8000 || result.Channels != 1 {
		t.Fatalf("real ffprobe failed: %+v: %v", result, err)
	}
}
