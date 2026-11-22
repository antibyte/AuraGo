package server

import (
	"context"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus-community/pro-bing"

	"aurago/internal/infra"
)

const (
	// desktopInfraLivenessTTL is how long a probe result stays authoritative.
	desktopInfraLivenessTTL = 60 * time.Second
	// desktopInfraLivenessBudget bounds one probe pass.
	desktopInfraLivenessBudget = 12 * time.Second
	// desktopInfraLivenessHosts bounds how many addresses one pass probes.
	desktopInfraLivenessHosts = 64
	// desktopInfraLivenessWorkers bounds the parallel probes of one pass.
	desktopInfraLivenessWorkers = 8
	// desktopInfraLivenessPingTimeout bounds one ICMP echo.
	desktopInfraLivenessPingTimeout = 900 * time.Millisecond
	// desktopInfraLivenessDialTimeout bounds one TCP connect.
	desktopInfraLivenessDialTimeout = 400 * time.Millisecond
)

// desktopInfraLivenessPorts are the TCP ports tried when ICMP is blocked or
// filtered. They are the common management ports of the assets an atlas holds.
var desktopInfraLivenessPorts = []string{"22", "80", "443"}

// desktopInfraLivenessEntry is one probe result for one address.
type desktopInfraLivenessEntry struct {
	State     string
	CheckedAt time.Time
}

// desktopInfraLiveness is the read side of the probe cache: it answers with a
// fresh verdict, with "unknown" for an expired one and with "" when the
// address was never probed.
type desktopInfraLiveness struct {
	states map[string]desktopInfraLivenessEntry
}

// state returns the reachability verdict of an address.
func (l desktopInfraLiveness) state(address string) string {
	if l.states == nil || address == "" {
		return ""
	}
	entry, ok := l.states[address]
	if !ok {
		return ""
	}
	if time.Since(entry.CheckedAt) > desktopInfraLivenessTTL {
		// The verdict expired: the address is known, its current state is not.
		return infra.LiveUnknown
	}
	return entry.State
}

// desktopInfraLivenessView reads the cached probe results of the runtime.
func desktopInfraLivenessView(s *Server) desktopInfraLiveness {
	if s == nil {
		return desktopInfraLiveness{}
	}
	s.infraMu.Lock()
	runtime := s.infraRuntime
	s.infraMu.Unlock()
	if runtime == nil {
		return desktopInfraLiveness{}
	}
	return runtime.livenessView()
}

