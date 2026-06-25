package server

import (
	"time"

	"aurago/internal/tools"
)

const homepageLedgerReconcileInterval = 15 * time.Minute

func startHomepageLedgerReconciler(shutdownCh <-chan struct{}, s *Server) {
	if s == nil || s.HomepageRegistryDB == nil || s.Cfg == nil || !s.Cfg.Homepage.Enabled {
		return
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
	sites, err := tools.ListHomepageManagedSites(s.HomepageRegistryDB)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn("Homepage ledger reconcile skipped", "error", err)
		}
		return
	}
	cfg := homepageConfigFromServer(s)
	for _, site := range sites {
		if _, err := tools.ReconcileHomepageProject(cfg, s.HomepageRegistryDB, site.ProjectDir, s.Logger); err != nil && s.Logger != nil {
			s.Logger.Warn("Homepage ledger reconcile failed", "project_dir", site.ProjectDir, "error", err)
		}
	}
}
