package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/cyd"
	"aurago/internal/prompts"
	"aurago/internal/tools"
	"github.com/gorilla/websocket"
)

var cydWSUpgrader = websocket.Upgrader{
	CheckOrigin:     func(*http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 2048,
}

type cydWSConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *cydWSConn) SendJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteJSON(v)
}

type cydRateLimit struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func (r *cydRateLimit) allow(key string, minGap time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.last == nil {
		r.last = make(map[string]time.Time)
	}
	now := time.Now()
	if prev, ok := r.last[key]; ok && now.Sub(prev) < minGap {
		return false
	}
	r.last[key] = now
	return true
}

var cydSnapshotLimiter = &cydRateLimit{}

func (s *Server) ensureCydHub() *cyd.Hub {
	if s.CydHub == nil {
		s.CydHub = cyd.NewHub()
		cyd.SetGlobal(s.CydHub)
	}
	return s.CydHub
}

func cydBearerToken(r *http.Request, allowQuery bool) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(auth) >= 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if allowQuery {
		return strings.TrimSpace(r.URL.Query().Get("token"))
	}
	return ""
}

func (s *Server) authorizeCYD(w http.ResponseWriter, r *http.Request, allowQuery bool) (tokenID, tokenName string, ok bool) {
	if s.TokenManager == nil {
		jsonError(w, "cyd tokens are not available", http.StatusServiceUnavailable)
		return "", "", false
	}
	raw := cydBearerToken(r, allowQuery)
	if raw == "" {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return "", "", false
	}
	meta, valid := s.TokenManager.Validate(raw, "cyd")
	if !valid {
		jsonError(w, "forbidden", http.StatusForbidden)
		return "", "", false
	}
	s.TokenManager.TouchLastUsed(meta.ID)
	return meta.ID, meta.Name, true
}

func (s *Server) refreshCydSnapshot() {
	hub := s.ensureCydHub()
	s.CfgMu.RLock()
	cfg := s.Cfg
	s.CfgMu.RUnlock()
	if cfg == nil || !cfg.Cyd.Enabled {
		return
	}

	in := cyd.Inputs{
		Busy:    tools.IsBusy(),
		Model:   cfg.LLM.Model,
		UptimeS: uint64(time.Since(s.StartedAt).Seconds()),
	}
	if personality, _ := prompts.ResolvePersonalityID(cfg.Personality.CorePersonality); personality != "" {
		in.Personality = personality
	}
	raw := tools.GetSystemMetrics("")
	var metricsResult struct {
		Status string              `json:"status"`
		Data   tools.SystemMetrics `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &metricsResult); err == nil {
		in.CPUPct = metricsResult.Data.CPU.UsagePercent
		in.MemPct = metricsResult.Data.Memory.UsedPercent
		in.DiskPct = metricsResult.Data.Disk.UsedPercent
	}
	if host := tools.GetHostInfo(); host.Uptime > 0 {
		in.HostUptimeS = host.Uptime
	}
	if s.MissionManagerV2 != nil {
		running := 0
		for _, m := range s.MissionManagerV2.List() {
			if m.Status == "running" {
				running++
			}
		}
		queue, runningID := s.MissionManagerV2.GetQueue()
		if runningID != "" {
			running = 1
		}
		queued := 0
		if queue != nil {
			queued = len(queue.List())
		}
		in.MissionsRunning = running
		in.MissionsQueued = queued
	}
	if s.ShortTermMem != nil {
		notes, err := s.ShortTermMem.ListNotes("", -1)
		if err == nil {
			open := 0
			for _, n := range notes {
				if !n.Done {
					open++
				}
			}
			in.NotesOpen = open
		}
		if h, err := s.ShortTermMem.GetHoursSinceLastUserMessage(""); err == nil {
			in.LastUserH = h
		}
	}
	hub.SetInputs(in)
	if cfg.Cyd.MQTTMirror && cfg.MQTT.Enabled {
		body, err := json.Marshal(hub.Snapshot())
		if err == nil {
			_ = tools.MQTTPublish("aurago/cyd/snapshot", string(body), 0, true, s.Logger)
		}
	}
}

func handleCYDSnapshot(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		enabled := s.Cfg != nil && s.Cfg.Cyd.Enabled
		s.CfgMu.RUnlock()
		if !enabled {
			jsonError(w, "cyd is disabled", http.StatusNotFound)
			return
		}
		tokenID, name, ok := s.authorizeCYD(w, r, false)
		if !ok {
			return
		}
		if !cydSnapshotLimiter.allow(tokenID, 500*time.Millisecond) {
			jsonError(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		s.ensureCydHub().Heartbeat(tokenID, name, cyd.Heartbeat{})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.CydHub.Snapshot())
	}
}

func handleCYDHeartbeat(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tokenID, name, ok := s.authorizeCYD(w, r, false)
		if !ok {
			return
		}
		var hb cyd.Heartbeat
		_ = json.NewDecoder(r.Body).Decode(&hb)
		s.ensureCydHub().Heartbeat(tokenID, name, hb)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleCYDAck(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if _, _, ok := s.authorizeCYD(w, r, false); !ok {
			return
		}
		var body struct {
			ID     string `json:"id"`
			Action string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
			jsonError(w, "id is required", http.StatusBadRequest)
			return
		}
		s.ensureCydHub().Ack(body.ID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleCYDStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()
		enabled := false
		poll := 5
		if cfg != nil {
			enabled = cfg.Cyd.Enabled
			if cfg.Cyd.PollSeconds > 0 {
				poll = cfg.Cyd.PollSeconds
			}
		}
		hub := s.ensureCydHub()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"enabled":      enabled,
			"devices":      hub.Devices(),
			"ws_clients":   hub.ClientCount(),
			"poll_seconds": poll,
		})
	}
}

func handleCYDTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		enabled := s.Cfg != nil && s.Cfg.Cyd.Enabled
		s.CfgMu.RUnlock()
		if !enabled {
			jsonError(w, "cyd is disabled", http.StatusBadRequest)
			return
		}
		var body struct {
			Title    string `json:"title"`
			Message  string `json:"message"`
			Priority string `json:"priority"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.TrimSpace(body.Message) == "" {
			body.Message = "AuraGo CYD test notification"
		}
		if body.Title == "" {
			body.Title = "AuraGo"
		}
		if body.Priority == "" {
			body.Priority = "normal"
		}
		n := s.ensureCydHub().Notify(body.Title, body.Message, body.Priority, 0)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "notify": n})
	}
}