// livenessView copies the cached probe results.
func (r *desktopInfraRuntime) livenessView() desktopInfraLiveness {
	if r == nil {
		return desktopInfraLiveness{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	states := make(map[string]desktopInfraLivenessEntry, len(r.liveness))
	for address, entry := range r.liveness {
		states[address] = entry
	}
	return desktopInfraLiveness{states: states}
}

// livenessStale reports whether the next pass may probe again.
func (r *desktopInfraRuntime) livenessStale(now time.Time) bool {
	if r == nil {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.livenessRunning || now.Sub(r.livenessAt) >= desktopInfraLivenessTTL
}

// beginLiveness claims the pass lock. It returns the offset that rotates the
// addresses (a pass that does not reach every address hands the start to the
// next pass) and the configuration key the answers belong to.
func (r *desktopInfraRuntime) beginLiveness(total int) (int, string, bool) {
	if r == nil {
		return 0, "", false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.livenessRunning {
		return 0, "", false
	}
	r.livenessRunning = true
	r.livenessPass++
	offset := 0
	if total > 0 {
		offset = int(r.livenessPass % uint64(total))
	}
	return offset, r.configKey, true
}

// setLiveness stores the results of a pass. Answers of a pass that ran under a
// replaced configuration are dropped: they describe the previous one.
func (r *desktopInfraRuntime) setLiveness(states map[string]desktopInfraLivenessEntry, at time.Time, configKey string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.livenessRunning = false
	if configKey != r.configKey {
		return
	}
	r.liveness = states
	r.livenessAt = at
}

// desktopInfraRefreshLiveness probes the addresses of the visible nodes and
// caches their reachability. The addresses are read right away, the probes
// run beside the request: a refresh must not wait for the network. A host
// that cannot be probed keeps its previous verdict until that verdict expires.
func desktopInfraRefreshLiveness(ctx context.Context, store *infra.Store, runtime *desktopInfraRuntime, force bool) {
	if runtime == nil || store == nil {
		return
	}
	if !force && !runtime.livenessStale(time.Now()) {
		return
	}
	addresses := desktopInfraLivenessAddresses(ctx, store)
	if len(addresses) == 0 {
		return
	}
	offset, configKey, ok := runtime.beginLiveness(len(addresses))
	if !ok {
		return
	}
	addresses = desktopInfraRotateAddresses(addresses, offset)
	go func() {
		// The pass outlives the request that started it, but not its budget. Its
		// answers are dropped when the configuration changed while it ran.
		passCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), desktopInfraLivenessBudget)
		defer cancel()
		results := desktopInfraProbeAddresses(passCtx, addresses)
		runtime.setLiveness(results, time.Now(), configKey)
	}()
}

// desktopInfraRotateAddresses moves the start of the list forward, so a pass
// that does not reach every address leaves the rest to the next pass.
func desktopInfraRotateAddresses(addresses []string, offset int) []string {
	if offset <= 0 || offset >= len(addresses) {
		return addresses
	}
	rotated := make([]string, 0, len(addresses))
	rotated = append(rotated, addresses[offset:]...)
	rotated = append(rotated, addresses[:offset]...)
	return rotated
}

// desktopInfraLivenessAddresses lists the addresses of the visible nodes that
// can be probed: a device, guest or service with an address of its own.
func desktopInfraLivenessAddresses(ctx context.Context, store *infra.Store) []string {
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		return nil
	}
	local := desktopInfraLocalAddressSet()
	seen := make(map[string]bool)
	addresses := make([]string, 0, len(snapshot.Nodes))
	for _, node := range desktopInfraVisibleNodes(&snapshot) {
		switch node.Kind {
		case infra.NodeDevice, infra.NodeVM, infra.NodeContainer, infra.NodeService:
		default:
			continue
		}
		address := desktopInfraNodeAddress(node)
		if address == "" || seen[address] || local[address] {
			continue
		}
		seen[address] = true
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)
	if len(addresses) > desktopInfraLivenessHosts {
		addresses = addresses[:desktopInfraLivenessHosts]
	}
	return addresses
}

// desktopInfraLocalAddressSet lists the addresses of the AuraGo host itself:
// probing them would report the agent as one of its own assets.
func desktopInfraLocalAddressSet() map[string]bool {
	local := map[string]bool{"127.0.0.1": true, "::1": true}
	for _, address := range desktopInfraLocalAddresses() {
		local[address.Unmap().String()] = true
	}
	return local
}

// desktopInfraNodeAddress returns the first usable address of a node.
func desktopInfraNodeAddress(node infra.Node) string {
	probeIP := func(value string) string {
		ip := net.ParseIP(strings.TrimSpace(value))
		if !ip.IsGlobalUnicast() {
			return ""
		}
		return ip.String()
	}
	if address := probeIP(node.Profile.IPAddress); address != "" {
		return address
	}
	for _, iface := range node.Profile.Interfaces {
		if address := probeIP(iface.IPAddress); address != "" {
			return address
		}
	}
	for _, source := range node.Sources {
		if address := probeIP(source.Observed.IPAddress); address != "" {
			return address
		}
	}
	return ""
}

// desktopInfraProbeAddresses probes the addresses with a bounded worker pool
// and returns the fresh verdicts. Addresses that the budget never reached keep
// no entry and therefore stay without a verdict.
func desktopInfraProbeAddresses(ctx context.Context, addresses []string) map[string]desktopInfraLivenessEntry {
	results := make(map[string]desktopInfraLivenessEntry, len(addresses))
	var mu sync.Mutex
	jobs := make(chan string)
	workers := desktopInfraLivenessWorkers
	if workers > len(addresses) {
		workers = len(addresses)
	}
	var wait sync.WaitGroup
	wait.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wait.Done()
			for address := range jobs {
				state := infra.LiveOffline
				if desktopInfraProbeAddress(ctx, address) {
					state = infra.LiveLive
				}
				mu.Lock()
				results[address] = desktopInfraLivenessEntry{State: state, CheckedAt: time.Now()}
				mu.Unlock()
			}
		}()
	}
	for _, address := range addresses {
		select {
		case jobs <- address:
		case <-ctx.Done():
			close(jobs)
			wait.Wait()
			return results
		}
	}
	close(jobs)
	wait.Wait()
	return results
}

