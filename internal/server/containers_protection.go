package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"aurago/internal/acestep"
	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// Confirmation contract of the administrator container API: terminal, update
// and remove on a protected container need ?confirm=protected.
const (
	containerConfirmParam              = "confirm"
	containerConfirmProtected          = "protected"
	containerCodeConfirmationRequired  = "container_protected_confirmation_required"
	containerCodeSelfUpdateUnsupported = "container_self_update_unsupported"
)

// containerProtectedOwners is the owner list the agent docker tool protects
// (internal/agent/agent_dispatch_services.go, tools.DockerContainerOwnership
// call). The app owner comes first so the app container names itself.
var containerProtectedOwners = []string{
	dockerutil.AppOwner,
	acestep.Owner,
	dockerutil.HomepageOwner,
	"go2rtc",
	dockerutil.LocalLLMOwner,
	dockerutil.BoringGarageOwner,
}

// containerProtection says why an administrator terminal, update or remove on
// a container needs an explicit confirmation or cannot work at all.
type containerProtection struct {
	Owner          string // AuraGo owner that manages the container, "" when none
	Self           bool   // the container this AuraGo process runs in
	DockerEndpoint bool   // the container that serves AuraGo's Docker endpoint
	SharedNetwork  bool   // AuraGo runs in it or shares its network namespace (see containerSelfInList)
	Unverified     bool   // Docker gave no usable answer for one of the checks
}

func (p containerProtection) protected() bool {
	return p.Owner != "" || p.Self || p.DockerEndpoint || p.SharedNetwork || p.Unverified
}

// updateCannotComplete reports whether the stop-rename-create-start update of
// tools.DockerUpdateContainerImage would stop the process or the Docker
// connection that has to finish it.
func (p containerProtection) updateCannotComplete() bool {
	return p.Self || p.DockerEndpoint
}

// label is the most specific reason; it is sent as the "owner" field.
func (p containerProtection) label() string {
	switch {
	case p.Self:
		return "self"
	case p.DockerEndpoint:
		return "docker-endpoint"
	case p.SharedNetwork:
		return "shared-network"
	case p.Owner != "":
		return p.Owner
	case p.Unverified:
		return "unverified"
	default:
		return ""
	}
}

// confirmationMessage names the reason of label() for the 409 answer.
func (p containerProtection) confirmationMessage() string {
	const repeat = " Repeat the request with confirm=protected to continue."
	switch label := p.label(); label {
	case "self":
		return "AuraGo runs in this container." + repeat
	case "docker-endpoint":
		return "AuraGo reaches Docker through this container." + repeat
	case "shared-network":
		return "AuraGo runs in this container or shares its network namespace with it." + repeat
	case "unverified":
		return "Docker did not answer the ownership check for this container." + repeat
	default:
		return "AuraGo manages this container (" + label + ")." + repeat
	}
}

// containerSelfHostname is os.Hostname; tests replace it.
var containerSelfHostname = os.Hostname

// containerSelfProcFile reads /proc/self/mountinfo and /proc/self/cgroup;
// tests replace it with fixtures.
var containerSelfProcFile = os.ReadFile

// dockerDefaultHostnamePattern matches the hostname Docker assigns by default:
// the first twelve hex characters of the container ID.
var dockerDefaultHostnamePattern = regexp.MustCompile(`^[0-9a-f]{12,64}$`)

// mountinfoContainerIDPattern matches the mount root of the files Docker
// bind-mounts into every container from <data-root>/containers/<id>/. The
// root is /<id>/... when that containers directory is its own filesystem.
// containerIDFromMountinfo only applies it to mounts at the three /etc files.
var mountinfoContainerIDPattern = regexp.MustCompile(`(?:^|/)([0-9a-f]{64})/(?:hostname|hosts|resolv\.conf)$`)

// cgroupContainerIDPattern matches the cgroup v1 path of a Docker container,
// both the cgroupfs (/docker/<id>) and the systemd (docker-<id>.scope) form.
var cgroupContainerIDPattern = regexp.MustCompile(`(?:/docker/|/docker-)([0-9a-f]{64})(?:\.scope)?$`)

