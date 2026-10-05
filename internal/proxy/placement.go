package proxy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// placement describes where the Caddy container finds its files and how it
// reaches the AuraGo backend.
type placement struct {
	binds    []string                 // native install: host paths
	mounts   []map[string]interface{} // AuraGo in Docker: AuraGo's own data mount
	network  string                   // AuraGo in Docker: user-defined network shared with AuraGo
	upstream string                   // host:port of the AuraGo backend
}

// subpathAPIVersion is the first Engine API with VolumeOptions.Subpath
// (Docker Engine 26).
const subpathAPIVersion = "1.45"

// upstreamPort is the port AuraGo listens on.
func upstreamPort(cfg *config.Config) int {
	if cfg != nil && cfg.Server.Port > 0 {
		return cfg.Server.Port
	}
	return 8088
}

// nativePlacement bind-mounts the proxy directory of a native install and
// proxies to AuraGo through the Docker host gateway.
func nativePlacement(cfg *config.Config, proxyDir string) placement {
	absCaddyfile, _ := filepath.Abs(filepath.Join(proxyDir, "Caddyfile"))
	absCaddyData, _ := filepath.Abs(filepath.Join(proxyDir, "caddy_data"))
	absCaddyConfig, _ := filepath.Abs(filepath.Join(proxyDir, "caddy_config"))
	// host.docker.internal on Mac/Windows, the default bridge gateway on Linux
	host := "host.docker.internal"
	if runtime.GOOS == "linux" {
		host = "172.17.0.1"
	}
	return placement{
		binds: []string{
			dockerutil.FormatBindMount(absCaddyfile, "/etc/caddy/Caddyfile"),
			dockerutil.FormatBindMount(absCaddyData, "/data"),
			dockerutil.FormatBindMount(absCaddyConfig, "/config"),
		},
		upstream: fmt.Sprintf("%s:%d", host, upstreamPort(cfg)),
	}
}

// resolvePlacement picks the placement for this process. When AuraGo runs in a
// container, its paths only exist inside that container: the proxy then
// mounts the same Docker volume or host directory and joins AuraGo's network.
func (m *Manager) resolvePlacement(cfg *config.Config, proxyDir string) (placement, error) {
	if !m.runsInDocker() {
		return nativePlacement(cfg, proxyDir), nil
	}
	dockerCfg := dockerConfigFor(cfg)
	self, found, err := m.inspectSelf(dockerCfg)
	if err != nil {
		return placement{}, err
	}
	if !found {
		// /.dockerenv without a container on this engine, e.g. a Proxmox LXC
		// guest: AuraGo's paths are host paths for the engine.
		m.log().Warn("Security proxy: this Docker engine does not know the AuraGo container; mounting the data directory as host paths")
		return nativePlacement(cfg, proxyDir), nil
	}
	apiVersion, err := m.engineAPIVersion(dockerCfg)
	if err != nil {
		return placement{}, err
	}
	internal, err := m.networkInternalFlags(dockerCfg, self)
	if err != nil {
		return placement{}, err
	}
	absProxyDir, err := filepath.Abs(proxyDir)
	if err != nil {
		return placement{}, fmt.Errorf("resolve proxy directory: %w", err)
	}
	return dockerPlacement(self, path.Clean(filepath.ToSlash(absProxyDir)), apiVersion, internal, upstreamPort(cfg))
}

// selfContainer is the part of `docker inspect` of the AuraGo container the
// placement needs.
type selfContainer struct {
	Name       string `json:"Name"`
	HostConfig struct {
		NetworkMode  string `json:"NetworkMode"`
		PortBindings map[string][]struct {
			HostPort string `json:"HostPort"`
		} `json:"PortBindings"`
	} `json:"HostConfig"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
	NetworkSettings struct {
		Networks map[string]struct {
			NetworkID string `json:"NetworkID"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

func parseSelfContainer(data []byte) (selfContainer, error) {
	var self selfContainer
	if err := json.Unmarshal(data, &self); err != nil {
		return selfContainer{}, fmt.Errorf("decode AuraGo container inspection: %w", err)
	}
	self.Name = strings.TrimPrefix(strings.TrimSpace(self.Name), "/")
	return self, nil
}

// containerIDPattern finds the container ID in the mountinfo entries Docker
// creates for /etc/hostname, /etc/hosts and /etc/resolv.conf.
var containerIDPattern = regexp.MustCompile(`/containers/([0-9a-f]{64})/`)

// selfContainerIDs lists identifiers of the current container: the full ID
// from /proc/self/mountinfo, which is exact and survives a custom hostname,
// then the hostname (Docker's default is the short ID).
func selfContainerIDs() []string {
	var ids []string
	if data, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		if match := containerIDPattern.FindSubmatch(data); match != nil {
			ids = append(ids, string(match[1]))
		}
	}
	if hostname, err := os.Hostname(); err == nil && strings.TrimSpace(hostname) != "" {
		ids = append(ids, strings.TrimSpace(hostname))
	}
	return ids
}