func handleCYDWebSocket(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		enabled := s.Cfg != nil && s.Cfg.Cyd.Enabled
		s.CfgMu.RUnlock()
		if !enabled {
			jsonError(w, "cyd is disabled", http.StatusNotFound)
			return
		}
		tokenID, name, ok := s.authorizeCYD(w, r, true)
		if !ok {
			return
		}
		conn, err := cydWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := &cydWSConn{conn: conn}
		hub := s.ensureCydHub()
		hub.AddClient(tokenID, client)
		hub.Heartbeat(tokenID, name, cyd.Heartbeat{})
		s.refreshCydSnapshot()
		_ = client.SendJSON(map[string]any{"type": "snapshot", "data": hub.Snapshot()})

		defer func() {
			hub.RemoveClient(client)
			_ = conn.Close()
		}()

		conn.SetReadLimit(2048)
		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg struct {
				Type   string `json:"type"`
				ID     string `json:"id"`
				Action string `json:"action"`
			}
			if json.Unmarshal(payload, &msg) != nil {
				continue
			}
			switch strings.ToLower(msg.Type) {
			case "pong":
				hub.Heartbeat(tokenID, name, cyd.Heartbeat{})
			case "ack":
				hub.Ack(msg.ID)
			case "heartbeat":
				hub.Heartbeat(tokenID, name, cyd.Heartbeat{})
			}
		}
	}
}

func registerCYDRoutes(mux *http.ServeMux, s *Server) {
	s.ensureCydHub()
	mux.HandleFunc("/api/cyd/snapshot", handleCYDSnapshot(s))
	mux.HandleFunc("/api/cyd/heartbeat", handleCYDHeartbeat(s))
	mux.HandleFunc("/api/cyd/ack", handleCYDAck(s))
	mux.HandleFunc("/api/cyd/ws", handleCYDWebSocket(s))
	mux.HandleFunc("/api/cyd/status", handleCYDStatus(s))
	mux.HandleFunc("/api/cyd/test", handleCYDTest(s))
}
