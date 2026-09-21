package server

import (
	"aurago/internal/config"
	"aurago/internal/rtlsdr"
	"aurago/internal/speechlab"
	"aurago/internal/tools"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// This test is deliberately opt-in. It operates only an isolated data directory,
// uses the configured ASR route, and does not save or restart the running server.
func TestRTLSDRHardwareAcceptance(t *testing.T) {
	directory := os.Getenv("AURAGO_RTLSDR_TEST_DIR")
	if runtime.GOOS != "linux" || directory == "" {
		t.Skip("requires opt-in Linux receiver test directory")
	}
	if !filepath.IsAbs(directory) {
		t.Fatal("absolute isolated test directory required")
	}
	data, err := os.ReadFile(os.Getenv("AURAGO_RTLSDR_TEST_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	var selected struct {
		SpeechLab config.SpeechLabConfig `yaml:"speech_lab"`
	}
	if err = yaml.Unmarshal(data, &selected); err != nil {
		t.Fatal("invalid speech configuration")
	}
	config.NormalizeSpeechLabConfig(&selected.SpeechLab, data)
	cfg := &config.Config{SpeechLab: selected.SpeechLab}
	cfg.VirtualDesktop.Enabled = true
	cfg.Docker.Enabled = true
	cfg.RTLSDR = config.RTLSDRConfig{Enabled: true, AllowAgent: true}
	cfg.Directories.DataDir = directory
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s.initRTLSDR()
	if s.RTLSDR == nil {
		t.Fatal("initialization failed")
	}
	defer func() { _ = s.RTLSDR.Close(); s.RTLSDRRuntime.Close(); tools.SetRTLSDRService(nil) }()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	tuning := rtlsdr.DefaultTuning()
	if frequency, err := strconv.ParseInt(os.Getenv("AURAGO_RTLSDR_FREQUENCY_HZ"), 10, 64); err == nil {
		tuning.Frequency = frequency
	}
	if err = s.RTLSDR.Tune(ctx, "hardware-test", tuning); err != nil {
		t.Fatal(err)
	}
	receiver, err := s.RTLSDR.Receiver(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("physical receiver: %s / %s, %d..%d Hz", receiver.Device, receiver.Tuner, receiver.Minimum, receiver.Maximum)
	retuneStarted := time.Now()
	if err := s.RTLSDR.Tune(ctx, "hardware-test", tuning); err != nil {
		t.Fatal(err)
	}
	if time.Since(retuneStarted) > 2*time.Second {
		t.Fatal("analog retune restarted the receive chain")
	}
	recording, err := s.RTLSDR.Record(rtlsdr.Recording{Name: "Hardware acceptance", Tuning: tuning, Duration: 10})
	if err != nil {
		t.Fatal(err)
	}
	// Closing the last browser does not cancel a server-side recording.
	if err = s.RTLSDR.Stop(ctx, "hardware-test"); err != nil {
		t.Fatal(err)
	}
	for {
		row, _ := s.RTLSDR.Recording(recording.ID)
		if row.Status == "complete" || row.Status == "partial" {
			recording = row
			break
		}
		if row.Status == "failed" {
			t.Fatalf("capture: %s", row.Error)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	if recording.Bytes < 8000 || recording.Seconds < 8 {
		t.Fatalf("capture too short: bytes=%d seconds=%f", recording.Bytes, recording.Seconds)
	}
	wav, err := s.RTLSDRRuntime.WAV(ctx, recording.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := speechlab.AnalyzePCM16WAV(wav)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("FLAC capture while no browser is present: %.2fs / %d bytes, PCM RMS %.6f", recording.Seconds, recording.Bytes, metrics.RMSLevel)
	// Use local TTS solely as a deterministic ASR input, distinct from RF reception.
	lab, err := speechlab.NewClient(effectiveSpeechLabConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	ready, err := lab.Require(ctx, true, true)
	if err != nil {
		t.Fatal(err)
	}
	speech, _, _, err := lab.Synthesize(ctx, "Dies ist die Radioaufnahme von AuraGo. Die Nachrichten beginnen um zwölf Uhr.", "de", ready.Voice, ready.TTSID, ready.Voice)
	if err != nil {
		t.Fatal(err)
	}
	// Decode the test speech through the same bounded worker used for radio files.
	// This original fixture is not represented as an over-the-air news recording.
	audioDirectory := filepath.Join(directory, "rtl-sdr")
	source := filepath.Join(audioDirectory, "asr-source.wav")
	if err = os.WriteFile(source, speech, 0600); err != nil {
		t.Fatal(err)
	}
	fixtureID := "00000000000000000000000000000001"
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "--network", "none", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--entrypoint", "ffmpeg", "-v", audioDirectory+":/data:rw", rtlsdr.RuntimeImage, "-nostdin", "-loglevel", "error", "-y", "-i", "/data/asr-source.wav", "-ar", "48000", "-ac", "2", "-c:a", "flac", "/data/"+fixtureID+".flac")
	if err = command.Run(); err != nil {
		t.Fatalf("test speech conversion: %v", err)
	}
	defer os.Remove(filepath.Join(audioDirectory, fixtureID+".flac"))
	speech, err = s.RTLSDRRuntime.WAV(ctx, fixtureID, 0, 60)
	if err != nil {
		t.Fatal(err)
	}
	recognizer, err := s.rtlSDRTranscriber(ctx)
	if err != nil {
		t.Fatal(err)
	}
	text, err := recognizer.Transcribe(ctx, speech)
	if err != nil {
		t.Fatal(err)
	}
	if len(text) < 12 {
		t.Fatal("ASR did not return a transcript")
	}
	t.Logf("existing frozen ASR route %s returned %d characters for original test speech", ready.ASRID, len(text))
}
