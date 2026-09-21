package server

import (
	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/planner"
	"aurago/internal/rtlsdr"
	"aurago/internal/speechlab"
	"aurago/internal/tools"
	"context"
	"errors"
	"html"
	"path/filepath"
	"strings"
	"time"
)

func (s *Server) initRTLSDR() {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return
	}
	directory := filepath.Join(cfg.Directories.DataDir, "rtl-sdr")
	manager, err := rtlsdr.NewManager(directory, func() rtlsdr.RuntimeConfig {
		c := s.ConfigSnapshot()
		return rtlsdr.RuntimeConfig{Enabled: c.RTLSDR.Enabled && c.VirtualDesktop.Enabled, ReadOnly: c.RTLSDR.ReadOnly || c.VirtualDesktop.ReadOnly, DockerEnabled: c.Docker.Enabled, DockerReadOnly: c.Docker.ReadOnly, DockerHost: c.Docker.Host, Device: c.RTLSDR.Device, InDocker: c.Runtime.IsDocker}
	})
	if err != nil {
		s.Logger.Error("RTL-SDR runtime initialization failed", "error", err)
		return
	}
	service, err := rtlsdr.New(rtlsdr.Options{Directory: directory, Backend: manager, Policy: func() rtlsdr.Policy {
		c := s.ConfigSnapshot()
		return rtlsdr.Policy{Enabled: c.RTLSDR.Enabled && c.VirtualDesktop.Enabled, ReadOnly: c.RTLSDR.ReadOnly || c.VirtualDesktop.ReadOnly || !c.Docker.Enabled || c.Docker.ReadOnly, AllowAgent: c.RTLSDR.AllowAgent, QuotaBytes: int64(c.RTLSDR.QuotaGB) << 30}
	}, NewTranscriber: s.rtlSDRTranscriber, Register: s.rtlSDRRegister, Unregister: s.rtlSDRUnregister, Issue: s.rtlSDRIssue, NotifyUpcoming: func() {
		broadcastDesktopEvent(s, s.DesktopHub, desktop.Event{Type: "rtl_sdr_recording_soon", CreatedAt: time.Now()})
	}})
	if err != nil {
		s.Logger.Error("RTL-SDR initialization failed", "error", err)
		return
	}
	s.RTLSDRRuntime = manager
	s.RTLSDR = service
	tools.SetRTLSDRService(service)
}

type rtlSDRASR struct {
	cfg   config.Config
	lab   *speechlab.Client
	asrID string
}

func (s *Server) rtlSDRTranscriber(ctx context.Context) (rtlsdr.Transcriber, error) {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return nil, rtlsdr.ErrUnavailable
	}
	tr := &rtlSDRASR{cfg: *cfg}
	if cfg.SpeechLab.Active() && cfg.SpeechLab.ChatInputEnabled {
		tr.cfg.SpeechLab = effectiveSpeechLabConfig(cfg)
		client, err := speechlab.NewClient(tr.cfg.SpeechLab)
		if err != nil {
			return nil, err
		}
		ready, err := client.Require(ctx, true, false)
		if err != nil {
			return nil, err
		}
		tr.lab = client
		tr.asrID = ready.ASRID
	}
	return tr, nil
}
func (t *rtlSDRASR) Transcribe(ctx context.Context, wav []byte) (string, error) {
	metrics, err := speechlab.AnalyzePCM16WAV(wav)
	if err != nil {
		return "", err
	}
	if metrics.RMSLevel < 0.0005 {
		return "", nil
	}
	if t.lab != nil {
		result, err := t.lab.Transcribe(ctx, wav, t.cfg.SpeechLab.Language, t.asrID)
		if errors.Is(err, speechlab.ErrNoSpeechDetected) {
			return "", nil
		}
		return result.Text, err
	}
	result, _, err := tools.TranscribeAudio(ctx, "radio.wav", wav, &t.cfg)
	// The shared tool API returns one owned isolation envelope; the UI stores
	// plain text. Agent results are isolated again at the tool boundary.
	result = strings.TrimSpace(result)
	if strings.HasPrefix(result, "<external_data>") && strings.HasSuffix(result, "</external_data>") {
		result = html.UnescapeString(strings.TrimSuffix(strings.TrimPrefix(result, "<external_data>"), "</external_data>"))
	}
	return result, err
}
func (s *Server) rtlSDRRegister(ctx context.Context, r rtlsdr.Recording, path string) error {
	if s.MediaRegistryDB == nil {
		return nil
	}
	hash, err := tools.ComputeMediaFileHash(path)
	if err != nil {
		return err
	}
	_, _, err = tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{MediaType: "audio", SourceTool: "rtl_sdr", Filename: r.ID + ".flac", FilePath: path, WebPath: "/api/desktop/rtl-sdr/recordings/" + r.ID + "/audio", FileSize: r.Bytes, Format: "flac", Description: r.Name, Hash: hash, Tags: []string{"radio", "rtl-sdr", "recording"}})
	return err
}
func (s *Server) rtlSDRIssue(operation string, success bool) {
	if s.PlannerDB == nil {
		return
	}
	fingerprint := "rtl_sdr:" + operation
	if success {
		_, _ = planner.ResolveOperationalIssue(s.PlannerDB, fingerprint, "Radio operation succeeded.", time.Now())
		return
	}
	_, _ = planner.RecordOperationalIssue(s.PlannerDB, planner.OperationalIssue{Source: "rtl_sdr", Context: operation, Title: "RTL-SDR needs attention", Detail: "Radio operation failed: " + operation, Severity: "warning", Kind: planner.OperationalIssueKindRuntimeFailure, Fingerprint: fingerprint, OccurredAt: time.Now()})
}

func (s *Server) rtlSDRUnregister(path string) error {
	if s.MediaRegistryDB == nil {
		return nil
	}
	item, err := tools.GetMediaByFilePath(s.MediaRegistryDB, path)
	if err != nil {
		if strings.HasSuffix(err.Error(), "not found") {
			return nil
		}
		return err
	}
	if item == nil {
		return nil
	}
	return tools.DeleteMedia(s.MediaRegistryDB, item.ID)
}
