package server

// desktop_fritzbox_widget.go – read-only overview endpoint for the Virtual
// Desktop Fritz!Box widget.
//
//	GET /api/desktop/fritzbox/overview?sections=connection,devices,telephony
//
// The handler reuses the TR-064 client of the Fritz!Box integration behind a
// per-section TTL cache with single-flight refreshes, so several browser tabs
// or reloads never turn into a request storm against the router. Sections are
// gated by the integration's feature groups; the payload never contains the
// password, MAC addresses, answering-machine paths or port-forwarding rules.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/fritzbox"
)

const (
	fritzWidgetTTLSystem     = 5 * time.Minute
	fritzWidgetTTLConnection = 4 * time.Second
	fritzWidgetTTLDevices    = 45 * time.Second
	fritzWidgetTTLTelephony  = 45 * time.Second
	fritzWidgetErrorRetry    = 10 * time.Second
	fritzWidgetBackendMaxAge = 10 * time.Minute
	fritzWidgetWaitTimeout   = 15 * time.Second
	fritzWidgetMaxHosts      = 40
	fritzWidgetMaxCalls      = 8
	fritzWidgetMaxNameRunes  = 48
	fritzWidgetCallDateForm  = "02.01.06 15:04"
)

var fritzWidgetSectionOrder = []string{"system", "connection", "devices", "telephony"}

// fritzWidgetBackend is the subset of *fritzbox.Client the widget reads from.
// Tests substitute a fake; production wraps the real client.
type fritzWidgetBackend interface {
	GetSystemInfo() (*fritzbox.SystemInfo, error)
	GetWANStatus() (*fritzbox.WANStatus, error)
	GetWANLinkInfo() (*fritzbox.WANLink, error)
	GetOnlineMonitor() (*fritzbox.OnlineMonitor, error)
	GetHostList() ([]fritzbox.HostEntry, error)
	GetWLANInfo(index int) (*fritzbox.WLANInfo, error)
	GetCallList() ([]fritzbox.CallEntry, error)
	GetTAMList(tamIndex int) ([]fritzbox.TAMEntry, error)
	Close()
}

type fritzWidgetSystem struct {
	Model         string `json:"model"`
	Firmware      string `json:"firmware"`
	UptimeSeconds int    `json:"uptime_seconds"`
}

type fritzWidgetMonitor struct {
	IntervalSeconds int     `json:"interval_seconds"`
	DownBps         []int64 `json:"down_bps"` // oldest first, bit/s
	UpBps           []int64 `json:"up_bps"`   // oldest first, bit/s
}

type fritzWidgetConnection struct {
	Online             bool                `json:"online"`
	Status             string              `json:"status"`
	UptimeSeconds      int                 `json:"uptime_seconds"`
	LastError          string              `json:"last_error,omitempty"`
	ExternalIPv4       string              `json:"external_ipv4"`
	ExternalIPv6       string              `json:"external_ipv6"`
	IPv6Prefix         string              `json:"ipv6_prefix"`
	AccessType         string              `json:"access_type"` // dsl, cable, fiber, ethernet, mobile, other
	LinkUp             bool                `json:"link_up"`
	MaxDownBps         int64               `json:"max_down_bps"`
	MaxUpBps           int64               `json:"max_up_bps"`
	DownBps            int64               `json:"down_bps"`
	UpBps              int64               `json:"up_bps"`
	TotalSentBytes     int64               `json:"total_sent_bytes"`
	TotalReceivedBytes int64               `json:"total_received_bytes"`
	Monitor            *fritzWidgetMonitor `json:"monitor,omitempty"`
}

type fritzWidgetHost struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	Active    bool   `json:"active"`
	Interface string `json:"interface"` // lan, wlan, other
}

type fritzWidgetWLAN struct {
	Index   int    `json:"index"`
	SSID    string `json:"ssid"`
	Enabled bool   `json:"enabled"`
	Channel int    `json:"channel"`
	Band    string `json:"band"` // "2.4", "5", "6" or ""
	Guest   bool   `json:"guest"`
}