// containerSelfSignals are the two facts that can name the container this
// process runs in: Docker's default hostname (a prefix of the container ID)
// and AuraGo's own container ID read from /proc, which survives a compose
// `hostname:` override. Both name the provider when AuraGo joins another
// container's network namespace; containerSelfInList rules that out.
//
// Podman is not covered: the runtime probe needs /.dockerenv, which Podman
// does not create, and Podman keeps the /etc files under
// overlay-containers/<id>/userdata/. Self is never proven there; the app
// container falls back to the reserved-name/label confirmation.
type containerSelfSignals struct {
	hostname string // the default Docker hostname, "" when the hostname is not one
	ownID    string // the 64-hex container ID from /proc, "" when /proc names none
}

// readContainerSelfSignals reads the hostname and /proc once; callers keep the
// result for the whole request. A native runtime has no signals.
func readContainerSelfSignals(isDocker bool) containerSelfSignals {
	if !isDocker {
		return containerSelfSignals{}
	}
	var signals containerSelfSignals
	if hostname, err := containerSelfHostname(); err == nil {
		hostname = strings.ToLower(strings.TrimSpace(hostname))
		if dockerDefaultHostnamePattern.MatchString(hostname) {
			signals.hostname = hostname
		}
	}
	signals.ownID = ownContainerID()
	return signals
}

// names reports whether the signals name the container with fullID.
func (s containerSelfSignals) names(fullID string) bool {
	id := strings.ToLower(strings.TrimSpace(fullID))
	if id == "" {
		return false
	}
	if s.hostname != "" && strings.HasPrefix(id, s.hostname) {
		return true
	}
	return s.ownID != "" && s.ownID == id
}

// containerIsSelf reports whether the self signals name fullID. It does not
// rule out a shared network namespace; classification goes through
// containerSelfInList.
func containerIsSelf(isDocker bool, fullID string) bool {
	return readContainerSelfSignals(isDocker).names(fullID)
}

// ownContainerID returns the 64-hex ID of the container this process runs in,
// or "" when /proc does not name exactly one. mountinfo comes first because it
// works for cgroup v1 and v2; /proc/self/cgroup names the container only on
// cgroup v1.
func ownContainerID() string {
	if data, err := containerSelfProcFile("/proc/self/mountinfo"); err == nil {
		if id := containerIDFromMountinfo(string(data)); id != "" {
			return id
		}
	}
	if data, err := containerSelfProcFile("/proc/self/cgroup"); err == nil {
		return containerIDFromCgroup(string(data))
	}
	return ""
}

// containerIDFromMountinfo reads the container ID from the /etc/hostname,
// /etc/hosts and /etc/resolv.conf bind mounts (mount point = field 5, mount
// root = field 4 of /proc/self/mountinfo). Other mounts are ignored; lines that
// disagree on the ID prove nothing.
func containerIDFromMountinfo(text string) string {
	found := ""
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		switch fields[4] {
		case "/etc/hostname", "/etc/hosts", "/etc/resolv.conf":
		default:
			continue
		}
		match := mountinfoContainerIDPattern.FindStringSubmatch(fields[3])
		if match == nil {
			continue
		}
		if found != "" && found != match[1] {
			return ""
		}
		found = match[1]
	}
	return found
}

// containerIDFromCgroup reads the container ID from cgroup v1 lines
// (hierarchy:controllers:path); lines that disagree prove nothing.
func containerIDFromCgroup(text string) string {
	found := ""
	for _, line := range strings.Split(text, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(parts) != 3 {
			continue
		}
		match := cgroupContainerIDPattern.FindStringSubmatch(parts[2])
		if match == nil {
			continue
		}
		if found != "" && found != match[1] {
			return ""
		}
		found = match[1]
	}
	return found
}

// containerIDPrefixPattern is the shortest ID prefix a network-mode reference
// may use to name a container: 12 hex characters, as in Docker's short ID.
var containerIDPrefixPattern = regexp.MustCompile(`^[0-9a-f]{12,64}$`)

// containerNetworkModeJoins reports whether a HostConfig.NetworkMode joins the
// network namespace of the container with fullID and names. Docker may record
// the reference as a full ID, an ID prefix (only 12 or more hex characters
// count) or a name; compose's service:<x> is sent to the Engine as
// container:<id>.
func containerNetworkModeJoins(mode, fullID string, names []string) bool {
	ref, ok := strings.CutPrefix(strings.TrimSpace(mode), "container:")
	ref = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ref), "/"))
	if !ok || ref == "" {
		return false
	}
	if id := strings.ToLower(strings.TrimSpace(fullID)); id != "" && (id == ref || (containerIDPrefixPattern.MatchString(ref) && strings.HasPrefix(id, ref))) {
		return true
	}
	for _, name := range names {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(name), "/"), ref) {
			return true
		}
	}
	return false
}