// inspectSelf inspects the AuraGo container. found is false when the engine
// knows none of the identifiers.
func (m *Manager) inspectSelf(dockerCfg tools.DockerConfig) (selfContainer, bool, error) {
	ids := selfContainerIDs()
	if m.selfIDs != nil {
		ids = m.selfIDs()
	}
	for _, id := range ids {
		data, code, err := m.engine.request(dockerCfg, "GET", "/containers/"+url.PathEscape(id)+"/json", "")
		if err != nil {
			return selfContainer{}, false, fmt.Errorf("inspect AuraGo container %q: %w", id, err)
		}
		if code == 404 {
			continue
		}
		if code != 200 {
			return selfContainer{}, false, fmt.Errorf("%w: inspect AuraGo container %q returned HTTP %d", ErrDockerPlacement, id, code)
		}
		self, err := parseSelfContainer(data)
		return self, err == nil, err
	}
	return selfContainer{}, false, nil
}

func (m *Manager) engineAPIVersion(dockerCfg tools.DockerConfig) (string, error) {
	data, code, err := m.engine.request(dockerCfg, "GET", "/version", "")
	if err != nil {
		return "", fmt.Errorf("read Docker Engine version: %w", err)
	}
	if code != 200 {
		return "", fmt.Errorf("%w: read Docker Engine version returned HTTP %d", ErrDockerPlacement, code)
	}
	var version struct {
		APIVersion string `json:"ApiVersion"`
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return "", fmt.Errorf("decode Docker Engine version: %w", err)
	}
	return strings.TrimPrefix(strings.TrimSpace(version.APIVersion), "v"), nil
}

// networkInternalFlags reports for each candidate network of the AuraGo
// container whether it is internal (no published ports, no egress).
func (m *Manager) networkInternalFlags(dockerCfg tools.DockerConfig, self selfContainer) (map[string]bool, error) {
	flags := make(map[string]bool, len(self.NetworkSettings.Networks))
	for name, endpoint := range self.NetworkSettings.Networks {
		if !isSharedNetworkCandidate(name) {
			continue
		}
		id := endpoint.NetworkID
		if id == "" {
			id = name
		}
		data, code, err := m.engine.request(dockerCfg, "GET", "/networks/"+url.PathEscape(id), "")
		if err != nil {
			return nil, fmt.Errorf("inspect Docker network %q: %w", name, err)
		}
		if code != 200 {
			return nil, fmt.Errorf("%w: inspect Docker network %q returned HTTP %d", ErrDockerPlacement, name, code)
		}
		var network struct {
			Internal bool `json:"Internal"`
		}
		if err := json.Unmarshal(data, &network); err != nil {
			return nil, fmt.Errorf("decode Docker network %q: %w", name, err)
		}
		flags[name] = network.Internal
	}
	return flags, nil
}

// isSharedNetworkCandidate excludes the engine's built-in networks (no name
// resolution) and AuraGo's private Docker control network.
func isSharedNetworkCandidate(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch lower {
	case "", "bridge", "host", "none", "default":
		return false
	}
	return !strings.Contains(lower, "docker-control") && !strings.Contains(lower, "docker_control")
}

// dockerPlacement maps the proxy directory onto the AuraGo container's mounts
// and picks the network Caddy shares with AuraGo. Caddy needs a network that
// publishes ports and reaches the internet (ACME), so internal networks do not
// qualify. Without one, Caddy uses the host gateway and AuraGo's published
// port, like a native install.
func dockerPlacement(self selfContainer, proxyDir, apiVersion string, internal map[string]bool, port int) (placement, error) {
	targets := []struct {
		dir, target string
		readOnly    bool
	}{
		{proxyDir, "/etc/caddy", true},
		{path.Join(proxyDir, "caddy_data"), "/data", false},
		{path.Join(proxyDir, "caddy_config"), "/config", false},
	}
	mounts := make([]map[string]interface{}, 0, len(targets))
	for _, target := range targets {
		mount, err := translateMount(self, target.dir, target.target, apiVersion)
		if err != nil {
			return placement{}, err
		}
		if target.readOnly {
			mount["ReadOnly"] = true
		}
		mounts = append(mounts, mount)
	}

	result := placement{mounts: mounts}
	if network := chooseSharedNetwork(self, internal); network != "" && self.Name != "" {
		result.network = network
		result.upstream = fmt.Sprintf("%s:%d", self.Name, port)
		return result, nil
	}
	hostPort := strconv.Itoa(port)
	for _, binding := range self.HostConfig.PortBindings[hostPort+"/tcp"] {
		if strings.TrimSpace(binding.HostPort) != "" {
			hostPort = strings.TrimSpace(binding.HostPort)
			break
		}
	}
	result.upstream = "host.docker.internal:" + hostPort
	return result, nil
}