// desktopInfraProbeAddress reports whether the address answers right now. It
// asks ICMP first and falls back to a TCP connect on the management ports,
// because a firewall often drops echo requests but still serves the device.
func desktopInfraProbeAddress(ctx context.Context, address string) bool {
	if desktopInfraProbePing(ctx, address) {
		return true
	}
	return desktopInfraProbeTCP(ctx, address)
}

// desktopInfraProbePing sends one ICMP echo request.
func desktopInfraProbePing(ctx context.Context, address string) bool {
	pinger, err := probing.NewPinger(address)
	if err != nil {
		return false
	}
	pinger.Count = 1
	pinger.Timeout = desktopInfraLivenessPingTimeout
	pinger.SetPrivileged(true)
	if err := pinger.RunWithContext(ctx); err != nil {
		// A host that forbids raw sockets still answers an unprivileged echo.
		fallback, fallbackErr := probing.NewPinger(address)
		if fallbackErr != nil {
			return false
		}
		fallback.Count = 1
		fallback.Timeout = desktopInfraLivenessPingTimeout
		fallback.SetPrivileged(false)
		if err := fallback.RunWithContext(ctx); err != nil {
			return false
		}
		pinger = fallback
	}
	stats := pinger.Statistics()
	return stats != nil && stats.PacketsRecv > 0
}

// desktopInfraProbeTCP tries the management ports of an address.
func desktopInfraProbeTCP(ctx context.Context, address string) bool {
	dialer := net.Dialer{Timeout: desktopInfraLivenessDialTimeout}
	for _, port := range desktopInfraLivenessPorts {
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(address, port))
		if err != nil {
			continue
		}
		_ = conn.Close()
		return true
	}
	return false
}

// desktopInfraDerivedLive reads the reachability a provider already states: a
// running guest or an active FRITZ!Box host is live, a stopped one is not.
// Sources that disagree leave the state unknown.
func desktopInfraDerivedLive(sources []infra.Source) string {
	chosen := ""
	for _, source := range sources {
		if source.State != infra.SourceCurrent {
			continue
		}
		switch source.Observed.Power {
		case infra.PowerRunning, infra.PowerStopped, infra.PowerPaused:
		default:
			continue
		}
		if chosen == "" {
			chosen = source.Observed.Power
			continue
		}
		if chosen != source.Observed.Power {
			return infra.LiveUnknown
		}
	}
	switch chosen {
	case infra.PowerRunning:
		return infra.LiveLive
	case infra.PowerStopped:
		return infra.LiveOffline
	case infra.PowerPaused:
		return infra.LiveUnknown
	}
	return ""
}

// desktopInfraProjectLive projects the reachability of a node. A live signal
// always wins over an offline one: a device that answers is reachable even
// when a provider still lists it as stopped, and a stopped device stays
// offline even when its address no longer answers.
func desktopInfraProjectLive(node infra.Node, liveness desktopInfraLiveness) string {
	derived := desktopInfraDerivedLive(node.Sources)
	probed := liveness.state(desktopInfraNodeAddress(node))
	if derived == infra.LiveLive || probed == infra.LiveLive {
		return infra.LiveLive
	}
	if derived == infra.LiveOffline || probed == infra.LiveOffline {
		return infra.LiveOffline
	}
	if derived == infra.LiveUnknown || probed == infra.LiveUnknown {
		return infra.LiveUnknown
	}
	return ""
}