// containerSelfInList applies the self signals to a full container list. The
// container the signals name is proven self only when no other container
// joins its network namespace: Docker gives every container that does
// (network_mode container:<x> or service:<x>, e.g. a Tailscale or Gluetun
// sidecar) the provider's hostname and /etc files, so the signals cannot tell
// AuraGo from its network provider. Then the named container and every
// container that joins it are marked SharedNetwork: a confirmation instead of
// the self-update refusal. Keys are lower-case full IDs.
func containerSelfInList(entries []tools.DockerContainerListEntry, signals containerSelfSignals) (self, shared map[string]bool) {
	self, shared = map[string]bool{}, map[string]bool{}
	for _, candidate := range entries {
		if !signals.names(candidate.FullID) {
			continue
		}
		candidateID := strings.ToLower(candidate.FullID)
		joined := false
		for _, other := range entries {
			otherID := strings.ToLower(other.FullID)
			if otherID == candidateID || !containerNetworkModeJoins(other.NetworkMode, candidate.FullID, candidate.Names) {
				continue
			}
			joined = true
			shared[otherID] = true
		}
		if joined {
			shared[candidateID] = true
		} else {
			self[candidateID] = true
		}
	}
	return self, shared
}

// containerEndpointLookup resolves a Docker endpoint host name; tests replace it.
var containerEndpointLookup = net.DefaultResolver.LookupHost

// containerDockerEndpointAddresses is dockerEndpointAddresses; handler tests
// replace it because their fake engine listens on loopback.
var containerDockerEndpointAddresses = dockerEndpointAddresses

// dockerEndpointAddresses returns the addresses AuraGo connects to for a TCP
// Docker endpoint, e.g. the docker-proxy service address for
// tcp://docker-proxy:2375. Socket and named-pipe endpoints, localhost, and
// loopback or unspecified addresses return none: no container is identified.
// An unparsable host or a failed lookup is an error, never "no endpoint
// container".
func dockerEndpointAddresses(ctx context.Context, dockerHost string) ([]string, error) {
	host := strings.TrimSpace(dockerHost)
	if host == "" || strings.HasPrefix(host, "unix://") || strings.HasPrefix(host, "npipe://") {
		return nil, nil
	}
	if !strings.Contains(host, "://") {
		host = "tcp://" + host
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parse docker host: %w", err)
	}
	name := strings.TrimSuffix(parsed.Hostname(), ".")
	if name == "" || strings.EqualFold(name, "localhost") {
		return nil, nil
	}
	candidates := []string{name}
	if net.ParseIP(name) == nil {
		lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		resolved, err := containerEndpointLookup(lookupCtx, name)
		if err != nil {
			return nil, fmt.Errorf("resolve docker host %q: %w", name, err)
		}
		candidates = resolved
	}
	var out []string
	for _, candidate := range candidates {
		ip := net.ParseIP(strings.TrimSpace(candidate))
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}
		out = append(out, ip.String())
	}
	return out, nil
}

// The list path (GET /api/containers) reuses one Docker endpoint lookup for
// containerEndpointCacheTTL and warns about a failed lookup once per failure
// streak, then at most every containerEndpointWarnInterval. Terminal, update
// and remove (classifyContainerForAction) always resolve again.
const (
	containerEndpointCacheTTL     = 30 * time.Second
	containerEndpointWarnInterval = 10 * time.Minute
)

// containerEndpointNow is time.Now; tests move the clock.
var containerEndpointNow = time.Now

type containerEndpointLookupCache struct {
	mu       sync.Mutex
	host     string
	addrs    []string
	err      error
	expires  time.Time
	warnHost string
	warnedAt time.Time
}

// listEndpointLookup is the lookup cache of the administrator list.
var listEndpointLookup containerEndpointLookupCache