type fritzWidgetDevices struct {
	Total          int               `json:"total"`
	Active         int               `json:"active"`
	LANActive      int               `json:"lan_active"`
	WLANActive     int               `json:"wlan_active"`
	HostsTruncated bool              `json:"hosts_truncated"`
	Hosts          []fritzWidgetHost `json:"hosts"`
	WLANs          []fritzWidgetWLAN `json:"wlans"`
}

type fritzWidgetCall struct {
	Type      string `json:"type"` // incoming, missed, outgoing, active, rejected, unknown
	Date      string `json:"date"`
	Timestamp string `json:"timestamp,omitempty"`
	Name      string `json:"name"`
	Number    string `json:"number"`
	Called    string `json:"called"`
	Duration  string `json:"duration"`
}

type fritzWidgetTelephony struct {
	MissedToday  int               `json:"missed_today"`
	TAMAvailable bool              `json:"tam_available"`
	TAMNew       int               `json:"tam_new"`
	Calls        []fritzWidgetCall `json:"calls"`
}

type fritzWidgetCapabilities struct {
	System     bool `json:"system"`
	Connection bool `json:"connection"`
	Devices    bool `json:"devices"`
	Telephony  bool `json:"telephony"`
}

func (c fritzWidgetCapabilities) has(section string) bool {
	switch section {
	case "system":
		return c.System
	case "connection":
		return c.Connection
	case "devices":
		return c.Devices
	case "telephony":
		return c.Telephony
	}
	return false
}

func fritzWidgetCapabilitiesFor(cfg *config.Config) fritzWidgetCapabilities {
	fb := cfg.FritzBox
	return fritzWidgetCapabilities{
		System:     fb.System.Enabled,
		Connection: fb.Network.Enabled,
		Devices:    fb.Network.Enabled && (fb.Network.SubFeatures.Hosts || fb.Network.SubFeatures.WLAN),
		Telephony:  fb.Telephony.Enabled && (fb.Telephony.SubFeatures.CallLists || fb.Telephony.SubFeatures.TAM),
	}
}

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

type fritzWidgetEntry struct {
	data        any
	err         error
	fetchedAt   time.Time
	attemptedAt time.Time
	inflight    chan struct{}
}

type fritzWidgetCache struct {
	mu         sync.Mutex
	now        func() time.Time
	newBackend func(cfg *config.Config) (fritzWidgetBackend, error)
	backend    fritzWidgetBackend
	backendKey string
	backendAt  time.Time
	entries    map[string]*fritzWidgetEntry
}

func newFritzWidgetCache(s *Server) *fritzWidgetCache {
	return &fritzWidgetCache{
		now:        time.Now,
		entries:    map[string]*fritzWidgetEntry{},
		newBackend: func(cfg *config.Config) (fritzWidgetBackend, error) { return newFritzWidgetClient(s, cfg) },
	}
}