func chooseSharedNetwork(self selfContainer, internal map[string]bool) string {
	var candidates []string
	for name := range self.NetworkSettings.Networks {
		if isInternal, known := internal[name]; !isSharedNetworkCandidate(name) || !known || isInternal {
			continue
		}
		candidates = append(candidates, name)
	}
	// Prefer the compose project network ("<project>_default").
	sort.Slice(candidates, func(i, j int) bool {
		iDefault, jDefault := strings.HasSuffix(candidates[i], "_default"), strings.HasSuffix(candidates[j], "_default")
		if iDefault != jDefault {
			return iDefault
		}
		return candidates[i] < candidates[j]
	})
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

// translateMount maps dir inside the AuraGo container to the mount backing it.
func translateMount(self selfContainer, dir, target, apiVersion string) (map[string]interface{}, error) {
	best := -1
	bestRelative := ""
	for i, mount := range self.Mounts {
		relative, ok := containerPathWithin(mount.Destination, dir)
		if !ok {
			continue
		}
		if best < 0 || len(path.Clean(mount.Destination)) > len(path.Clean(self.Mounts[best].Destination)) {
			best, bestRelative = i, relative
		}
	}
	if best < 0 {
		return nil, fmt.Errorf("%w: %s is not on a Docker volume or bind mount of the AuraGo container", ErrDockerPlacement, dir)
	}
	mount := self.Mounts[best]
	switch strings.ToLower(mount.Type) {
	case "volume":
		if strings.TrimSpace(mount.Name) == "" {
			return nil, fmt.Errorf("%w: the volume behind %s has no name", ErrDockerPlacement, dir)
		}
		result := map[string]interface{}{"Type": "volume", "Source": mount.Name, "Target": target}
		if bestRelative != "." {
			if compareAPIVersions(apiVersion, subpathAPIVersion) < 0 {
				return nil, fmt.Errorf("%w: Docker Engine API %s cannot mount a volume subdirectory; Docker Engine 26 or newer is required", ErrDockerPlacement, apiVersion)
			}
			result["VolumeOptions"] = map[string]interface{}{"Subpath": bestRelative}
		}
		return result, nil
	case "bind":
		if strings.TrimSpace(mount.Source) == "" {
			return nil, fmt.Errorf("%w: the bind mount behind %s has no source", ErrDockerPlacement, dir)
		}
		return map[string]interface{}{"Type": "bind", "Source": path.Join(mount.Source, bestRelative), "Target": target}, nil
	}
	return nil, fmt.Errorf("%w: %s is on a %q mount, which another container cannot share", ErrDockerPlacement, dir, mount.Type)
}

// containerPathWithin reports p relative to root when p is root or below it.
func containerPathWithin(root, p string) (string, bool) {
	root = path.Clean(root)
	p = path.Clean(p)
	if p == root {
		return ".", true
	}
	prefix := root
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if strings.HasPrefix(p, prefix) {
		return p[len(prefix):], true
	}
	return "", false
}

// compareAPIVersions compares Docker Engine API versions such as "1.45".
// An unparseable version sorts first.
func compareAPIVersions(left, right string) int {
	parse := func(version string) (int, int, bool) {
		major, minor, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(version), "v"), ".")
		if !ok {
			return 0, 0, false
		}
		maj, errMajor := strconv.Atoi(major)
		mnr, errMinor := strconv.Atoi(minor)
		return maj, mnr, errMajor == nil && errMinor == nil
	}
	lMajor, lMinor, lOK := parse(left)
	rMajor, rMinor, rOK := parse(right)
	switch {
	case !lOK && !rOK:
		return 0
	case !lOK:
		return -1
	case !rOK:
		return 1
	case lMajor != rMajor:
		if lMajor < rMajor {
			return -1
		}
		return 1
	case lMinor != rMinor:
		if lMinor < rMinor {
			return -1
		}
		return 1
	}
	return 0
}