// addresses returns the cached lookup for host, or resolves and caches it. A
// lookup cut short by the caller's cancellation is not cached.
func (c *containerEndpointLookupCache) addresses(ctx context.Context, host string) ([]string, error) {
	now := containerEndpointNow()
	c.mu.Lock()
	if c.host == host && now.Before(c.expires) {
		addrs, err := slices.Clone(c.addrs), c.err
		c.mu.Unlock()
		return addrs, err
	}
	c.mu.Unlock()
	addrs, err := containerDockerEndpointAddresses(ctx, host)
	if err != nil && ctx.Err() != nil {
		return addrs, err
	}
	c.mu.Lock()
	c.host, c.addrs, c.err, c.expires = host, slices.Clone(addrs), err, now.Add(containerEndpointCacheTTL)
	if err == nil && c.warnHost == host {
		c.warnHost = "" // the next failure starts a new streak and warns at once
	}
	c.mu.Unlock()
	return addrs, err
}

// shouldWarn reports whether a failed lookup for host is logged now.
func (c *containerEndpointLookupCache) shouldWarn(host string) bool {
	now := containerEndpointNow()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.warnHost == host && now.Sub(c.warnedAt) < containerEndpointWarnInterval {
		return false
	}
	c.warnHost, c.warnedAt = host, now
	return true
}

// containerTraceConnIP returns a context that records the remote IP of the
// connection the next Docker request uses, and a reader for it; tests replace it.
var containerTraceConnIP = traceDockerConnRemoteIP

func traceDockerConnRemoteIP(ctx context.Context) (context.Context, func() string) {
	var mu sync.Mutex
	var ip string
	trace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			if info.Conn == nil {
				return
			}
			addr, ok := info.Conn.RemoteAddr().(*net.TCPAddr)
			if !ok || addr == nil {
				return
			}
			mu.Lock()
			ip = addr.IP.String()
			mu.Unlock()
		},
	}
	return httptrace.WithClientTrace(ctx, trace), func() string {
		mu.Lock()
		defer mu.Unlock()
		return ip
	}
}

// dockerEndpointFromConnection turns the remote IP of the connection AuraGo's
// Docker requests use into endpoint addresses. ok is false when no TCP
// connection was observed; a loopback or unspecified address names no
// container, as on the lookup path.
func dockerEndpointFromConnection(ip string) (addrs []string, ok bool) {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return nil, false
	}
	if parsed.IsLoopback() || parsed.IsUnspecified() {
		return nil, true
	}
	return []string{parsed.String()}, true
}

// observedDockerEndpoint pings Docker and returns the endpoint addresses that
// the ping's connection names; ok is false when it observed no TCP connection.
func observedDockerEndpoint(ctx context.Context, cfg tools.DockerConfig) ([]string, bool) {
	pingCtx, connIP := containerTraceConnIP(ctx)
	if _, code, err := tools.DockerRequestContext(pingCtx, cfg, http.MethodGet, "/_ping", ""); err != nil || code != http.StatusOK {
		return nil, false
	}
	return dockerEndpointFromConnection(connIP())
}

// containersAtAddresses returns the listed containers that have one of addrs.
func containersAtAddresses(entries []tools.DockerContainerListEntry, addrs []string) []tools.DockerContainerListEntry {
	var out []tools.DockerContainerListEntry
	for _, entry := range entries {
		if containerServesDockerEndpoint(addrs, entry.NetworkIPs) {
			out = append(out, entry)
		}
	}
	return out
}

const composeServiceLabel = "com.docker.compose.service"

// sameComposeServiceName reports whether two containers carry the same
// compose service name, in any project: replicas of one service and
// same-named services of other projects on a shared network answer under one
// DNS alias.
func sameComposeServiceName(a, b map[string]string) bool {
	service := strings.TrimSpace(a[composeServiceLabel])
	return service != "" && service == strings.TrimSpace(b[composeServiceLabel])
}

// dockerHostName returns the lower-case host name of a TCP docker.host, or ""
// for sockets, named pipes, IP addresses and localhost.
func dockerHostName(dockerHost string) string {
	host := strings.TrimSpace(dockerHost)
	if host == "" || strings.HasPrefix(host, "unix://") || strings.HasPrefix(host, "npipe://") {
		return ""
	}
	if !strings.Contains(host, "://") {
		host = "tcp://" + host
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return ""
	}
	name := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if name == "" || name == "localhost" || net.ParseIP(name) != nil {
		return ""
	}
	return name
}

