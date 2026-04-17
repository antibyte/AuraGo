package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
)

func (s *Server) registerInfrastructureRoutes(mux *http.ServeMux, shutdownCh chan struct{}) {
	if s.Cfg.WebConfig.Enabled {
		mux.HandleFunc("/api/invasion/nests", handleInvasionNests(s))
		mux.HandleFunc("/api/invasion/eggs", handleInvasionEggs(s))
		mux.HandleFunc("/api/invasion/eggs/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/api/invasion/eggs/")
			if strings.HasSuffix(path, "/toggle") {
				handleInvasionEggToggle(s)(w, r)
			} else {
				handleInvasionEgg(s)(w, r)
			}
		})
		mux.HandleFunc("/api/invasion/nests/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/api/invasion/nests/")
			if strings.HasSuffix(path, "/toggle") {
				handleInvasionNestToggle(s)(w, r)
			} else if strings.HasSuffix(path, "/validate") {
				handleInvasionNestValidate(s)(w, r)
			} else if strings.HasSuffix(path, "/hatch") {
				handleInvasionNestHatch(s)(w, r)
			} else if strings.HasSuffix(path, "/stop") {
				handleInvasionNestStop(s)(w, r)
			} else if strings.HasSuffix(path, "/status") {
				handleInvasionNestHatchStatus(s)(w, r)
			} else if strings.HasSuffix(path, "/send-secret") {
				handleInvasionNestSendSecret(s)(w, r)
			} else if strings.HasSuffix(path, "/send-task") {
				handleInvasionNestSendTask(s)(w, r)
			} else if strings.HasSuffix(path, "/tasks") {
				handleInvasionNestTasks(s)(w, r)
			} else if strings.HasSuffix(path, "/rotate-key") {
				handleInvasionNestRotateKey(s)(w, r)
			} else if strings.HasSuffix(path, "/rollback") {
				handleInvasionNestRollback(s)(w, r)
			} else if strings.HasSuffix(path, "/deployments") {
				handleInvasionNestDeployments(s)(w, r)
			} else {
				handleInvasionNest(s)(w, r)
			}
		})
		mux.HandleFunc("/api/invasion/ws", handleInvasionWebSocket(s))
		mux.HandleFunc("/api/invasion/tasks/", handleInvasionTask(s))
		s.EggHub.OnDisconnect = func(nestID, eggID string) {
			s.Logger.Info("Egg disconnected", "nest_id", nestID, "egg_id", eggID)
			_ = invasion.UpdateNestHatchStatus(s.InvasionDB, nestID, "stopped", "connection lost")
		}
		s.EggHub.OnResult = func(nestID string, result bridge.ResultPayload) {
			s.Logger.Info("Task result received", "nest_id", nestID, "task_id", result.TaskID, "status", result.Status)
			status := "completed"
			if result.Status == "failure" {
				status = "failed"
			}
			_ = invasion.UpdateTaskStatus(s.InvasionDB, result.TaskID, status, result.Output, result.Error)
		}
		s.EggHub.OnConnect = func(nestID, eggID string) {
			_ = invasion.UpdateNestHatchStatus(s.InvasionDB, nestID, "running", "")
			go func() {
				time.Sleep(2 * time.Second)
				tasks, err := invasion.GetPendingTasks(s.InvasionDB, nestID)
				if err != nil || len(tasks) == 0 {
					return
				}
				s.Logger.Info("Re-sending pending tasks after reconnect", "nest_id", nestID, "count", len(tasks))
				for _, t := range tasks {
					taskPayload := bridge.TaskPayload{
						TaskID:      t.ID,
						Description: t.Description,
						Timeout:     t.Timeout,
					}
					if err := s.EggHub.SendTask(nestID, taskPayload); err != nil {
						s.Logger.Warn("Failed to re-send task", "task_id", t.ID, "error", err)
						continue
					}
					_ = invasion.UpdateTaskStatus(s.InvasionDB, t.ID, "sent", "", "")
				}
			}()
		}
		hbCtx, hbCancel := context.WithCancel(context.Background())
		go func() {
			<-shutdownCh
			hbCancel()
		}()
		s.EggHub.StartHeartbeatMonitor(hbCtx, 30*time.Second, 90*time.Second, func(nestID, eggID string) {
			s.Logger.Warn("Egg heartbeat stale, marking as failed", "nest_id", nestID, "egg_id", eggID)
			_ = invasion.UpdateNestHatchStatus(s.InvasionDB, nestID, "failed", "heartbeat timeout")
		})
		go func() {
			ticker := time.NewTicker(6 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-shutdownCh:
					return
				case <-ticker.C:
					if n, err := invasion.CleanupOldTasks(s.InvasionDB, 7*24*time.Hour); err != nil {
						s.Logger.Warn("Task cleanup failed", "error", err)
					} else if n > 0 {
						s.Logger.Info("Cleaned up old invasion tasks", "count", n)
					}
					if n, err := invasion.CleanupOldDeployments(s.InvasionDB, 30*24*time.Hour); err != nil {
						s.Logger.Warn("Deployment history cleanup failed", "error", err)
					} else if n > 0 {
						s.Logger.Info("Cleaned up old deployment history", "count", n)
					}
				}
			}
		}()
		s.Logger.Info("Invasion Control API registered at /api/invasion/...")
	}

	mux.HandleFunc("/api/proxy/status", handleProxyStatus(s))
	mux.HandleFunc("/api/proxy/start", handleProxyStart(s))
	mux.HandleFunc("/api/proxy/stop", handleProxyStop(s))
	mux.HandleFunc("/api/proxy/destroy", handleProxyDestroy(s))
	mux.HandleFunc("/api/proxy/reload", handleProxyReload(s))
	mux.HandleFunc("/api/proxy/logs", handleProxyLogs(s))
	s.Logger.Info("Security Proxy API registered at /api/proxy/...")

	mux.HandleFunc("/api/cloudflare-tunnel/status", handleCloudflareTunnelStatus(s))
	mux.HandleFunc("/api/cloudflare-tunnel/restart", handleCloudflareTunnelRestart(s))
	s.Logger.Info("Cloudflare Tunnel API registered at /api/cloudflare-tunnel/...")

	mux.HandleFunc("/api/tsnet/status", handleTsNetStatus(s))
	mux.HandleFunc("/api/tsnet/start", handleTsNetStart(s))
	mux.HandleFunc("/api/tsnet/stop", handleTsNetStop(s))
	s.Logger.Info("tsnet API registered at /api/tsnet/...")

	mux.HandleFunc("/api/cert/status", handleCertStatus(s))
	mux.HandleFunc("/api/cert/regenerate", handleCertRegenerate(s))
	mux.HandleFunc("/api/cert/upload", handleCertUpload(s))
	s.Logger.Info("Certificate API registered at /api/cert/...")

	if s.Cfg.TrueNAS.Enabled {
		registerTrueNASHandlers(mux, s)
		s.Logger.Info("TrueNAS API registered at /api/truenas/...")
	}

	registerJellyfinHandlers(mux, s)
	s.Logger.Info("Jellyfin API registered at /api/jellyfin/...")

	registerPreferencesHandlers(mux, s)
	s.Logger.Info("Preferences API registered at /api/preferences")
}
