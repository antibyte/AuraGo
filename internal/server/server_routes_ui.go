package server

import (
	"fmt"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/tools"
	"aurago/ui"
)

func (s *Server) registerUIRoutes(mux *http.ServeMux, shutdownCh chan struct{}) (*http.Server, error) {
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
	_ = mime.AddExtensionType(".js", "application/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".woff", "font/woff")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")

	// Serve the embedded Web UI at root via html/template for i18n injection
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to create UI filesystem: %w", err)
	}

	// Load i18n translations from embedded ui/lang/ directory
	loadI18N(uiFS, s.Logger)

	tmpl, err := template.ParseFS(uiFS, "index.html")
	if err != nil {
		s.Logger.Error("Failed to parse UI template", "error", err)
	}

	// Config page (separate template, guarded by WebConfig.Enabled)
	if s.Cfg.WebConfig.Enabled {
		cfgTmpl, cfgErr := template.ParseFS(uiFS, "config.html")
		if cfgErr != nil {
			s.Logger.Error("Failed to parse config UI template", "error", cfgErr)
		}
		mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
			if cfgTmpl == nil {
				http.Error(w, "Config template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang":     lang,
				"I18N":     getI18NJSON(lang),
				"I18NMeta": getI18NMetaJSON(),
			}
			if err := cfgTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute config template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		// Serve the help texts JSON
		mux.HandleFunc("/config_help.json", func(w http.ResponseWriter, r *http.Request) {
			helpData, err := fs.ReadFile(uiFS, "config_help.json")
			if err != nil {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(helpData)
		})
		s.Logger.Info("Config UI enabled at /config")

		// Dashboard page (separate template, guarded by WebConfig.Enabled)
		dashTmpl, dashErr := template.ParseFS(uiFS, "dashboard.html")
		if dashErr != nil {
			s.Logger.Error("Failed to parse dashboard UI template", "error", dashErr)
		}
		mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			if dashTmpl == nil {
				http.Error(w, "Dashboard template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := dashTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute dashboard template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Dashboard UI enabled at /dashboard")

		plansTmpl, plansErr := template.ParseFS(uiFS, "plans.html")
		if plansErr != nil {
			s.Logger.Error("Failed to parse plans UI template", "error", plansErr)
		}
		mux.HandleFunc("/plans", func(w http.ResponseWriter, r *http.Request) {
			if plansTmpl == nil {
				http.Error(w, "Plans template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := plansTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute plans template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Plans UI enabled at /plans")

		// Mission Control page (legacy v1)
		mux.HandleFunc("/missions", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/missions/v2", http.StatusMovedPermanently)
		})
		s.Logger.Info("Mission Control UI /missions redirects to /missions/v2")

		// Mission Control V2 page (enhanced with triggers)
		missionV2Tmpl, missionV2Err := template.ParseFS(uiFS, "missions_v2.html")
		if missionV2Err != nil {
			s.Logger.Error("Failed to parse mission V2 UI template", "error", missionV2Err)
		}
		mux.HandleFunc("/missions/v2", func(w http.ResponseWriter, r *http.Request) {
			if missionV2Tmpl == nil {
				http.Error(w, "Mission V2 template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := missionV2Tmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute mission V2 template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Mission Control V2 UI enabled at /missions/v2")

		// Cheat Sheet Editor page
		cheatsheetTmpl, cheatsheetErr := template.ParseFS(uiFS, "cheatsheets.html")
		if cheatsheetErr != nil {
			s.Logger.Error("Failed to parse cheatsheet UI template", "error", cheatsheetErr)
		}
		mux.HandleFunc("/cheatsheets", func(w http.ResponseWriter, r *http.Request) {
			if cheatsheetTmpl == nil {
				http.Error(w, "Cheatsheet template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := cheatsheetTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute cheatsheet template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Cheat Sheet Editor UI enabled at /cheatsheets")

		// ── Media View Page (replaces Gallery) ──
		mediaTmpl, mediaTmplErr := template.ParseFS(uiFS, "media.html")
		if mediaTmplErr != nil {
			s.Logger.Error("Failed to parse media UI template", "error", mediaTmplErr)
		}
		serveMediaPage := func(w http.ResponseWriter, r *http.Request) {
			if mediaTmpl == nil {
				http.Error(w, "Media template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := mediaTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute media template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		}
		mux.HandleFunc("/media", serveMediaPage)
		// Legacy /gallery redirect for backward compatibility
		mux.HandleFunc("/gallery", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/media", http.StatusMovedPermanently)
		})
		s.Logger.Info("Media View UI enabled at /media (/gallery redirects here)")

		// ── Knowledge Center Page ──
		knowledgeTmpl, knowledgeTmplErr := template.ParseFS(uiFS, "knowledge.html")
		if knowledgeTmplErr != nil {
			s.Logger.Error("Failed to parse knowledge UI template", "error", knowledgeTmplErr)
		}
		mux.HandleFunc("/knowledge", func(w http.ResponseWriter, r *http.Request) {
			if knowledgeTmpl == nil {
				http.Error(w, "Knowledge template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := knowledgeTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute knowledge template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Knowledge Center UI enabled at /knowledge")

		// ── Containers Page ──
		containersTmpl, containersTmplErr := template.ParseFS(uiFS, "containers.html")
		if containersTmplErr != nil {
			s.Logger.Error("Failed to parse containers UI template", "error", containersTmplErr)
		}
		mux.HandleFunc("/containers", func(w http.ResponseWriter, r *http.Request) {
			if containersTmpl == nil {
				http.Error(w, "Containers template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := containersTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute containers template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Containers UI enabled at /containers")

		// ── TrueNAS Storage Page ──
		truenasTmpl, truenasTmplErr := template.ParseFS(uiFS, "truenas.html")
		if truenasTmplErr != nil {
			s.Logger.Error("Failed to parse TrueNAS UI template", "error", truenasTmplErr)
		}
		mux.HandleFunc("/truenas", func(w http.ResponseWriter, r *http.Request) {
			if truenasTmpl == nil {
				http.Error(w, "TrueNAS template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := truenasTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute TrueNAS template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("TrueNAS Storage UI enabled at /truenas")

		// ── Skills Manager Page ──
		skillsTmpl, skillsTmplErr := template.ParseFS(uiFS, "skills.html")
		if skillsTmplErr != nil {
			s.Logger.Error("Failed to parse Skills UI template", "error", skillsTmplErr)
		}
		mux.HandleFunc("/skills", func(w http.ResponseWriter, r *http.Request) {
			if skillsTmpl == nil {
				http.Error(w, "Skills template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := skillsTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute Skills template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Skills Manager UI enabled at /skills")
	}

	// Invasion Control UI page (always registered — same pattern as /setup)
	invasionTmpl, invasionErr := template.ParseFS(uiFS, "invasion_control.html")
	if invasionErr != nil {
		s.Logger.Error("Failed to parse invasion control UI template", "error", invasionErr)
	}
	mux.HandleFunc("/invasion", func(w http.ResponseWriter, r *http.Request) {
		if invasionTmpl == nil {
			http.Error(w, "Invasion Control template error", http.StatusInternalServerError)
			return
		}
		lang := normalizeLang(s.Cfg.Server.UILanguage)
		data := map[string]interface{}{
			"Lang": lang,
			"I18N": getI18NJSON(lang),
		}
		if err := invasionTmpl.Execute(w, data); err != nil {
			s.Logger.Error("Failed to execute invasion control template", "error", err)
			http.Error(w, "Template render error", http.StatusInternalServerError)
		}
	})
	s.Logger.Info("Invasion Control UI registered at /invasion")

	// Quick Setup wizard page (always available — parsed outside WebConfig guard)
	setupTmpl, setupErr := template.ParseFS(uiFS, "setup.html")
	if setupErr != nil {
		s.Logger.Error("Failed to parse setup UI template", "error", setupErr)
	}
	mux.HandleFunc("/setup", func(w http.ResponseWriter, r *http.Request) {
		if setupTmpl == nil {
			http.Error(w, "Setup template error", http.StatusInternalServerError)
			return
		}
		lang := normalizeLang(s.Cfg.Server.UILanguage)
		data := map[string]interface{}{
			"Lang": lang,
			"I18N": getI18NJSON(lang),
		}
		if err := setupTmpl.Execute(w, data); err != nil {
			s.Logger.Error("Failed to execute setup template", "error", err)
			http.Error(w, "Template render error", http.StatusInternalServerError)
		}
	})

	// Auth login / logout pages (registered here so they can use uiFS)
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleAuthLoginPage(s, uiFS)(w, r)
		case http.MethodPost:
			handleAuthLogin(s)(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/auth/logout", handleAuthLogout(s))

	staticHandler := http.FileServer(http.FS(uiFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Redirect to setup wizard if LLM is not configured (first start)
			s.CfgMu.RLock()
			showSetup := needsSetup(s.Cfg)
			s.CfgMu.RUnlock()

			if showSetup && r.URL.Query().Get("skip_setup") != "1" {
				http.Redirect(w, r, "/setup", http.StatusTemporaryRedirect)
				return
			}

			if tmpl != nil {
				lang := normalizeLang(s.Cfg.Server.UILanguage)
				data := map[string]interface{}{
					"Lang":               lang,
					"I18N":               getI18NJSON(lang),
					"ShowToolResults":    s.Cfg.Agent.ShowToolResults,
					"DebugMode":          agent.GetDebugMode(),
					"PersonalityEnabled": s.Cfg.Personality.Engine,
				}
				if err := tmpl.Execute(w, data); err != nil {
					s.Logger.Error("Failed to execute UI template", "error", err)
					http.Error(w, "Template render error", http.StatusInternalServerError)
					return
				}
			} else {
				http.Error(w, "Template error", http.StatusInternalServerError)
			}
			return
		}
		// Serve static assets from embedded UI FS (logos, etc.)
		staticHandler.ServeHTTP(w, r)
	})

	// Serve generated documents from the document_creator output directory
	docDir := s.Cfg.Tools.DocumentCreator.OutputDir
	if docDir == "" {
		docDir = filepath.Join(s.Cfg.Directories.DataDir, "documents")
	}
	os.MkdirAll(docDir, 0755) // ensure directory exists
	docHandler := http.StripPrefix("/files/documents/", http.FileServer(neuteredFileSystem{http.Dir(docDir)}))
	mux.HandleFunc("/files/documents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		filename := filepath.Base(r.URL.Path)
		// Allow inline display when ?inline=1 is set (e.g. PDF preview)
		if r.URL.Query().Get("inline") == "1" {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
		} else {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		}
		docHandler.ServeHTTP(w, r)
	})

	// Serve agent audio files from data/audio directory
	audioDir := filepath.Join(s.Cfg.Directories.DataDir, "audio")
	os.MkdirAll(audioDir, 0755) // ensure directory exists
	audioHandler := http.StripPrefix("/files/audio/", http.FileServer(neuteredFileSystem{http.Dir(audioDir)}))
	mux.HandleFunc("/files/audio/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		audioHandler.ServeHTTP(w, r)
	})

	// Serve generated images from data directory
	genImgDir := filepath.Join(s.Cfg.Directories.DataDir, "generated_images")
	genImgHandler := http.StripPrefix("/files/generated_images/", http.FileServer(neuteredFileSystem{http.Dir(genImgDir)}))
	mux.HandleFunc("/files/generated_images/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		genImgHandler.ServeHTTP(w, r)
	})

	// Serve generated videos from data directory
	genVideoDir := filepath.Join(s.Cfg.Directories.DataDir, "generated_videos")
	os.MkdirAll(genVideoDir, 0755)
	genVideoHandler := http.StripPrefix("/files/generated_videos/", http.FileServer(neuteredFileSystem{http.Dir(genVideoDir)}))
	mux.HandleFunc("/files/generated_videos/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		genVideoHandler.ServeHTTP(w, r)
	})

	// Serve static files securely from the workspace directory
	fsHandler := http.StripPrefix("/files/", http.FileServer(neuteredFileSystem{http.Dir(s.Cfg.Directories.WorkspaceDir)}))
	mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fsHandler.ServeHTTP(w, r)
	})

	// Serve TTS audio files from data/tts/ on the main server
	ttsDir := tools.TTSAudioDir(s.Cfg.Directories.DataDir)
	os.MkdirAll(ttsDir, 0755)
	mainTTSHandler := http.StripPrefix("/tts/", http.FileServer(http.Dir(ttsDir)))
	mux.HandleFunc("/tts/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wav") {
			w.Header().Set("Content-Type", "audio/wav")
		} else {
			w.Header().Set("Content-Type", "audio/mpeg")
		}
		mainTTSHandler.ServeHTTP(w, r)
	})

	// Phase X: Dedicated TTS Server for Chromecast
	// Declared outside the if-block so the graceful shutdown goroutine can close it.
	var ttsServer *http.Server
	if s.Cfg.Chromecast.Enabled && s.Cfg.Chromecast.TTSPort > 0 {
		ccTTSDir := tools.TTSAudioDir(s.Cfg.Directories.DataDir)
		ttsMux := http.NewServeMux()
		ttsFsHandler := http.StripPrefix("/tts/", http.FileServer(http.Dir(ccTTSDir)))
		ttsMux.HandleFunc("/tts/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".wav") {
				w.Header().Set("Content-Type", "audio/wav")
			} else {
				w.Header().Set("Content-Type", "audio/mpeg")
			}
			ttsFsHandler.ServeHTTP(w, r)
		})

		// Bind TTS to the configured server host so it doesn't accidentally
		// listen on all interfaces when the server is internet-facing.
		// Chromecasts reach it on the LAN IP the operator put in server.host.
		ttsHost := s.Cfg.Server.Host
		if ttsHost == "" {
			ttsHost = "0.0.0.0"
		}
		ttsServer = &http.Server{
			Addr:    fmt.Sprintf("%s:%d", ttsHost, s.Cfg.Chromecast.TTSPort),
			Handler: ttsMux,
		}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.Logger.Error("[TTS Server] Goroutine panic recovered", "error", r)
				}
			}()
			s.Logger.Info("Starting Dedicated TTS Server", "host", ttsHost, "port", s.Cfg.Chromecast.TTSPort)
			if err := ttsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.Logger.Warn("Dedicated TTS Server failed (Chromecast audio will not be available)", "error", err)
			}
		}()
	}

	return ttsServer, nil
}