// containerAnswersDockerHostName reports whether one of a container's names
// (container name, compose service, network alias or DNS name) equals the host
// name of docker.host or its first label, so the container may answer it.
func containerAnswersDockerHostName(dockerHost string, names []string) bool {
	host := dockerHostName(dockerHost)
	if host == "" {
		return false
	}
	first, _, _ := strings.Cut(host, ".")
	for _, name := range names {
		name = strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(name), "/"), "."))
		if name != "" && (name == host || name == first) {
			return true
		}
	}
	return false
}

// containerActionInspect is the part of a container inspect that
// classifyContainerForAction reads.
type containerActionInspect struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	HostConfig struct {
		NetworkMode string `json:"NetworkMode"`
	} `json:"HostConfig"`
	GraphDriver struct {
		Data map[string]string `json:"Data"`
	} `json:"GraphDriver"`
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress         string   `json:"IPAddress"`
			GlobalIPv6Address string   `json:"GlobalIPv6Address"`
			Aliases           []string `json:"Aliases"`
			DNSNames          []string `json:"DNSNames"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

// endpointFallbackTargetFromInspect collects what dockerEndpointByConnection
// needs from a target's inspect: its addresses, labels and network mode, and
// whether its container name, compose service, a network alias or a DNS name
// answers the host name of dockerHost.
func endpointFallbackTargetFromInspect(info containerActionInspect, containerID, dockerHost string) endpointFallbackTarget {
	ips := make([]string, 0, 2*len(info.NetworkSettings.Networks))
	hostNames := []string{info.Name, containerID, info.Config.Labels[composeServiceLabel]}
	for _, network := range info.NetworkSettings.Networks {
		ips = append(ips, network.IPAddress, network.GlobalIPv6Address)
		hostNames = append(hostNames, network.Aliases...)
		hostNames = append(hostNames, network.DNSNames...)
	}
	return endpointFallbackTarget{
		ID:          info.ID,
		IPs:         ips,
		Labels:      info.Config.Labels,
		NetworkMode: info.HostConfig.NetworkMode,
		AnswersHost: containerAnswersDockerHostName(dockerHost, hostNames),
	}
}

// endpointFallbackTarget is what dockerEndpointByConnection knows about the
// target from its inspect.
type endpointFallbackTarget struct {
	ID          string
	IPs         []string
	Labels      map[string]string
	NetworkMode string // HostConfig.NetworkMode, e.g. "container:<id>"
	AnswersHost bool   // a name, compose service, alias or DNS name equals the docker.host name
}

// dockerEndpointByConnection classifies a target against the Docker endpoint
// when the docker.host lookup failed. The remote IP of AuraGo's own Docker
// connection proves which container serves the endpoint only when the target
// or a listed container has that address. verified is false in every other
// case, so the target stays unverified exactly as without this fallback: no
// TCP connection observed, a loopback or host address (a proxy behind a
// published port stays unknown), a failed list, a target that answers the
// docker.host name, a target in the connected container's network namespace
// (it may be the process serving the port), or a target with the connected
// container's compose service name in any project (it may answer the name too).
func dockerEndpointByConnection(connIP string, target endpointFallbackTarget, list func() ([]tools.DockerContainerListEntry, bool)) (endpoint, verified bool) {
	observed, ok := dockerEndpointFromConnection(connIP)
	if !ok || len(observed) == 0 {
		return false, false
	}
	if containerServesDockerEndpoint(observed, target.IPs) {
		return true, true
	}
	if target.AnswersHost {
		return false, false
	}
	entries, ok := list()
	if !ok {
		return false, false
	}
	connected := containersAtAddresses(entries, observed)
	if len(connected) == 0 {
		return false, false
	}
	for _, entry := range connected {
		if target.ID != "" && strings.EqualFold(entry.FullID, target.ID) {
			return true, true
		}
		if containerNetworkModeJoins(target.NetworkMode, entry.FullID, entry.Names) ||
			sameComposeServiceName(entry.Labels, target.Labels) {
			return false, false
		}
	}
	return false, true
}

// containerServesDockerEndpoint reports whether one of the container's network
// addresses is an address AuraGo reaches its Docker endpoint at. Compose service
// names and network aliases resolve alike; an endpoint reached through a
// host-published port is not attributed to any container.
func containerServesDockerEndpoint(endpoint, containerIPs []string) bool {
	for _, candidate := range containerIPs {
		ip := net.ParseIP(strings.TrimSpace(candidate))
		if ip == nil {
			continue
		}
		for _, addr := range endpoint {
			if ip.Equal(net.ParseIP(addr)) {
				return true
			}
		}
	}
	return false
}

func firstContainerOwner(names []string, labels map[string]string) string {
	owned := tools.DockerContainerOwnersFromMetadata(names, labels, containerProtectedOwners...)
	for _, owner := range containerProtectedOwners {
		if owned[owner] {
			return owner
		}
	}
	return ""
}

func containerRuntimeIsDocker(s *Server) bool {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return s.Cfg != nil && s.Cfg.Runtime.IsDocker
}

// containerProtectionFor classifies the target of a terminal, update or remove
// request. It is a variable so handler tests without a Docker engine can
// substitute a fixed answer.
var containerProtectionFor = classifyContainerForAction

func classifyContainerForAction(ctx context.Context, s *Server, cfg tools.DockerConfig, containerID string) containerProtection {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	// The inspect's connection names the address AuraGo's Docker requests
	// reach; it stands in for the endpoint lookup when docker.host does not
	// resolve (dockerEndpointByConnection).
	inspectCtx, connIP := containerTraceConnIP(ctx)
	data, code, err := tools.DockerRequestContext(inspectCtx, cfg, http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/json", "")
	if err == nil && code == http.StatusNotFound {
		// No such container: the action reports it. A reserved name still
		// names its owner, as on the error path below.
		return containerProtection{Owner: firstContainerOwner([]string{containerID}, nil)}
	}
	var info containerActionInspect
	if err != nil || code != http.StatusOK || json.Unmarshal(data, &info) != nil {
		// Fail toward a confirmation, like tools.DockerContainerManagedBy: a
		// reserved name still names its owner, anything else is unverified.
		return containerProtection{Owner: firstContainerOwner([]string{containerID}, nil), Unverified: true}
	}
	p := containerProtection{Owner: firstContainerOwner([]string{info.Name, containerID}, info.Config.Labels)}

	target := endpointFallbackTargetFromInspect(info, containerID, cfg.Host)
	ips := target.IPs
	// At most one container list per request: the endpoint fallback and the
	// self check share it.
	var listEntries []tools.DockerContainerListEntry
	listLoaded, listOK := false, false
	listContainers := func() ([]tools.DockerContainerListEntry, bool) {
		if !listLoaded {
			var failure string
			listEntries, failure = tools.DockerListContainerEntries(cfg, true)
			listLoaded, listOK = true, failure == ""
		}
		return listEntries, listOK
	}

	if endpoint, err := containerDockerEndpointAddresses(ctx, cfg.Host); err == nil {
		p.DockerEndpoint = containerServesDockerEndpoint(endpoint, ips)
	} else if isEndpoint, verified := dockerEndpointByConnection(connIP(), target, listContainers); verified {
		// docker.host did not resolve, but AuraGo's own Docker connection
		// names the endpoint container.
		p.DockerEndpoint = isEndpoint
	} else {
		// Neither the lookup nor the connection rules the endpoint container
		// out: ask instead of allowing.
		p.Unverified = true
	}

	signals := readContainerSelfSignals(containerRuntimeIsDocker(s))
	if containerUpperDirProvesSelf(containerRuntimeIsDocker(s), info.GraphDriver.Data["UpperDir"]) {
		// The overlay upper directory is unique per container and does not
		// change with network sharing: this is the container AuraGo runs in
		// (containers_self_proof.go).
		p.Self = true
		return p
	}
	named := signals.names(info.ID)
	joinsOther := strings.HasPrefix(strings.TrimSpace(info.HostConfig.NetworkMode), "container:")
	if signals == (containerSelfSignals{}) || (!named && !joinsOther) {
		return p
	}
	// The signals name this container, or it joins another container's network
	// namespace: one list request decides between self and a shared namespace.
	entries, ok := listContainers()
	if !ok {
		p.Unverified = true
		return p
	}
	id := strings.ToLower(info.ID)
	listed := false
	for _, entry := range entries {
		if strings.ToLower(entry.FullID) == id {
			listed = true
			break
		}
	}
	if !listed {
		p.Unverified = true
		return p
	}
	self, shared := containerSelfInList(entries, signals)
	p.Self = self[id]
	p.SharedNetwork = shared[id]
	if p.SharedNetwork && proveSelfInSharedGroup(ctx, cfg, containerRuntimeIsDocker(s), shared) == id {
		// Proven by a signal that survives network sharing (containers_self_proof.go).
		p.Self, p.SharedNetwork = true, false
	}
	return p
}

// containerActionAllowed applies the confirmation rules for terminal, update
// and remove. It writes the refusal and returns false when the request stops.
// start, stop, restart, logs, inspect and stats never call it.
func containerActionAllowed(s *Server, cfg tools.DockerConfig, containerID, action string, w http.ResponseWriter, r *http.Request) bool {
	p := containerProtectionFor(r.Context(), s, cfg, containerID)
	if action == "update" && p.updateCannotComplete() {
		message := "AuraGo cannot update the container it runs in: stopping it ends this process before the replacement exists. Run docker compose pull && docker compose up -d on the Docker host."
		if !p.Self {
			message = "AuraGo cannot update the container that serves its Docker endpoint: stopping it cuts the connection before the replacement exists. Run docker compose pull && docker compose up -d on the Docker host."
		}
		containerJSON(w, http.StatusConflict, map[string]string{"status": "error", "code": containerCodeSelfUpdateUnsupported, "owner": p.label(), "message": message})
		return false
	}
	if p.protected() && r.URL.Query().Get(containerConfirmParam) != containerConfirmProtected {
		containerJSON(w, http.StatusConflict, map[string]string{"status": "error", "code": containerCodeConfirmationRequired, "owner": p.label(), "message": p.confirmationMessage()})
		return false
	}
	return true
}

// adminContainerEntry is a list entry plus the protection flags the Containers
// page needs before it offers a terminal, update or remove.
type adminContainerEntry struct {
	tools.DockerContainerListEntry
	ProtectedOwner string `json:"protected_owner,omitempty"`
	Self           bool   `json:"self,omitempty"`
	DockerEndpoint bool   `json:"docker_endpoint,omitempty"`
	SharedNetwork  bool   `json:"shared_network,omitempty"`
}

// adminContainerListJSON returns the administrator container list: the fields
// and error JSON of tools.DockerListContainers plus the protection flags. The
// SSE container_update feed keeps using tools.DockerListContainers unchanged.
func adminContainerListJSON(ctx context.Context, s *Server, cfg tools.DockerConfig) string {
	entries, failure := tools.DockerListContainerEntries(cfg, true)
	if failure != "" {
		return failure
	}
	// Hostname and /proc are read once per request; the endpoint lookup is
	// shared between lists for containerEndpointCacheTTL.
	self, shared := containerSelfInList(entries, readContainerSelfSignals(containerRuntimeIsDocker(s)))
	if proven := proveSelfInSharedGroup(ctx, cfg, containerRuntimeIsDocker(s), shared); proven != "" {
		// The hostname and /etc signals cannot tell AuraGo from its network
		// provider; a proof that survives network sharing can
		// (containers_self_proof.go).
		self[proven] = true
		delete(shared, proven)
	}
	endpoint, err := listEndpointLookup.addresses(ctx, cfg.Host)
	if err != nil {
		// docker.host did not resolve: a listed container at the address of
		// AuraGo's own Docker connection is the endpoint. Loopback and host
		// addresses name none.
		if observed, ok := observedDockerEndpoint(ctx, cfg); ok && len(containersAtAddresses(entries, observed)) > 0 {
			endpoint, err = observed, nil
		}
	}
	if err != nil {
		// No endpoint container is marked; terminal, update and remove still
		// classify their target and ask for a confirmation.
		endpoint = nil
		if s.Logger != nil && listEndpointLookup.shouldWarn(cfg.Host) {
			s.Logger.Warn("[Containers] Docker endpoint lookup failed; no endpoint container marked", "error", err, "repeat_after", containerEndpointWarnInterval.String())
		}
	}
	var out []adminContainerEntry
	for _, entry := range entries {
		id := strings.ToLower(entry.FullID)
		out = append(out, adminContainerEntry{
			DockerContainerListEntry: entry,
			ProtectedOwner:           firstContainerOwner(entry.Names, entry.Labels),
			Self:                     self[id],
			DockerEndpoint:           containerServesDockerEndpoint(endpoint, entry.NetworkIPs),
			SharedNetwork:            shared[id],
		})
	}
	encoded, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(out), "containers": out})
	return string(encoded)
}
