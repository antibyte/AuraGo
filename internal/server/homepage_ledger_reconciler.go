package server

import (
	"time"

	"aurago/internal/tools"
)

const homepageLedgerReconcileInterval = 15 * time.Minute

func startHomepageLedgerReconciler(shutdownCh <-chan struct{}, s *Server) {
	if s == nil || s.HomepageRegistryDB == nil {
		return
	}
	if err := tools.RecoverHomepageOperations(s.HomepageRegistryDB); err != nil && s.Logger != nil {
		s.Logger.Warn("Homepage operation recovery failed", "error", err)
	}
	go func() {
		runHomepageLedgerReconcileOnce(s)
		ticker := time.NewTicker(homepageLedgerReconcileInterval)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownCh:
				return
			case <-ticker.C:
				runHomepageLedgerReconcileOnce(s)
			}
		}
	}()
}

func runHomepageLedgerReconcileOnce(s *Server) {
	s.CfgMu.RLock()
	enabled := s.Cfg != nil && s.Cfg.Homepage.Enabled
	s.CfgMu.RUnlock()
	if !enabled {
		return
	}

	sites, err := tools.ListHomepageManagedSites(s.HomepageRegistryDB)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn("Homepage ledger reconcile skipped", "error", err)
		}
		return
	}
	cfg := homepageConfigFromServer(s)
	for _, site := range sites {
		if site.Status == "archived" {
			continue
		}
		if _, err := tools.ReconcileHomepageProject(cfg, s.HomepageRegistryDB, site.ProjectDir, s.Logger); err != nil && s.Logger != nil {
			s.Logger.Warn("Homepage ledger reconcile failed", "project_dir", site.ProjectDir, "error", err)
		}
	}
}
