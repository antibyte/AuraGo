package server

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
)

func (s *Server) registerInfrastructureRoutes(mux *http.ServeMux, shutdownCh chan struct{}) {
	registerSpeechLabRoutes(mux, s)
	if s.Cfg.WebConfig.Enabled {
		handleInvasion := func(pattern string, handler http.HandlerFunc) {
			mux.HandleFunc(pattern, s.guardInvasion(handler))
		}
		handleInvasion("/api/invasion/nests", handleInvasionNests(s))
		handleInvasion("/api/invasion/eggs", handleInvasionEggs(s))
		handleInvasion("/api/invasion/eggs/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/api/invasion/eggs/")
			if strings.HasSuffix(path, "/toggle") {
				handleInvasionEggToggle(s)(w, r)
			} else {
				handleInvasionEgg(s)(w, r)
			}
		})
		handleInvasion("/api/invasion/nests/", func(w http.ResponseWriter, r *http.Request) {
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
			} else if strings.HasSuffix(path, "/safe-reconfigure") {
				handleInvasionNestSafeReconfigure(s)(w, r)
			} else if strings.HasSuffix(path, "/config-history") {
				handleInvasionNestConfigHistory(s)(w, r)
			} else if strings.HasSuffix(path, "/config-rollback") {
				handleInvasionNestConfigRollback(s)(w, r)
			} else {
				handleInvasionNest(s)(w, r)
			}
		})
		handleInvasion("/api/invasion/ws", handleInvasionWebSocket(s))
		handleInvasion("/api/invasion/tasks/", handleInvasionTask(s))
		handleInvasion("/api/invasion/artifacts/offer", handleInvasionArtifactOffer(s))
		handleInvasion("/api/invasion/artifacts/upload/", handleInvasionArtifactUpload(s))
		handleInvasion("/api/invasion/artifacts/", handleInvasionArtifactDownload(s))
		handleInvasion("/api/invasion/messages", handleInvasionEggMessage(s))
		s.EggHub.OnDisconnect = func(nestID, eggID string) {
			s.Logger.Info("Egg disconnected", "nest_id", nestID, "egg_id", eggID)
		}
		s.EggHub.OnResult = func(nestID string, result bridge.ResultPayload) {
			s.Logger.Info("Task result received", "nest_id", nestID, "task_id", result.TaskID, "status", result.Status)
			if err := s.recordInvasionTaskResult(nestID, result); err != nil {
				s.Logger.Warn("Rejected Egg task result", "nest_id", nestID, "task_id", result.TaskID, "error", err)
			}
		}
		s.EggHub.OnMissionResult = func(nestID string, result bridge.MissionResultPayload) {
			s.Logger.Info("Remote mission result received", "nest_id", nestID, "mission_id", result.MissionID, "result", result.Result, "artifact_count", len(result.ArtifactIDs))
			if s.MissionManagerV2 != nil && s.invasionAllowed(false) {
				nest, err := invasion.GetNest(s.InvasionDB, nestID)
				if err != nil || s.validateInvasionWork(nestID, nest.EggID, "", result.MissionID, true) != nil || s.validateInvasionArtifacts(nestID, nest.EggID, "", result.MissionID, result.ArtifactIDs) != nil {
					return
				}
				output := result.Output
				if output == "" {
					output = result.Error
				}
				if len(result.ArtifactIDs) > 0 {
					output = strings.TrimSpace(output)
					if output != "" {
						output += "\n\n"
					}
					output += "Artifacts: " + strings.Join(result.ArtifactIDs, ", ")
				}
				if err := s.MissionManagerV2.SetRemoteResultFromNest(nestID, result.MissionID, result.Result, output); err != nil {
					s.Logger.Warn("Rejected remote mission result", "nest_id", nestID, "mission_id", result.MissionID, "error", err)
				} else {
					broadcastMissionState(s)
				}
			}
		}
		s.EggHub.OnConnect = func(nestID, eggID string) {
			_ = invasion.UpdateNestHatchStatus(s.InvasionDB, nestID, "running", "")
			go func() {
				time.Sleep(2 * time.Second)
				if s.MissionManagerV2 != nil {
					synced, err := s.MissionManagerV2.SyncRemoteMissionsForNest(nestID)
					if err != nil {
						s.Logger.Warn("Failed to re-sync remote missions after egg reconnect", "nest_id", nestID, "synced", synced, "error", err)
					} else if synced > 0 {
						s.Logger.Info("Re-synced remote missions after egg reconnect", "nest_id", nestID, "count", synced)
					}
					if synced > 0 || err != nil {
						broadcastMissionState(s)
					}
				}
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
			s.Logger.Warn("Egg heartbeat stale", "nest_id", nestID, "egg_id", eggID)
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
					if result, err := invasion.CleanupExpiredArtifactUploads(s.InvasionDB, 24*time.Hour); err != nil {
						s.Logger.Warn("Artifact upload cleanup failed", "error", err)
					} else if result.ExpiredUploads > 0 || result.StalePendingArtifacts > 0 {
						s.Logger.Info("Cleaned up stale invasion artifacts", "expired_uploads", result.ExpiredUploads, "stale_pending_artifacts", result.StalePendingArtifacts)
					}
					storage := invasion.NewArtifactStorage(filepath.Join(s.Cfg.Directories.DataDir, "invasion_artifacts"))
					if n, err := storage.CleanupTempFiles(24 * time.Hour); err != nil {
						s.Logger.Warn("Artifact temp-file cleanup failed", "error", err)
					} else if n > 0 {
						s.Logger.Info("Cleaned up stale invasion artifact temp files", "count", n)
					}
				}
			}
		}()
		s.Logger.Info("Invasion Control API registered at /api/invasion/...")
	}

	mux.HandleFunc("/api/proxy/status", handleProxyStatus(s))
	mux.HandleFunc("/api/proxy/start", handleProxyStart(s))
	mux.HandleFunc("/api/proxy/stop", handleProxyStop(s))
	mux.Handle("/api/proxy/destroy", requireAdmin(s, handleProxyDestroy(s)))
	mux.HandleFunc("/api/proxy/reload", handleProxyReload(s))
	mux.HandleFunc("/api/proxy/logs", handleProxyLogs(s))
	s.Logger.Info("Security Proxy API registered at /api/proxy/...")

	mux.Handle("/api/cloudflare-tunnel/status", requireAdmin(s, handleCloudflareTunnelStatus(s)))
	mux.Handle("/api/cloudflare-tunnel/start", requireAdmin(s, handleCloudflareTunnelStart(s)))
	mux.Handle("/api/cloudflare-tunnel/stop", requireAdmin(s, handleCloudflareTunnelStop(s)))
	mux.Handle("/api/cloudflare-tunnel/restart", requireAdmin(s, handleCloudflareTunnelRestart(s)))
	s.Logger.Info("Cloudflare Tunnel API registered at /api/cloudflare-tunnel/...")

	mux.HandleFunc("/api/tsnet/status", handleTsNetStatus(s))
	mux.Handle("/api/tsnet/start", requireAdmin(s, handleTsNetStart(s)))
	mux.Handle("/api/tsnet/stop", requireAdmin(s, handleTsNetStop(s)))
	mux.Handle("/api/tsnet/credentials", requireAdmin(s, handleTsNetCredentials(s)))
	mux.Handle("/api/tsnet/reauth", requireAdmin(s, handleTsNetReauth(s)))
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

	registerObsidianHandlers(mux, s)
	s.Logger.Info("Obsidian API registered at /api/obsidian/...")

	registerPreferencesHandlers(mux, s)
	s.Logger.Info("Preferences API registered at /api/preferences")
}
