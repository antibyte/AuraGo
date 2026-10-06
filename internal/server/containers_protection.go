package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
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
	Unverified     bool   // Docker gave no usable inspect answer
}

func (p containerProtection) protected() bool {
	return p.Owner != "" || p.Self || p.DockerEndpoint || p.Unverified
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
	case p.Owner != "":
		return p.Owner
	case p.Unverified:
		return "unverified"
	default:
		return ""
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
// bind-mounts into every container from <data-root>/containers/<id>/.
var mountinfoContainerIDPattern = regexp.MustCompile(`(?:^|/)containers/([0-9a-f]{64})/(?:hostname|hosts|resolv\.conf)$`)

// cgroupContainerIDPattern matches the cgroup v1 path of a Docker container,
// both the cgroupfs (/docker/<id>) and the systemd (docker-<id>.scope) form.
var cgroupContainerIDPattern = regexp.MustCompile(`(?:/docker/|/docker-)([0-9a-f]{64})(?:\.scope)?$`)

// containerIsSelf reports whether fullID is the container this process runs
// in. It needs the Docker runtime and one of two signals: Docker's default
// hostname as a prefix of the ID, or AuraGo's own container ID read from
// /proc (which survives a compose `hostname:` override). Without either, the
// app container stays protected through its reserved name or owner label.
func containerIsSelf(isDocker bool, fullID string) bool {
	if !isDocker {
		return false
	}
	id := strings.ToLower(strings.TrimSpace(fullID))
	if id == "" {
		return false
	}
	if hostname, err := containerSelfHostname(); err == nil {
		hostname = strings.ToLower(strings.TrimSpace(hostname))
		if dockerDefaultHostnamePattern.MatchString(hostname) && strings.HasPrefix(id, hostname) {
			return true
		}
	}
	own := ownContainerID()
	return own != "" && own == id
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

// containerEndpointLookup resolves a Docker endpoint host name; tests replace it.
var containerEndpointLookup = net.DefaultResolver.LookupHost

// containerDockerEndpointAddresses is dockerEndpointAddresses; handler tests
// replace it because their fake engine listens on loopback.
var containerDockerEndpointAddresses = dockerEndpointAddresses

// dockerEndpointAddresses returns the addresses AuraGo connects to for a TCP
// Docker endpoint, e.g. the docker-proxy service address for
// tcp://docker-proxy:2375. Socket and named-pipe endpoints, localhost, and
// loopback or unspecified addresses return none: no container is identified.
func dockerEndpointAddresses(ctx context.Context, dockerHost string) []string {
	host := strings.TrimSpace(dockerHost)
	if host == "" || strings.HasPrefix(host, "unix://") || strings.HasPrefix(host, "npipe://") {
		return nil
	}
	if !strings.Contains(host, "://") {
		host = "tcp://" + host
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return nil
	}
	name := strings.TrimSuffix(parsed.Hostname(), ".")
	if name == "" || strings.EqualFold(name, "localhost") {
		return nil
	}
	candidates := []string{name}
	if net.ParseIP(name) == nil {
		lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		resolved, err := containerEndpointLookup(lookupCtx, name)
		if err != nil {
			return nil
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
	return out
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

func classifyContainer(isDocker bool, endpoint []string, fullID string, names []string, labels map[string]string, ips []string) containerProtection {
	return containerProtection{
		Owner:          firstContainerOwner(names, labels),
		Self:           containerIsSelf(isDocker, fullID),
		DockerEndpoint: containerServesDockerEndpoint(endpoint, ips),
	}
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
	data, code, err := tools.DockerRequestContext(ctx, cfg, http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/json", "")
	if err == nil && code == http.StatusNotFound {
		// No such container: nothing to protect; the action reports it.
		return containerProtection{}
	}
	var info struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		NetworkSettings struct {
			Networks map[string]struct {
				IPAddress         string `json:"IPAddress"`
				GlobalIPv6Address string `json:"GlobalIPv6Address"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err != nil || code != http.StatusOK || json.Unmarshal(data, &info) != nil {
		// Fail toward a confirmation, like tools.DockerContainerManagedBy: a
		// reserved name still names its owner, anything else is unverified.
		return containerProtection{Owner: firstContainerOwner([]string{containerID}, nil), Unverified: true}
	}
	ips := make([]string, 0, 2*len(info.NetworkSettings.Networks))
	for _, network := range info.NetworkSettings.Networks {
		ips = append(ips, network.IPAddress, network.GlobalIPv6Address)
	}
	return classifyContainer(containerRuntimeIsDocker(s), containerDockerEndpointAddresses(ctx, cfg.Host), info.ID, []string{info.Name, containerID}, info.Config.Labels, ips)
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
		message := "AuraGo manages this container (" + p.label() + "). Repeat the request with confirm=protected to continue."
		if p.label() == "unverified" {
			message = "Docker did not answer the ownership check for this container. Repeat the request with confirm=protected to continue."
		}
		containerJSON(w, http.StatusConflict, map[string]string{"status": "error", "code": containerCodeConfirmationRequired, "owner": p.label(), "message": message})
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
}

// adminContainerListJSON returns the administrator container list: the fields
// and error JSON of tools.DockerListContainers plus the protection flags. The
// SSE container_update feed keeps using tools.DockerListContainers unchanged.
func adminContainerListJSON(ctx context.Context, s *Server, cfg tools.DockerConfig) string {
	entries, failure := tools.DockerListContainerEntries(cfg, true)
	if failure != "" {
		return failure
	}
	isDocker := containerRuntimeIsDocker(s)
	endpoint := containerDockerEndpointAddresses(ctx, cfg.Host)
	var out []adminContainerEntry
	for _, entry := range entries {
		p := classifyContainer(isDocker, endpoint, entry.FullID, entry.Names, entry.Labels, entry.NetworkIPs)
		out = append(out, adminContainerEntry{
			DockerContainerListEntry: entry,
			ProtectedOwner:           p.Owner,
			Self:                     p.Self,
			DockerEndpoint:           p.DockerEndpoint,
		})
	}
	encoded, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(out), "containers": out})
	return string(encoded)
}
