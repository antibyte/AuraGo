package server

import (
	"aurago/internal/config"
	"aurago/internal/rtlsdr"
	"errors"
	"gopkg.in/yaml.v3"
	"net/http"
	"os"
	"strings"
	"time"
)

func rtlSDRError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	code := "sdr_request_failed"
	switch {
	case errors.Is(err, rtlsdr.ErrNotFound):
		status = 404
		code = rtlsdr.ErrNotFound.Error()
	case errors.Is(err, rtlsdr.ErrBusy):
		status = 409
		code = rtlsdr.ErrBusy.Error()
	case errors.Is(err, rtlsdr.ErrQuota):
		status = 507
		code = rtlsdr.ErrQuota.Error()
	case errors.Is(err, rtlsdr.ErrReadOnly):
		status = 403
		code = rtlsdr.ErrReadOnly.Error()
	case errors.Is(err, rtlsdr.ErrDisabled), errors.Is(err, rtlsdr.ErrUnavailable):
		status = 503
	}
	if strings.HasPrefix(err.Error(), "sdr_") && !strings.ContainsAny(err.Error(), " :/\\\r\n") {
		code = err.Error()
	}
	radioJSON(w, status, map[string]string{"error": code})
}
func (s *Server) handleRTLSDR(w http.ResponseWriter, r *http.Request) {
	scope := desktopScopeRead
	write := r.Method != http.MethodGet && r.Method != http.MethodHead
	if write {
		scope = desktopScopeWrite
	}
	if !requireDesktopPermission(s, w, r, scope) {
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		rtlSDRError(w, rtlsdr.ErrDisabled)
		return
	}
	if write && cfg.VirtualDesktop.ReadOnly {
		rtlSDRError(w, rtlsdr.ErrReadOnly)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/desktop/rtl-sdr/"), "/")
	parts := strings.Split(path, "/")
	svc := s.RTLSDR
	if svc == nil {
		rtlSDRError(w, rtlsdr.ErrUnavailable)
		return
	}
	if path == "config" && r.Method == http.MethodPut {
		requireAdmin(s, http.HandlerFunc(s.handleRTLSDRConfig)).ServeHTTP(w, r)
		return
	}
	if path == "setup" && r.Method == http.MethodPost {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		requireAdmin(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.RTLSDRRuntime == nil {
				rtlSDRError(w, rtlsdr.ErrUnavailable)
				return
			}
			if err := s.RTLSDRRuntime.Prepare(); err != nil {
				rtlSDRError(w, err)
				return
			}
			radioJSON(w, 202, map[string]string{"job_id": "setup", "status": "preparing"})
		})).ServeHTTP(w, r)
		return
	}
	if path == "state" && r.Method == http.MethodGet {
		var runtime any
		if s.RTLSDRRuntime != nil {
			runtime = s.RTLSDRRuntime.Status()
		}
		radioJSON(w, 200, map[string]any{"state": svc.Snapshot(), "config": cfg.RTLSDR, "runtime": runtime})
		return
	}
	if path == "devices" && r.Method == http.MethodGet {
		radioJSON(w, 200, rtlsdr.Devices())
		return
	}
	if path == "receiver" && r.Method == http.MethodGet {
		v, err := svc.Receiver(r.Context())
		if err != nil {
			rtlSDRError(w, err)
			return
		}
		radioJSON(w, 200, v)
		return
	}
	if path == "stream" && r.Method == http.MethodGet {
		stream, err := svc.Stream(r.Context(), r.URL.Query().Get("client"))
		if err != nil {
			rtlSDRError(w, err)
			return
		}
		defer stream.Close()
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Accel-Buffering", "no")
		controller := http.NewResponseController(w)
		buffer := make([]byte, 8192)
		for {
			n, err := stream.Read(buffer)
			if n > 0 {
				_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if _, e := w.Write(buffer[:n]); e != nil {
					return
				}
				_ = controller.Flush()
			}
			if err != nil {
				return
			}
		}
	}
	if (path == "tune" || path == "heartbeat" || path == "stop") && r.Method == http.MethodPost {
		var v struct {
			Client string        `json:"client"`
			Tuning rtlsdr.Tuning `json:"tuning"`
		}
		if err := radioDecode(w, r, &v); err != nil {
			rtlSDRError(w, rtlsdr.ErrInvalid)
			return
		}
		var err error
		switch path {
		case "tune":
			err = svc.Tune(r.Context(), v.Client, v.Tuning)
		case "heartbeat":
			err = svc.Heartbeat(v.Client)
		case "stop":
			err = svc.Stop(r.Context(), v.Client)
		}
		if err != nil {
			rtlSDRError(w, err)
			return
		}
		radioJSON(w, 200, map[string]string{"status": "ok"})
		return
	}
	if path == "scan" && (r.Method == http.MethodPost || r.Method == http.MethodDelete) {
		if r.Method == http.MethodDelete {
			svc.CancelScan()
		} else if err := svc.Scan(r.Context()); err != nil {
			rtlSDRError(w, err)
			return
		}
		radioJSON(w, 202, map[string]string{"job_id": "scan"})
		return
	}
	if path == "favorites" && r.Method == http.MethodPost {
		var v rtlsdr.Station
		if err := radioDecode(w, r, &v); err != nil {
			rtlSDRError(w, rtlsdr.ErrInvalid)
			return
		}
		v, err := svc.SaveFavorite(v)
		if err != nil {
			rtlSDRError(w, err)
			return
		}
		radioJSON(w, 201, v)
		return
	}
	if len(parts) == 2 && parts[0] == "favorites" && r.Method == http.MethodDelete {
		if err := svc.DeleteFavorite(parts[1]); err != nil {
			rtlSDRError(w, err)
			return
		}
		radioJSON(w, 200, map[string]string{"status": "ok"})
		return
	}
	if path == "recordings" && r.Method == http.MethodPost {
		var v rtlsdr.Recording
		if err := radioDecode(w, r, &v); err != nil {
			rtlSDRError(w, rtlsdr.ErrInvalid)
			return
		}
		v, err := svc.Record(v)
		if err != nil {
			rtlSDRError(w, err)
			return
		}
		radioJSON(w, 202, v)
		return
	}
	if path == "schedules" && r.Method == http.MethodPost {
		var v rtlsdr.Schedule
		if err := radioDecode(w, r, &v); err != nil {
			rtlSDRError(w, rtlsdr.ErrInvalid)
			return
		}
		v, err := svc.SaveSchedule(v)
		if err != nil {
			rtlSDRError(w, err)
			return
		}
		radioJSON(w, 201, v)
		return
	}
	if len(parts) == 2 && parts[0] == "schedules" && r.Method == http.MethodDelete {
		if err := svc.DeleteSchedule(parts[1]); err != nil {
			rtlSDRError(w, err)
			return
		}
		radioJSON(w, 200, map[string]string{"status": "ok"})
		return
	}
	if len(parts) >= 2 && parts[0] == "recordings" {
		id := parts[1]
		if len(parts) == 2 && r.Method == http.MethodGet {
			v, err := svc.Recording(id)
			if err != nil {
				rtlSDRError(w, err)
				return
			}
			radioJSON(w, 200, v)
			return
		}
		if len(parts) == 2 && r.Method == http.MethodDelete {
			if err := svc.DeleteRecording(id); err != nil {
				rtlSDRError(w, err)
				return
			}
			radioJSON(w, 200, map[string]string{"status": "ok"})
			return
		}
		if len(parts) == 3 && parts[2] == "audio" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			f, err := svc.Audio(id)
			if err != nil {
				rtlSDRError(w, err)
				return
			}
			defer f.Close()
			st, err := f.Stat()
			if err != nil {
				rtlSDRError(w, rtlsdr.ErrNotFound)
				return
			}
			w.Header().Set("Content-Type", "audio/flac")
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			if r.URL.Query().Get("download") == "1" {
				w.Header().Set("Content-Disposition", `attachment; filename="radio-`+id+`.flac"`)
			}
			http.ServeContent(w, r, "radio.flac", st.ModTime(), f)
			return
		}
		if len(parts) == 3 && r.Method == http.MethodPost {
			var err error
			switch parts[2] {
			case "stop":
				err = svc.StopRecording(id)
			case "transcribe":
				err = svc.RetryTranscription(r.Context(), id)
			default:
				http.NotFound(w, r)
				return
			}
			if err != nil {
				rtlSDRError(w, err)
				return
			}
			radioJSON(w, 202, map[string]string{"job_id": id})
			return
		}
	}
	http.NotFound(w, r)
}

