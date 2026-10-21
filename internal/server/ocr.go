package server

import (
	"aurago/internal/config"
	"aurago/internal/ocr"
	"aurago/internal/tools"
	"context"
	"path/filepath"
	"time"
)

func ocrPolicy(cfg *config.Config) ocr.Policy {
	if cfg == nil {
		return ocr.Policy{}
	}
	return ocr.Policy{Enabled: cfg.OCR.Enabled, DockerEnabled: cfg.Docker.Enabled, DockerReadOnly: cfg.Docker.ReadOnly,
		DockerHost: cfg.Docker.Host, InDocker: cfg.Runtime.IsDocker}
}
func (s *Server) initOCR() {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return
	}
	var manager *ocr.Manager
	runtime := ocr.NewDockerRuntime(func() ocr.Policy {
		if manager == nil {
			return ocrPolicy(cfg)
		}
		return manager.Policy()
	}, s.decisionsDockerSelfContainer, "")
	var err error
	manager, err = ocr.New(filepath.Join(cfg.Directories.DataDir, "ocr", "jobs"), ocrPolicy(cfg), runtime)
	if err != nil {
		s.Logger.Error("OCR initialization failed", "error", err)
		return
	}
	s.OCR = manager
	if cfg.OCR.Enabled {
		tools.PublishOCR(manager)
	}
}
func (s *Server) startOCR(ctx context.Context) {
	if s.OCR == nil {
		return
	}
	if !s.OCR.Policy().Enabled {
		go func() {
			cleanup, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			_ = s.OCR.CleanupDisabled(cleanup)
		}()
		return
	}
	go func() { _ = s.OCR.Start(ctx, false) }()
}
func (s *Server) shutdownOCR() {
	if s.OCR == nil {
		return
	}
	tools.WithdrawOCR(s.OCR)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = s.OCR.Close(ctx)
}
