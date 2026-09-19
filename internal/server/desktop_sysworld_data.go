package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"maps"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
	"aurago/internal/systemworld"
	"aurago/internal/tools"
)

type systemWorldRuntime struct {
	mu          sync.Mutex
	metrics     map[string]float64
	entities    map[string]systemworld.Entity
	pending     []systemworld.Entity
	overflow    bool
	at          int64
	previousNet [2]float64
	netAt       int64
	storeMu     sync.Mutex
	storeDB     *sql.DB
	store       *systemworld.Store
	actionMu    sync.Mutex
	requests    map[string]worldActionResult
	toolStates  map[string]string
	lastFacts   int64
}

func (s *Server) worldRuntime() *systemWorldRuntime {
	s.systemWorldOnce.Do(func() {
		s.systemWorld = &systemWorldRuntime{metrics: map[string]float64{}, entities: map[string]systemworld.Entity{}, requests: map[string]worldActionResult{}, toolStates: map[string]string{}}
		for _, id := range []string{"agent", "infra", "integrations", "missions", "memory", "graph", "operations"} {
			s.systemWorld.entities[id] = systemworld.Entity{ID: id, Kind: "district", District: id, State: "unknown"}
		}
	})
	return s.systemWorld
}
func worldText(v string) string {
	v = security.Scrub(v)
	r := []rune(v)
	if len(r) > 96 {
		r = r[:96]
	}
	return strings.Map(func(c rune) rune {
		if c < 32 {
			return -1
		}
		return c
	}, string(r))
}

