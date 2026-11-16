package server

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/tools"
)

func (s *Server) runStoreZiroExport(ctx context.Context, project, operation string) (any, error) {
	cfg := s.ConfigSnapshot()
	if !cfg.Docker.Enabled || cfg.Docker.ReadOnly || cfg.VirtualDesktop.ReadOnly || (operation != "probe" && !cfg.Agent.AllowFilesystemWrite) {
		return nil, errors.New("KiCad exports require Docker write and Desktop file publication permissions")
	}
	if p, ok := tools.EffectiveRuntimePermissions(ctx); ok && (!p.DockerEnabled || p.DockerReadOnly || (operation != "probe" && !p.AllowFilesystemWrite)) {
		return nil, errors.New("KiCad export permissions are disabled for this run")
	}
	if !printGuardEngineIsLocal(cfg.Docker.Host, cfg.Runtime.IsDocker) {
		return nil, errors.New("KiCad snapshots require a local Docker engine with access to AuraGo's data directory")
	}
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		return nil, err
	}
	desktopCfg := svc.Config()
	jobs := filepath.Join(desktopCfg.DataDir, "ziro-export", "jobs")
	if desktopCfg.DataDir == "" {
		jobs = filepath.Join(filepath.Dir(desktopCfg.DBPath), "ziro-export", "jobs")
	}
	jobs, err = filepath.Abs(jobs)
	if err != nil {
		return nil, errors.New("Invalid export storage")
	}
	adapter, err := s.newZiroDocker(ctx, desktopCfg, jobs)
	if err != nil {
		return nil, err
	}
	defer adapter.client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	data, extension, err := svc.RunZiroExport(ctx, adapter, jobs, project, operation, storeFileLimit(svc))
	if err != nil {
		return nil, err
	}
	if operation == "probe" {
		return map[string]any{"version": "10.0.8", "image": desktop.ZiroExportImage}, nil
	}
	return s.publishStoreFile(ctx, svc, "ziro", extension, data)
}
