package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
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
		body, err := json.Marshal(s.CydHub.Snapshot())
		if err != nil {
			jsonError(w, "snapshot encode failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
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

func firstLANIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if v4 := ipnet.IP.To4(); v4 != nil {
			return v4.String()
		}
	}
	return "127.0.0.1"
}

func cydDeviceURL(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	host := strings.TrimSpace(cfg.Server.Host)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "localhost" || host == "127.0.0.1" {
		host = firstLANIPv4()
	}
	if cfg.Server.HTTPS.Enabled {
		port := cfg.Server.HTTPS.HTTPSPort
		if port <= 0 {
			port = 443
		}
		return fmt.Sprintf("https://%s:%d", host, port)
	}
	port := cfg.Server.Port
	if port <= 0 {
		port = 8088
	}
	return fmt.Sprintf("http://%s:%d", host, port)
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
			"device_url":   cydDeviceURL(cfg),
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

func (s *Server) cydDataDir() string {
	if s == nil || s.Cfg == nil {
		return ""
	}
	return strings.TrimSpace(s.Cfg.Directories.DataDir)
}

func handleCYDFirmwareStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()
		variants := cyd.DiscoverFirmware(s.cydDataDir())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"variants":          variants,
			"provision_offset":  cyd.FactoryOffset,
			"provision_size":    cyd.FactorySize,
			"device_url":        cydDeviceURL(cfg),
			"erase_recommended": true,
		})
	}
}

func handleCYDFirmwareProvision(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Token) == "" {
			jsonError(w, "token is required", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.URL) == "" {
			s.CfgMu.RLock()
			body.URL = cydDeviceURL(s.Cfg)
			s.CfgMu.RUnlock()
		}
		blob, err := cyd.EncodeFactoryBlob(body.URL, body.Token)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(blob)))
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(blob)
	}
}

func handleCYDFirmwareFile(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/api/cyd/firmware/")
		rest = strings.Trim(rest, "/")
		parts := strings.Split(rest, "/")
		if len(parts) != 2 {
			jsonError(w, "not found", http.StatusNotFound)
			return
		}
		path := cyd.FirmwareFilePath(s.cydDataDir(), parts[0], parts[1])
		if path == "" {
			jsonError(w, "firmware image not found", http.StatusNotFound)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			jsonError(w, "firmware image not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			jsonError(w, "firmware image not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = io.Copy(w, f)
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
	mux.HandleFunc("/api/cyd/firmware/status", handleCYDFirmwareStatus(s))
	mux.HandleFunc("/api/cyd/firmware/provision", handleCYDFirmwareProvision(s))
	mux.HandleFunc("/api/cyd/firmware/", handleCYDFirmwareFile(s))
}