// newFritzWidgetClient builds the production TR-064 client. The password is
// read from the Vault only here, never echoed anywhere.
func newFritzWidgetClient(s *Server, cfg *config.Config) (fritzWidgetBackend, error) {
	clientCfg := *cfg
	if clientCfg.FritzBox.Password == "" && s != nil && s.Vault != nil {
		if v, _ := s.Vault.ReadSecret("fritzbox_password"); v != "" {
			clientCfg.FritzBox.Password = v
		}
	}
	client, err := fritzbox.NewClient(clientCfg)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func fritzWidgetBackendKey(cfg *config.Config) string {
	fb := cfg.FritzBox
	return fmt.Sprintf("%s|%d|%t|%d|%t|%d|%s|%t|%t", fb.Host, fb.Port, fb.HTTPS, fb.WebPort, fb.InsecureSkipVerify, fb.Timeout, fb.Username, fb.Network.Enabled, fb.Telephony.Enabled)
}

func (c *fritzWidgetCache) backendFor(cfg *config.Config) (fritzWidgetBackend, error) {
	key := fritzWidgetBackendKey(cfg)
	c.mu.Lock()
	if c.backend != nil && c.backendKey == key && c.now().Sub(c.backendAt) < fritzWidgetBackendMaxAge {
		backend := c.backend
		c.mu.Unlock()
		return backend, nil
	}
	old := c.backend
	c.backend = nil
	c.mu.Unlock()
	if old != nil {
		old.Close()
	}
	backend, err := c.newBackend(cfg)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.backend != nil {
		// Another refresh rebuilt the client concurrently; keep that one.
		go backend.Close()
		return c.backend, nil
	}
	c.backend = backend
	c.backendKey = key
	c.backendAt = c.now()
	return backend, nil
}

func (c *fritzWidgetCache) dropBackend(backend fritzWidgetBackend) {
	c.mu.Lock()
	if c.backend != backend {
		c.mu.Unlock()
		return
	}
	c.backend = nil
	c.mu.Unlock()
	if backend != nil {
		backend.Close()
	}
}

// section returns cached data for one section. Fresh entries are served
// directly. Stale entries trigger one background refresh; sections marked
// waitWhenStale block on that refresh (fast connection data), the others
// return the stale value immediately (stale-while-revalidate).
func (c *fritzWidgetCache) section(ctx context.Context, cfg *config.Config, name string, ttl time.Duration, waitWhenStale bool,
	fetch func(fritzWidgetBackend) (any, error)) (any, time.Time, bool, error) {
	c.mu.Lock()
	entry := c.entries[name]
	if entry == nil {
		entry = &fritzWidgetEntry{}
		c.entries[name] = entry
	}
	now := c.now()
	window := ttl
	if entry.err != nil && window > fritzWidgetErrorRetry {
		window = fritzWidgetErrorRetry
	}
	if !entry.attemptedAt.IsZero() && now.Sub(entry.attemptedAt) < window {
		data, at, err := entry.data, entry.fetchedAt, entry.err
		c.mu.Unlock()
		return data, at, err != nil && data != nil, err
	}
	if entry.inflight == nil {
		done := make(chan struct{})
		entry.inflight = done
		go c.refresh(cfg, entry, fetch, done)
	}
	wait := entry.inflight
	staleData, staleAt, staleErr := entry.data, entry.fetchedAt, entry.err
	c.mu.Unlock()

	if staleData != nil && !waitWhenStale {
		return staleData, staleAt, true, staleErr
	}
	select {
	case <-wait:
	case <-ctx.Done():
		return staleData, staleAt, staleData != nil, ctx.Err()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return entry.data, entry.fetchedAt, entry.err != nil && entry.data != nil, entry.err
}

func (c *fritzWidgetCache) refresh(cfg *config.Config, entry *fritzWidgetEntry, fetch func(fritzWidgetBackend) (any, error), done chan struct{}) {
	defer close(done)
	backend, err := c.backendFor(cfg)
	var data any
	if err == nil {
		data, err = fetch(backend)
		if err != nil {
			// Force a rebuild (fresh Vault password, fresh digest state) on the next attempt.
			c.dropBackend(backend)
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	entry.attemptedAt = now
	entry.inflight = nil
	if err != nil {
		entry.err = err
		return
	}
	entry.err = nil
	entry.data = data
	entry.fetchedAt = now
}

// ---------------------------------------------------------------------------
// Section fetchers
// ---------------------------------------------------------------------------

func fritzWidgetFetchSystem(backend fritzWidgetBackend) (any, error) {
	info, err := backend.GetSystemInfo()
	if err != nil {
		return nil, err
	}
	return &fritzWidgetSystem{Model: info.ModelName, Firmware: info.SoftwareVersion, UptimeSeconds: info.Uptime}, nil
}

func fritzWidgetFetchConnection(backend fritzWidgetBackend) (any, error) {
	status, err := backend.GetWANStatus()
	if err != nil {
		return nil, err
	}
	out := &fritzWidgetConnection{
		Online:        status.Connected,
		Status:        status.Status,
		UptimeSeconds: status.Uptime,
		ExternalIPv4:  status.ExternalIPv4,
		ExternalIPv6:  status.ExternalIPv6,
		IPv6Prefix:    status.IPv6Prefix,
	}
	if status.LastError != "" && status.LastError != "ERROR_NONE" {
		out.LastError = status.LastError
	}
	// Link properties and the online monitor are best-effort enrichments.
	if link, err := backend.GetWANLinkInfo(); err == nil {
		out.AccessType = fritzWidgetAccessType(link.AccessType)
		out.LinkUp = link.LinkUp
		out.MaxDownBps = link.MaxDownstreamBps
		out.MaxUpBps = link.MaxUpstreamBps
		out.DownBps = link.ByteReceiveRate * 8
		out.UpBps = link.ByteSendRate * 8
		out.TotalSentBytes = link.TotalBytesSent
		out.TotalReceivedBytes = link.TotalBytesReceived
	}
	if monitor, err := backend.GetOnlineMonitor(); err == nil && len(monitor.DownstreamBytesPerS) > 0 {
		out.Monitor = &fritzWidgetMonitor{
			IntervalSeconds: monitor.IntervalSeconds,
			DownBps:         fritzWidgetBytesToBits(monitor.DownstreamBytesPerS),
			UpBps:           fritzWidgetBytesToBits(monitor.UpstreamBytesPerS),
		}
		// The newest monitor sample is the most accurate "current" throughput.
		out.DownBps = out.Monitor.DownBps[len(out.Monitor.DownBps)-1]
		if n := len(out.Monitor.UpBps); n > 0 {
			out.UpBps = out.Monitor.UpBps[n-1]
		}
		if out.MaxDownBps == 0 {
			out.MaxDownBps = monitor.MaxDownstreamBytesPerS * 8
		}
		if out.MaxUpBps == 0 {
			out.MaxUpBps = monitor.MaxUpstreamBytesPerS * 8
		}
	}
	return out, nil
}

func fritzWidgetFetchDevices(cfg *config.Config) func(fritzWidgetBackend) (any, error) {
	wantHosts := cfg.FritzBox.Network.SubFeatures.Hosts
	wantWLAN := cfg.FritzBox.Network.SubFeatures.WLAN
	return func(backend fritzWidgetBackend) (any, error) {
		out := &fritzWidgetDevices{Hosts: []fritzWidgetHost{}, WLANs: []fritzWidgetWLAN{}}
		var firstErr error
		if wantHosts {
			hosts, err := backend.GetHostList()
			if err != nil {
				firstErr = err
			} else {
				fritzWidgetFillHosts(out, hosts)
			}
		}
		if wantWLAN {
			answered := 0
			for index := 1; index <= 4; index++ {
				info, err := backend.GetWLANInfo(index)
				if err != nil {
					// Boxes expose 2–4 radios; the first missing index ends the probe.
					break
				}
				answered++
				out.WLANs = append(out.WLANs, fritzWidgetWLAN{
					Index:   info.Index,
					SSID:    fritzWidgetTrimName(info.SSID),
					Enabled: info.Enabled,
					Channel: atoiDefaultServer(info.Channel),
					Band:    fritzWidgetBand(info.Channel),
				})
			}
			if answered >= 2 {
				out.WLANs[len(out.WLANs)-1].Guest = true
			}
			if answered == 0 && !wantHosts && firstErr == nil {
				firstErr = fmt.Errorf("fritzbox widget: no WLAN radio answered")
			}
		}
		if firstErr != nil && (!wantHosts || len(out.Hosts) == 0) && len(out.WLANs) == 0 {
			return nil, firstErr
		}
		return out, nil
	}
}

func fritzWidgetFillHosts(out *fritzWidgetDevices, hosts []fritzbox.HostEntry) {
	list := make([]fritzWidgetHost, 0, len(hosts))
	for _, host := range hosts {
		iface := fritzWidgetInterface(host.Interface)
		entry := fritzWidgetHost{Name: fritzWidgetTrimName(host.Name), IP: strings.TrimSpace(host.IPAddress), Active: host.Active, Interface: iface}
		if entry.Name == "" {
			entry.Name = entry.IP
		}
		out.Total++
		if host.Active {
			out.Active++
			switch iface {
			case "lan":
				out.LANActive++
			case "wlan":
				out.WLANActive++
			}
		}
		list = append(list, entry)
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Active != list[j].Active {
			return list[i].Active
		}
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
	if len(list) > fritzWidgetMaxHosts {
		list = list[:fritzWidgetMaxHosts]
		out.HostsTruncated = true
	}
	out.Hosts = list
}

func fritzWidgetFetchTelephony(cfg *config.Config, now func() time.Time) func(fritzWidgetBackend) (any, error) {
	wantCalls := cfg.FritzBox.Telephony.SubFeatures.CallLists
	wantTAM := cfg.FritzBox.Telephony.SubFeatures.TAM
	return func(backend fritzWidgetBackend) (any, error) {
		out := &fritzWidgetTelephony{Calls: []fritzWidgetCall{}}
		var firstErr error
		if wantCalls {
			calls, err := backend.GetCallList()
			if err != nil {
				firstErr = err
			} else {
				today := now().Format("02.01.06")
				for _, call := range calls {
					kind := fritzWidgetCallType(call.Type)
					if kind == "missed" && strings.HasPrefix(strings.TrimSpace(call.Date), today) {
						out.MissedToday++
					}
					if len(out.Calls) >= fritzWidgetMaxCalls {
						continue
					}
					entry := fritzWidgetCall{
						Type:     kind,
						Date:     strings.TrimSpace(call.Date),
						Name:     fritzWidgetTrimName(call.Name),
						Number:   strings.TrimSpace(call.Number),
						Called:   strings.TrimSpace(call.Called),
						Duration: strings.TrimSpace(call.Duration),
					}
					if ts, err := time.ParseInLocation(fritzWidgetCallDateForm, entry.Date, time.Local); err == nil {
						entry.Timestamp = ts.Format(time.RFC3339)
					}
					out.Calls = append(out.Calls, entry)
				}
			}
		}
		if wantTAM {
			if messages, err := backend.GetTAMList(0); err == nil {
				out.TAMAvailable = true
				for _, message := range messages {
					if !message.Read {
						out.TAMNew++
					}
				}
			} else if !wantCalls {
				firstErr = err
			}
		}
		if firstErr != nil && len(out.Calls) == 0 && !out.TAMAvailable {
			return nil, firstErr
		}
		return out, nil
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func fritzWidgetBytesToBits(values []int64) []int64 {
	out := make([]int64, len(values))
	for i, v := range values {
		out[i] = v * 8
	}
	return out
}

func fritzWidgetAccessType(raw string) string {
	value := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(raw), "X_AVM-DE_"))
	switch value {
	case "dsl", "cable", "fiber", "ethernet":
		return value
	case "lte", "umts", "mobile", "5g":
		return "mobile"
	case "":
		return ""
	}
	return "other"
}

func fritzWidgetInterface(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case value == "ethernet" || strings.HasPrefix(value, "lan"):
		return "lan"
	case strings.HasPrefix(value, "802.11") || strings.Contains(value, "wlan") || strings.Contains(value, "wifi"):
		return "wlan"
	}
	return "other"
}

func fritzWidgetBand(channel string) string {
	ch := atoiDefaultServer(channel)
	switch {
	case ch <= 0:
		return ""
	case ch <= 14:
		return "2.4"
	case ch <= 177:
		return "5"
	}
	return "6"
}

func fritzWidgetCallType(raw string) string {
	switch strings.TrimSpace(raw) {
	case "1":
		return "incoming"
	case "2":
		return "missed"
	case "3":
		return "outgoing"
	case "9", "11":
		return "active"
	case "10":
		return "rejected"
	}
	return "unknown"
}

func fritzWidgetTrimName(raw string) string {
	value := strings.TrimSpace(raw)
	if utf8.RuneCountInString(value) <= fritzWidgetMaxNameRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:fritzWidgetMaxNameRunes-1]) + "…"
}

func atoiDefaultServer(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return v
}

func fritzWidgetParseSections(raw string, caps fritzWidgetCapabilities) []string {
	requested := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		if name := strings.ToLower(strings.TrimSpace(part)); name != "" {
			requested[name] = true
		}
	}
	out := make([]string, 0, len(fritzWidgetSectionOrder))
	for _, name := range fritzWidgetSectionOrder {
		if !caps.has(name) {
			continue
		}
		if len(requested) == 0 || requested[name] {
			out = append(out, name)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

func handleDesktopFritzBoxOverview(s *Server) http.HandlerFunc {
	return handleDesktopFritzBoxOverviewWithCache(s, newFritzWidgetCache(s))
}

func handleDesktopFritzBoxOverviewWithCache(s *Server, cache *fritzWidgetCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			jsonError(w, "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		// Scoped desktop readers must not learn external addresses, device names or calls.
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.VirtualDesktop.Enabled {
			jsonError(w, "desktop_disabled", http.StatusServiceUnavailable)
			return
		}
		if !cfg.FritzBox.Enabled || strings.TrimSpace(cfg.FritzBox.Host) == "" {
			jsonError(w, "fritzbox_disabled", http.StatusForbidden)
			return
		}
		caps := fritzWidgetCapabilitiesFor(cfg)
		sections := fritzWidgetParseSections(r.URL.Query().Get("sections"), caps)

		ctx, cancel := context.WithTimeout(r.Context(), fritzWidgetWaitTimeout)
		defer cancel()
		payload := map[string]any{
			"ready":        true,
			"capabilities": caps,
			"generated_at": cache.now().UTC().Format(time.RFC3339),
		}
		sectionErrors := map[string]string{}
		fetchedAt := map[string]string{}
		stale := map[string]bool{}
		for _, name := range sections {
			var (
				data any
				at   time.Time
				old  bool
				err  error
			)
			switch name {
			case "system":
				data, at, old, err = cache.section(ctx, cfg, name, fritzWidgetTTLSystem, true, fritzWidgetFetchSystem)
			case "connection":
				data, at, old, err = cache.section(ctx, cfg, name, fritzWidgetTTLConnection, true, fritzWidgetFetchConnection)
			case "devices":
				data, at, old, err = cache.section(ctx, cfg, name, fritzWidgetTTLDevices, false, fritzWidgetFetchDevices(cfg))
			case "telephony":
				data, at, old, err = cache.section(ctx, cfg, name, fritzWidgetTTLTelephony, false, fritzWidgetFetchTelephony(cfg, cache.now))
			}
			if data != nil {
				payload[name] = data
				fetchedAt[name] = at.UTC().Format(time.RFC3339)
			}
			if old {
				stale[name] = true
			}
			if err != nil {
				sectionErrors[name] = fritzWidgetErrorCode(err)
				if s != nil && s.Logger != nil {
					s.Logger.Debug("fritzbox widget section failed", "section", name, "error", err)
				}
			}
		}
		payload["errors"] = sectionErrors
		payload["fetched_at"] = fetchedAt
		payload["stale"] = stale
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_ = json.NewEncoder(w).Encode(payload)
	}
}

// fritzWidgetErrorCode maps transport errors to a small set of stable codes so
// the browser never receives raw error text that could echo hostnames.
func fritzWidgetErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "auth"):
		return "auth_failed"
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return "timeout"
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host") || strings.Contains(msg, "dial"):
		return "unreachable"
	}
	return "fetch_failed"
}