// This narrow configuration transaction preserves unrelated YAML sections and
// publishes through the same immutable authorization snapshot mechanism.
func (s *Server) handleRTLSDRConfig(w http.ResponseWriter, r *http.Request) {
	if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
		return
	}
	var patch struct {
		Enabled    *bool   `json:"enabled"`
		ReadOnly   *bool   `json:"read_only"`
		AllowAgent *bool   `json:"allow_agent"`
		Device     *string `json:"device"`
		QuotaGB    *int    `json:"quota_gb"`
	}
	if err := radioDecode(w, r, &patch); err != nil {
		rtlSDRError(w, rtlsdr.ErrInvalid)
		return
	}
	err := func() error {
		s.CfgSaveMu.Lock()
		defer s.CfgSaveMu.Unlock()
		old := s.ConfigSnapshot()
		next := *old
		if patch.Enabled != nil {
			next.RTLSDR.Enabled = *patch.Enabled
		}
		if patch.ReadOnly != nil {
			next.RTLSDR.ReadOnly = *patch.ReadOnly
		}
		if patch.AllowAgent != nil {
			next.RTLSDR.AllowAgent = *patch.AllowAgent
		}
		if patch.Device != nil {
			found := *patch.Device == ""
			for _, d := range rtlsdr.Devices() {
				if d.ID == *patch.Device {
					found = true
				}
			}
			if !found {
				return rtlsdr.ErrInvalid
			}
			next.RTLSDR.Device = *patch.Device
		}
		if patch.QuotaGB != nil {
			if *patch.QuotaGB < 1 || *patch.QuotaGB > 1000 {
				return rtlsdr.ErrInvalid
			}
			next.RTLSDR.QuotaGB = *patch.QuotaGB
		}
		if next.RTLSDR.Enabled && (!next.Docker.Enabled || next.Docker.ReadOnly) {
			return errors.New("sdr_docker_disabled")
		}
		if next.RTLSDR.Device != old.RTLSDR.Device {
			st := s.RTLSDR.Snapshot()
			if st.Active != "" || st.Listening || st.Scanning {
				return rtlsdr.ErrBusy
			}
		}
		data, err := os.ReadFile(old.ConfigPath)
		if err != nil {
			return err
		}
		var raw map[string]any
		if err = yaml.Unmarshal(data, &raw); err != nil {
			return err
		}
		if raw == nil {
			return rtlsdr.ErrInvalid
		}
		b, err := yaml.Marshal(next.RTLSDR)
		if err != nil {
			return err
		}
		var section map[string]any
		if err = yaml.Unmarshal(b, &section); err != nil {
			return err
		}
		raw["rtl_sdr"] = section
		out, err := yaml.Marshal(raw)
		if err != nil {
			return err
		}
		if err = config.WriteFileAtomic(old.ConfigPath, out, 0600); err != nil {
			return err
		}
		s.CfgMu.Lock()
		s.replaceConfigSnapshot(&next)
		s.CfgMu.Unlock()
		return nil
	}()
	if err != nil {
		rtlSDRError(w, err)
		return
	}
	radioJSON(w, 200, s.ConfigSnapshot().RTLSDR)
}