var worldID = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,160}$`)

func (w *systemWorldRuntime) set(e systemworld.Entity) {
	if !worldID.MatchString(e.ID) {
		return
	}
	if len(w.entities) >= 1000 && w.entities[e.ID].ID == "" {
		return
	}
	e.Label = worldText(e.Label)
	if e.Source == "" {
		e.Source = "system-world/" + e.Kind
	}
	if e.Kind != "tool" && e.At > 0 {
		// One source heartbeat covers an unchanged inventory without writing a
		// separate event for every entity on every poll.
		key := "observed:" + e.Source
		w.metrics[key] = math.Max(w.metrics[key], float64(e.At))
	}
	if old, ok := w.entities[e.ID]; !ok || old.State != e.State || old.Label != e.Label || old.Source != e.Source || old.Model != e.Model || old.Provider != e.Provider || !maps.Equal(old.Values, e.Values) {
		if len(w.pending) < 1000 {
			w.pending = append(w.pending, e)
		} else {
			w.overflow = true
		}
	}
	if len(w.entities) < 1000 || w.entities[e.ID].ID != "" {
		w.entities[e.ID] = e
	}
}
func (w *systemWorldRuntime) observe(kind SSEEventType, payload any) {
	// Explicit allowlist: chat, thinking, logs and tool arguments never enter the model.
	switch kind {
	case EventSystemMetrics, EventContainerUpdate, EventAgentStatus, EventMissionUpdate, EventAgentAction:
	default:
		return
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) > 2<<20 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now().UnixMilli()
	if kind == EventSystemMetrics {
		var p struct {
			CPU struct {
				Usage *float64 `json:"usage_percent"`
				Cores *float64 `json:"cores"`
			} `json:"cpu"`
			Memory struct {
				Used  *float64 `json:"used_percent"`
				Total *float64 `json:"total"`
				Bytes *float64 `json:"used"`
			} `json:"memory"`
			Disk struct {
				Used *float64 `json:"used_percent"`
				Free *float64 `json:"free"`
			} `json:"disk"`
			Network struct {
				Sent *float64 `json:"bytes_sent"`
				Recv *float64 `json:"bytes_recv"`
			} `json:"network"`
			Uptime    *float64        `json:"uptime_seconds"`
			Available map[string]bool `json:"available"`
		}
		if json.Unmarshal(data, &p) != nil {
			return
		}
		w.at = now
		for k, v := range map[string]*float64{"cpu": p.CPU.Usage, "cores": p.CPU.Cores, "ram": p.Memory.Used, "memory_total": p.Memory.Total, "memory_used": p.Memory.Bytes, "disk": p.Disk.Used, "disk_free": p.Disk.Free, "uptime": p.Uptime} {
			source := map[string]string{"ram": "memory", "memory_total": "memory", "memory_used": "memory", "disk_free": "disk"}[k]
			if source == "" {
				source = k
			}
			available, known := p.Available[source]
			if v != nil && (!known || available) && !math.IsNaN(*v) && !math.IsInf(*v, 0) && *v >= 0 {
				w.metrics[k] = *v
			} else {
				delete(w.metrics, k)
			}
		}
		delete(w.metrics, "network_sent")
		delete(w.metrics, "network_received")
		available, known := p.Available["network"]
		if p.Network.Sent != nil && p.Network.Recv != nil && (!known || available) {
			if w.netAt > 0 && now > w.netAt && now-w.netAt < 90000 {
				for i, v := range []float64{*p.Network.Sent, *p.Network.Recv} {
					key := []string{"network_sent", "network_received"}[i]
					if v >= w.previousNet[i] {
						w.metrics[key] = (v - w.previousNet[i]) * 1000 / float64(now-w.netAt)
					} else {
						delete(w.metrics, key)
					}
				}
			}
			w.previousNet = [2]float64{*p.Network.Sent, *p.Network.Recv}
			w.netAt = now
		} else {
			w.netAt = 0
		}
		state := "unknown"
		if _, ok := w.metrics["cpu"]; ok {
			state = "idle"
		}
		w.set(systemworld.Entity{ID: "infra", Kind: "district", District: "infra", State: state, At: now, Source: "system_metrics"})
	} else if kind == EventAgentAction {
		var p struct {
			Tool  string `json:"tool_name"`
			State string `json:"state"`
		}
		if json.Unmarshal(data, &p) != nil || !worldID.MatchString(p.Tool) {
			return
		}
		switch p.State {
		case "started", "succeeded", "failed", "blocked", "cancelled":
		default:
			return
		}
		if len(w.toolStates) < 128 || w.toolStates[p.Tool] != "" {
			w.toolStates[p.Tool] = p.State
		}
		w.set(systemworld.Entity{ID: "tool:" + p.Tool, Kind: "tool", District: "agent", Label: p.Tool, State: p.State, At: now, Source: "agent_action"})
	} else if kind == EventAgentStatus {
		var p struct {
			Busy   *bool `json:"busy"`
			IsBusy *bool `json:"is_busy"`
		}
		if json.Unmarshal(data, &p) != nil {
			return
		}
		busy := p.Busy
		if busy == nil {
			busy = p.IsBusy
		}
		if busy == nil {
			return
		}
		state := "idle"
		if *busy {
			state = "running"
		}
		e := w.entities["agent"]
		e.State = state
		e.At = now
		w.set(e)
	} else {
		var rows []struct {
			ID     string   `json:"id"`
			Name   string   `json:"name"`
			Names  []string `json:"names"`
			State  string   `json:"state"`
			Status string   `json:"status"`
		}
		if kind == EventMissionUpdate {
			var p struct {
				Missions json.RawMessage `json:"missions"`
			}
			if json.Unmarshal(data, &p) != nil {
				return
			}
			data = p.Missions
		}
		if json.Unmarshal(data, &rows) != nil {
			return
		}
		prefix, district := "container", "infra"
		if kind == EventMissionUpdate {
			prefix, district = "mission", "missions"
		}
		seen := map[string]bool{}
		w.metrics[prefix+"_count"] = float64(len(rows))
		sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
		for _, v := range rows[:min(len(rows), 800)] {
			id := prefix + ":" + v.ID
			seen[id] = true
			label := v.Name
			if label == "" && len(v.Names) > 0 {
				label = strings.TrimPrefix(v.Names[0], "/")
			}
			state := v.State
			if state == "" {
				state = v.Status
			}
			if !worldID.MatchString(state) {
				state = "unknown"
			}
			w.set(systemworld.Entity{ID: id, Kind: prefix, District: district, Label: label, State: state, At: now})
		}
		for id, e := range w.entities {
			if e.Kind == prefix && !seen[id] {
				e.State = "removed"
				e.At = now
				w.set(e)
				delete(w.entities, id)
			}
		}
	}
}
func (w *systemWorldRuntime) snapshot() systemworld.Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.snapshotLocked()
}
func (w *systemWorldRuntime) snapshotLocked() systemworld.Snapshot {
	out := systemworld.Snapshot{At: w.at, Metrics: map[string]float64{}, Entities: []systemworld.Entity{}}
	for k, v := range w.metrics {
		out.Metrics[k] = v
	}
	for _, e := range w.entities {
		out.Entities = append(out.Entities, e)
	}
	sort.Slice(out.Entities, func(i, j int) bool { return out.Entities[i].ID < out.Entities[j].ID })
	return out
}
func (s *Server) worldStore(ctx context.Context) (*systemworld.Store, error) {
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		return nil, err
	}
	w := s.worldRuntime()
	w.storeMu.Lock()
	defer w.storeMu.Unlock()
	db := svc.DB()
	if w.store == nil || w.storeDB != db {
		w.store, err = systemworld.NewStore(ctx, db)
		if err != nil {
			return nil, err
		}
		w.storeDB = db
	}
	return w.store, nil
}
func (s *Server) flushSystemWorld(ctx context.Context) {
	if !s.ConfigSnapshot().VirtualDesktop.Enabled {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	w := s.worldRuntime()
	now := time.Now().UnixMilli()
	w.observe(EventAgentStatus, map[string]any{"busy": tools.IsBusy()})
	if s.MissionManagerV2 != nil {
		w.observe(EventMissionUpdate, map[string]any{"missions": s.MissionManagerV2.List()})
	}
	if s.DaemonSupervisor != nil {
		w.mu.Lock()
		seen := map[string]bool{}
		for _, d := range s.DaemonSupervisor.ListDaemons() {
			id := "daemon:" + d.SkillID
			seen[id] = true
			w.set(systemworld.Entity{ID: id, Kind: "daemon", District: "infra", Label: d.SkillName, State: string(d.Status), At: now})
		}
		for id, e := range w.entities {
			if e.Kind == "daemon" && !seen[id] {
				e.State = "removed"
				e.At = now
				w.set(e)
				delete(w.entities, id)
			}
		}
		w.mu.Unlock()
	}
	s.collectWorldFacts(ctx, now)
	store, err := s.worldStore(ctx)
	if err != nil {
		return
	}
	w.mu.Lock()
	snap := w.snapshotLocked()
	snap.IncompleteBefore = w.overflow
	pending := append([]systemworld.Entity(nil), w.pending...)
	w.pending = nil
	w.overflow = false
	w.mu.Unlock()
	if err = store.Save(ctx, snap, pending); err != nil {
		w.mu.Lock()
		w.pending = append(pending, w.pending...)
		if len(w.pending) > 1000 {
			w.pending = w.pending[:1000]
			w.overflow = true
		}
		w.overflow = w.overflow || snap.IncompleteBefore
		w.mu.Unlock()
		s.Logger.Warn("System World history sample unavailable", "error", err)
	}
}
func (s *Server) worldCapabilities(e systemworld.Entity) []string {
	cfg := s.ConfigSnapshot()
	if cfg.VirtualDesktop.ReadOnly {
		return nil
	}
	if e.At == 0 || time.Now().UnixMilli()-e.At > 45000 {
		return nil
	}
	switch e.Kind {
	case "container":
		if cfg.Docker.Enabled && !cfg.Docker.ReadOnly {
			if e.State == "running" {
				return []string{"stop", "restart"}
			}
			if e.State == "exited" || e.State == "created" {
				return []string{"start"}
			}
		}
	case "daemon":
		if s.DaemonSupervisor != nil {
			if e.State == "running" {
				return []string{"stop"}
			}
			if e.State == "stopped" {
				return []string{"start"}
			}
		}
	case "mission":
		if s.MissionManagerV2 != nil && cfg.Tools.Missions.Enabled && !cfg.Tools.Missions.ReadOnly {
			if e.State == "running" {
				return []string{"cancel"}
			}
			if e.State != "queued" {
				return []string{"start"}
			}
		}
	}
	return nil
}
func handleSystemWorldRead(s *Server, kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if !s.ConfigSnapshot().VirtualDesktop.Enabled {
			w.WriteHeader(503)
			return
		}
		store, err := s.worldStore(r.Context())
		if err != nil {
			jsonError(w, "System World history unavailable", 503)
			return
		}
		now := time.Now().UnixMilli()
		since := now - int64(24*time.Hour/time.Millisecond)
		var result any
		switch kind {
		case "snapshot", "entity":
			snap := s.worldRuntime().snapshot()
			at, _ := strconv.ParseInt(r.URL.Query().Get("at"), 10, 64)
			if at > 0 {
				if at < since || at > now {
					w.WriteHeader(400)
					return
				}
				snap, err = store.At(r.Context(), at)
			} else {
				for i := range snap.Entities {
					snap.Entities[i].Actions = s.worldCapabilities(snap.Entities[i])
				}
			}
			result = snap
			if kind == "entity" {
				result = nil
				for _, e := range snap.Entities {
					if e.ID == r.URL.Query().Get("id") {
						result = e
						break
					}
				}
				if result == nil {
					w.WriteHeader(404)
					return
				}
			}
		case "history":
			result, err = store.History(r.Context(), since)
		case "events":
			if requested, parseErr := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64); parseErr == nil {
				if requested > now {
					requested = now
				}
				if requested > since {
					since = requested
				}
			}
			after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
			if after < 0 {
				after = 0
			}
			result, err = store.Events(r.Context(), since, after, 200)
		}
		if err != nil {
			if err == sql.ErrNoRows {
				jsonError(w, "No recorded sample at this time", 404)
			} else {
				jsonError(w, "System World history unavailable", 503)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
}
