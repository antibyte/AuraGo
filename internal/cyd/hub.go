package cyd

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"aurago/internal/uid"
)

var (
	globalMu  sync.RWMutex
	globalHub *Hub
)

// SetGlobal publishes the process-wide hub used by send_notification.
func SetGlobal(h *Hub) {
	globalMu.Lock()
	globalHub = h
	globalMu.Unlock()
}

// Global returns the process-wide hub, or nil.
func Global() *Hub {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalHub
}

type wsSender interface {
	SendJSON(v any) error
}

// Hub holds the latest snapshot, overlay queue, and connected CYD devices.
type Hub struct {
	mu           sync.Mutex
	inputs       Inputs
	overlay      *Notify
	overlayUntil time.Time
	devices      map[string]*Device
	clients      map[wsSender]string
	pinnedTask   string
	page         string
	brightness   int
	led          string
}

func NewHub() *Hub {
	return &Hub{
		devices: make(map[string]*Device),
		clients: make(map[wsSender]string),
		page:    "status",
	}
}

func (h *Hub) SetInputs(in Inputs) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.inputs = in
	if h.pinnedTask != "" {
		h.inputs.Task = h.pinnedTask
	}
	if h.page != "" {
		h.inputs.Page = h.page
	}
	if h.brightness > 0 {
		h.inputs.Brightness = h.brightness
	}
	if h.led != "" {
		h.inputs.LED = h.led
	}
	h.dropExpiredLocked(time.Now())
}

func (h *Hub) Snapshot() Snapshot {
	if h == nil {
		return BuildSnapshot(Inputs{}, nil)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dropExpiredLocked(time.Now())
	return BuildSnapshot(h.inputs, h.overlay)
}

func (h *Hub) Notify(title, body, priority string, ttl int) Notify {
	n := Notify{
		ID:       "ntf_" + uid.New(),
		Title:    Truncate(title, TitleMax),
		Body:     Truncate(body, BodyMax),
		Priority: priority,
		TTLS:     NotifyTTL(priority, ttl),
	}
	if n.Title == "" {
		n.Title = "AuraGo"
	}
	if h == nil {
		return n
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.overlay != nil && NotifyRank(priority) < NotifyRank(h.overlay.Priority) {
		return *h.overlay
	}
	copyN := n
	h.overlay = &copyN
	h.overlayUntil = time.Now().Add(time.Duration(n.TTLS) * time.Second)
	h.broadcastLocked(map[string]any{
		"type":     "notify",
		"id":       n.ID,
		"title":    n.Title,
		"body":     n.Body,
		"priority": n.Priority,
		"ttl_s":    n.TTLS,
	})
	return n
}

func (h *Hub) Ack(id string) {
	if h == nil || id == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.overlay != nil && h.overlay.ID == id {
		h.overlay = nil
		h.overlayUntil = time.Time{}
		h.broadcastLocked(map[string]any{"type": "clear", "id": id})
	}
}

func (h *Hub) Clear() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	id := ""
	if h.overlay != nil {
		id = h.overlay.ID
	}
	h.overlay = nil
	h.overlayUntil = time.Time{}
	h.pinnedTask = ""
	h.inputs.Task = ""
	if id != "" {
		h.broadcastLocked(map[string]any{"type": "clear", "id": id})
	}
}

func (h *Hub) PinTask(task string, ttl int) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pinnedTask = Truncate(task, TaskMax)
	h.inputs.Task = h.pinnedTask
	if ttl > 0 {
		h.overlayUntil = time.Now().Add(time.Duration(NotifyTTL("normal", ttl)) * time.Second)
	}
}

func NormalizePage(page string) string {
	switch strings.ToLower(strings.TrimSpace(page)) {
	case "load", "work", "host":
		return strings.ToLower(strings.TrimSpace(page))
	case "home":
		return "status"
	default:
		return "status"
	}
}

func (h *Hub) SetPage(page string) {
	if h == nil {
		return
	}
	page = NormalizePage(page)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.page = page
	h.inputs.Page = page
	h.broadcastLocked(map[string]any{"type": "page", "page": page})
}

func (h *Hub) SetBrightness(v int) {
	if h == nil {
		return
	}
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.brightness = v
	h.inputs.Brightness = v
}

func (h *Hub) SetLED(color string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.led = color
	h.inputs.LED = color
	h.broadcastLocked(map[string]any{"type": "led", "color": color})
}

func (h *Hub) Heartbeat(tokenID, name string, hb Heartbeat) {
	if h == nil || tokenID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	dev := h.devices[tokenID]
	if dev == nil {
		dev = &Device{TokenID: tokenID, Name: name}
		h.devices[tokenID] = dev
	}
	if name != "" {
		dev.Name = name
	}
	dev.Firmware = hb.Firmware
	dev.Variant = hb.Variant
	dev.RSSI = hb.RSSI
	dev.Width = hb.Width
	dev.Height = hb.Height
	dev.LastSeen = time.Now()
}

func (h *Hub) AddClient(tokenID string, client wsSender) {
	if h == nil || client == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = tokenID
	if tokenID != "" {
		dev := h.devices[tokenID]
		if dev == nil {
			dev = &Device{TokenID: tokenID}
			h.devices[tokenID] = dev
		}
		dev.WS = true
		dev.LastSeen = time.Now()
	}
}

func (h *Hub) RemoveClient(client wsSender) {
	if h == nil || client == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	tokenID := h.clients[client]
	delete(h.clients, client)
	if tokenID == "" {
		return
	}
	still := false
	for _, id := range h.clients {
		if id == tokenID {
			still = true
			break
		}
	}
	if !still {
		if dev := h.devices[tokenID]; dev != nil {
			dev.WS = false
		}
	}
}

func (h *Hub) Devices() []Device {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Device, 0, len(h.devices))
	for _, d := range h.devices {
		copyD := *d
		out = append(out, copyD)
	}
	return out
}

func (h *Hub) HasRecentDevice(maxAge time.Duration) bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.clients) > 0 {
		return true
	}
	cutoff := time.Now().Add(-maxAge)
	for _, d := range h.devices {
		if d.LastSeen.After(cutoff) {
			return true
		}
	}
	return false
}

func (h *Hub) ClientCount() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *Hub) dropExpiredLocked(now time.Time) {
	if h.overlay != nil && !h.overlayUntil.IsZero() && now.After(h.overlayUntil) {
		h.overlay = nil
		h.overlayUntil = time.Time{}
	}
}

func (h *Hub) broadcastLocked(v any) {
	if len(h.clients) == 0 {
		return
	}
	if _, err := json.Marshal(v); err != nil {
		return
	}
	for client := range h.clients {
		_ = client.SendJSON(v)
	}
}
