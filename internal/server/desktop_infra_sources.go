package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"aurago/internal/acestep"
	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/infra"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)

const desktopInfraMaxProviderRows = 4096

func desktopInfraRefreshRegistry(ctx context.Context, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, force bool) {
	if s.InventoryDB == nil {
		return
	}
	instance := "inventory"
	key := desktopInfraConfigKey("registry", instance)
	desktopInfraRefreshOne(ctx, runtime, store, "registry", instance, key, desktopInfraTTL, force, func(context.Context) ([]infra.SourceObservation, error) {
		devices, err := inventory.ListAllDevices(s.InventoryDB)
		if err != nil {
			return nil, fmt.Errorf("read registry devices: %w", err)
		}
		if len(devices) > desktopInfraMaxProviderRows {
			return nil, fmt.Errorf("registry observation exceeds the supported limit")
		}
		observations := make([]infra.SourceObservation, 0, len(devices))
		for _, device := range devices {
			if strings.TrimSpace(device.ID) == "" {
				return nil, fmt.Errorf("registry observation contains an invalid device identity")
			}
			observed := desktopInfraRegistryObserved(device)
			observations = append(observations, infra.SourceObservation{
				Identity: infra.SourceIdentity{Kind: "registry", Instance: instance, Key: device.ID},
				Name:     observed.Name, Observed: observed,
			})
		}
		return observations, nil
	})
}

func desktopInfraRegistryObserved(device inventory.DeviceRecord) infra.SourceObserved {
	name := desktopInfraText(device.Name, 200)
	if name == "" {
		name = "Unnamed device"
	}
	deviceType := desktopInfraText(device.Type, 256)
	ip := desktopInfraValidIP(device.IPAddress)
	mac := desktopInfraValidMAC(device.MACAddress)
	tags := desktopInfraStrings(device.Tags, 64, 64)
	interfaces := []infra.NetworkInterface{}
	if ip != "" || mac != "" {
		interfaces = append(interfaces, infra.NetworkInterface{Name: "primary", IPAddress: ip, MACAddress: mac})
	}
	return infra.SourceObserved{
		Kind: infra.NodeDevice, Name: name, Icon: desktopInfraRegistryIcon(deviceType),
		Role: deviceType, Model: "", IPAddress: ip, MACAddress: mac,
		Tags: tags, Interfaces: interfaces,
	}
}

func desktopInfraRefreshDocker(ctx context.Context, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, force bool) {
	instance := desktopInfraDockerInstance(cfg)
	key := desktopInfraConfigKey("docker", cfg.Docker.Host, fmt.Sprint(cfg.Docker.Enabled), fmt.Sprint(cfg.Docker.ReadOnly))
	requestCfg := tools.DockerConfig{Host: cfg.Docker.Host}
	desktopInfraRefreshOne(ctx, runtime, store, "docker", instance, key, desktopInfraTTL, force, func(callCtx context.Context) ([]infra.SourceObservation, error) {
		callCtx, cancel := desktopInfraContext(callCtx)
		defer cancel()
		body, code, err := desktopInfraDockerRequest(runtime, callCtx, requestCfg, http.MethodGet, "/containers/json?all=true", "")
		if err != nil {
			return nil, fmt.Errorf("read Docker containers: %w", err)
		}
		if code != 200 {
			return nil, fmt.Errorf("Docker list returned HTTP %d", code)
		}
		var rows []struct {
			ID              string            `json:"Id"`
			Names           []string          `json:"Names"`
			Image           string            `json:"Image"`
			State           string            `json:"State"`
			Status          string            `json:"Status"`
			Labels          map[string]string `json:"Labels"`
			NetworkSettings struct {
				Networks map[string]struct {
					IPAddress string `json:"IPAddress"`
				} `json:"Networks"`
			} `json:"NetworkSettings"`
		}
		if err := json.Unmarshal(body, &rows); err != nil {
			return nil, fmt.Errorf("decode Docker list: %w", err)
		}
		if len(rows) > desktopInfraMaxProviderRows {
			return nil, fmt.Errorf("Docker observation exceeds the supported limit")
		}
		observations := make([]infra.SourceObservation, 0, len(rows))
		for _, row := range rows {
			id := strings.TrimSpace(row.ID)
			if id == "" || !tools.DockerNameValid(id) {
				return nil, fmt.Errorf("Docker observation contains an invalid container identity")
			}
			names := desktopInfraStrings(row.Names, 8, 256)
			owners := tools.DockerContainerOwnersFromMetadata(names, row.Labels,
				acestep.Owner, dockerutil.AppOwner, dockerutil.HomepageOwner, dockerutil.LocalLLMOwner,
				dockerutil.BoringGarageOwner, dockerutil.CYDCreatorOwner, dockerutil.DecisionsOwner,
				dockerutil.SecurityProxyOwner,
			)
			protected := len(owners) != 0
			name := ""
			if len(names) > 0 {
				name = strings.TrimPrefix(names[0], "/")
			}
			name = desktopInfraText(name, 200)
			if name == "" {
				name = id[:min(12, len(id))]
			}
			ips := make([]string, 0, len(row.NetworkSettings.Networks))
			for _, network := range row.NetworkSettings.Networks {
				if ip := desktopInfraValidIP(network.IPAddress); ip != "" {
					ips = append(ips, ip)
				}
			}
			sort.Strings(ips)
			ip := ""
			if len(ips) > 0 {
				ip = ips[0]
			}
			observed := infra.SourceObserved{
				Kind: infra.NodeContainer, Name: name, Icon: "container",
				Model: desktopInfraText(row.Image, 256), Power: desktopInfraPower("docker", row.State),
				IPAddress: ip, Tags: []string{}, Interfaces: []infra.NetworkInterface{}, Protected: protected,
			}
			observations = append(observations, infra.SourceObservation{
				Identity: infra.SourceIdentity{Kind: "docker", Instance: instance, Key: id},
				Name:     name, Observed: observed,
			})
		}
		return observations, nil
	})
}

func desktopInfraRefreshProxmox(ctx context.Context, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, force bool) {
	instance := desktopInfraProxmoxInstance(cfg)
	key := desktopInfraConfigKey("proxmox", cfg.Proxmox.URL, cfg.Proxmox.TokenID, cfg.Proxmox.Secret, cfg.Proxmox.Node,
		fmt.Sprint(cfg.Proxmox.Insecure), fmt.Sprint(cfg.Proxmox.Enabled), fmt.Sprint(cfg.Proxmox.ReadOnly))
	provider := desktopInfraProxmoxConfig(cfg)
	refreshed := desktopInfraRefreshOne(ctx, runtime, store, "proxmox", instance, key, desktopInfraTTL, force, func(callCtx context.Context) ([]infra.SourceObservation, error) {
		resources, err := tools.ProxmoxInfraResources(callCtx, provider)
		if err != nil {
			return nil, fmt.Errorf("read Proxmox resources: %w", err)
		}
		if len(resources) > desktopInfraMaxProviderRows {
			return nil, fmt.Errorf("Proxmox observation exceeds the supported limit")
		}
		observations := make([]infra.SourceObservation, 0, len(resources))
		for _, resource := range resources {
			observation, err := desktopInfraProxmoxObservation(instance, resource)
			if err != nil {
				return nil, err
			}
			observations = append(observations, observation)
		}
		return observations, nil
	})
	if refreshed {
		if err := desktopInfraReconcileProxmoxEdges(ctx, store, instance); err != nil {
			// A failed relationship update must not turn a successful source read into
			// a failed/removed observation. The next explicit refresh can reconcile it.
			slog.Warn("desktop Infra could not refresh Proxmox placement edges")
		}
	}
}

func desktopInfraProxmoxObservation(instance string, resource tools.InfraProxmoxResource) (infra.SourceObservation, error) {
	kind := strings.TrimSpace(resource.Type)
	vmid := strings.TrimSpace(resource.VMID)
	node := strings.TrimSpace(resource.Node)
	if instance == "" || node == "" || (kind == "node" && vmid != "") || ((kind == "qemu" || kind == "lxc") && vmid == "") || (kind != "node" && kind != "qemu" && kind != "lxc") {
		return infra.SourceObservation{}, fmt.Errorf("Proxmox observation contains an invalid resource identity")
	}
	name := desktopInfraText(resource.Name, 200)
	if name == "" {
		if kind == "node" {
			name = node
		} else {
			name = strings.ToUpper(kind) + " " + vmid
		}
	}
	key := kind + ":" + vmid
	observed := infra.SourceObserved{Kind: infra.NodeVM, Name: name, Icon: "vm", Tags: []string{}, Interfaces: []infra.NetworkInterface{}, Host: node}
	if kind == "node" {
		// A Proxmox cluster node is a placement target only. Keep its public
		// projection to identity and icon; host metadata and power controls are
		// intentionally unavailable through the Infra editor.
		key = "node:" + node
		observed = infra.SourceObserved{Kind: infra.NodeDevice, Name: name, Icon: "hypervisor", Tags: []string{}, Interfaces: []infra.NetworkInterface{}}
	} else {
		cpu := ""
		if resource.MaxCPU > 0 {
			cpu = fmt.Sprintf("%.1f / %.1f vCPU", resource.CPU, resource.MaxCPU)
		}
		model := kind
		if kind == "lxc" {
			observed.Kind, observed.Icon, model = infra.NodeContainer, "container", "lxc"
		}
		observed.Power = desktopInfraPower("proxmox", resource.Status)
		observed.Vendor = "Proxmox VE"
		observed.Model = model
		observed.CPU = cpu
		observed.RAM = desktopInfraBytes(resource.MaxMemory)
		observed.Storage = desktopInfraBytes(resource.MaxDisk)
		observed.Tags = desktopInfraTags(resource.Tags)
	}
	return infra.SourceObservation{
		Identity: infra.SourceIdentity{Kind: "proxmox", Instance: instance, Key: key},
		Name:     name, Observed: observed,
	}, nil
}

func desktopInfraRefreshFritz(ctx context.Context, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, force bool) {
	instance := desktopInfraFritzInstance(cfg)
	password := desktopInfraFritzPassword(s, cfg)
	key := desktopInfraConfigKey("fritzbox", cfg.FritzBox.Host, fmt.Sprint(cfg.FritzBox.Port), fmt.Sprint(cfg.FritzBox.HTTPS), cfg.FritzBox.Username,
		password, fmt.Sprint(cfg.FritzBox.InsecureSkipVerify), fmt.Sprint(cfg.FritzBox.Enabled), fmt.Sprint(cfg.FritzBox.Network.Enabled),
		fmt.Sprint(cfg.FritzBox.Network.ReadOnly), fmt.Sprint(cfg.FritzBox.Network.SubFeatures.Hosts), fmt.Sprint(cfg.FritzBox.Network.SubFeatures.WakeOnLAN))
	desktopInfraRefreshOne(ctx, runtime, store, "fritzbox", instance, key, 45*time.Second, force, func(callCtx context.Context) ([]infra.SourceObservation, error) {
		callCtx, cancel := desktopInfraContext(callCtx)
		defer cancel()
		client, cancel, err := desktopInfraNewFritzClient(callCtx, s, cfg)
		if err != nil {
			return nil, fmt.Errorf("create Fritz!Box client: %w", err)
		}
		defer func() { client.Close(); cancel() }()
		hosts, err := client.InfraHosts()
		if err != nil {
			return nil, fmt.Errorf("read Fritz!Box Hosts: %w", err)
		}
		if len(hosts) > desktopInfraMaxProviderRows {
			return nil, fmt.Errorf("Fritz!Box observation exceeds the supported limit")
		}
		observations := make([]infra.SourceObservation, 0, len(hosts))
		for _, host := range hosts {
			key := strings.TrimSpace(host.Key)
			if key == "" {
				return nil, fmt.Errorf("Fritz!Box observation contains an invalid host identity")
			}
			name := desktopInfraText(host.Name, 200)
			if name == "" {
				name = host.IPAddress
			}
			if name == "" {
				name = "Fritz!Box host"
			}
			interfaces := []infra.NetworkInterface{}
			if ip := desktopInfraValidIP(host.IPAddress); ip != "" {
				interfaces = append(interfaces, infra.NetworkInterface{Name: desktopInfraText(host.Interface, 64), IPAddress: ip})
			}
			observed := infra.SourceObserved{
				Kind: infra.NodeDevice, Name: name, Icon: "generic-device", Vendor: "AVM FRITZ!Box",
				Power: desktopInfraFritzPower(host.Active), IPAddress: desktopInfraValidIP(host.IPAddress),
				Tags: []string{}, Interfaces: interfaces,
			}
			observations = append(observations, infra.SourceObservation{
				Identity: infra.SourceIdentity{Kind: "fritzbox", Instance: instance, Key: key},
				Name:     name, Observed: observed,
			})
		}
		return observations, nil
	})
}

func desktopInfraRefreshOne(ctx context.Context, runtime *desktopInfraRuntime, store *infra.Store, kind, instance, configKey string, ttl time.Duration, force bool,
	fetch func(context.Context) ([]infra.SourceObservation, error)) bool {
	cacheKey := kind + "\x00" + instance
	forceRefresh := force
	for {
		runtime.mu.Lock()
		entry := runtime.cache[cacheKey]
		if entry == nil {
			entry = &desktopInfraProviderCache{}
			runtime.cache[cacheKey] = entry
		}
		if entry.inflight != nil {
			wait := entry.inflight
			runtime.mu.Unlock()
			select {
			case <-wait:
				// Recheck the key and freshness after the in-flight observation. A
				// caller may have arrived with a newer configuration snapshot.
				forceRefresh = false
				continue
			case <-ctx.Done():
				return false
			}
		}
		if !forceRefresh && entry.configKey == configKey && time.Now().Before(entry.retryAt) {
			runtime.mu.Unlock()
			return false
		}
		done := make(chan struct{})
		entry.inflight = done
		generation := entry.generation
		runtime.mu.Unlock()

		callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		observations, err := fetch(callCtx)
		cancel()
		if err == nil {
			err = store.ObserveSources(ctx, kind, instance, observations, true)
		}
		if err != nil {
			markCtx, markCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = store.MarkSourcesStale(markCtx, kind, instance)
			markCancel()
		}

		runtime.mu.Lock()
		entry.configKey = configKey
		entry.err = err
		if entry.generation != generation {
			// A mutation raced this read. Keep the observation, but force the next
			// explicit client request to reconcile it.
			entry.retryAt = time.Time{}
		} else if err != nil {
			entry.retryAt = time.Now().Add(desktopInfraErrorTTL)
		} else {
			entry.retryAt = time.Now().Add(ttl)
		}
		entry.inflight = nil
		close(done)
		runtime.mu.Unlock()
		return err == nil
	}
}

func desktopInfraReconcileProxmoxEdges(ctx context.Context, store *infra.Store, instance string) error {
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("read Infra graph for Proxmox placement: %w", err)
	}
	linked := make(map[infra.SourceIdentity]string)
	for _, node := range snapshot.Nodes {
		for _, source := range node.Sources {
			if source.Kind == "proxmox" && source.Instance == instance && source.State == infra.SourceCurrent {
				linked[source.SourceIdentity] = node.ID
			}
		}
	}
	sources, err := store.ListSources(ctx, "proxmox", instance)
	if err != nil {
		return fmt.Errorf("list Proxmox sources for placement: %w", err)
	}
	hostNodes := make(map[string]string)
	for identity, nodeID := range linked {
		if strings.HasPrefix(identity.Key, "node:") {
			hostNodes[strings.TrimPrefix(identity.Key, "node:")] = nodeID
		}
	}
	for _, source := range sources {
		if strings.HasPrefix(source.Key, "node:") {
			continue
		}
		resourceType := strings.SplitN(source.Key, ":", 2)[0]
		if resourceType != "qemu" && resourceType != "lxc" {
			continue
		}
		edges := []infra.Edge{}
		from := linked[source.SourceIdentity]
		to := hostNodes[source.Observed.Host]
		if source.State == infra.SourceCurrent && from != "" && to != "" && from != to {
			edges = append(edges, infra.Edge{From: from, To: to, Kind: infra.EdgeRunsOn})
		}
		if err := store.ReplaceProviderEdges(ctx, source.SourceIdentity, edges); err != nil {
			return fmt.Errorf("replace Proxmox placement for %s: %w", source.Key, err)
		}
	}
	return nil
}

func desktopInfraDockerRequest(runtime *desktopInfraRuntime, ctx context.Context, cfg tools.DockerConfig, method, endpoint, body string) ([]byte, int, error) {
	if runtime != nil && runtime.dockerRequest != nil {
		return runtime.dockerRequest(ctx, cfg, method, endpoint, body)
	}
	return tools.DockerRequestContext(ctx, cfg, method, endpoint, body)
}

func desktopInfraFritzAllowed(cfg *config.Config) bool {
	return cfg != nil && cfg.FritzBox.Enabled && strings.TrimSpace(cfg.FritzBox.Host) != "" &&
		cfg.FritzBox.Network.Enabled && cfg.FritzBox.Network.SubFeatures.Hosts
}

func desktopInfraFritzPassword(s *Server, cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if cfg.FritzBox.Password != "" {
		return cfg.FritzBox.Password
	}
	if s != nil && s.Vault != nil {
		password, _ := s.Vault.ReadSecret("fritzbox_password")
		return password
	}
	return ""
}

func desktopInfraProxmoxConfig(cfg *config.Config) tools.ProxmoxConfig {
	return tools.ProxmoxConfig{
		URL: cfg.Proxmox.URL, TokenID: cfg.Proxmox.TokenID, Secret: cfg.Proxmox.Secret,
		Node: cfg.Proxmox.Node, Insecure: cfg.Proxmox.Insecure,
		ReadOnly: cfg.Proxmox.ReadOnly, AllowDestructive: cfg.Proxmox.AllowDestructive,
	}
}

func desktopInfraDockerInstance(cfg *config.Config) string {
	return desktopInfraInstance("docker", cfg.Docker.Host)
}

func desktopInfraProxmoxInstance(cfg *config.Config) string {
	parsed, err := url.Parse(strings.TrimSpace(cfg.Proxmox.URL))
	endpoint := cfg.Proxmox.URL
	if err == nil && parsed.Host != "" {
		parsed.User = nil
		parsed.RawQuery = ""
		parsed.Fragment = ""
		endpoint = parsed.Scheme + "://" + parsed.Host + strings.TrimSuffix(parsed.EscapedPath(), "/")
	}
	return desktopInfraInstance("proxmox", endpoint)
}

func desktopInfraFritzInstance(cfg *config.Config) string {
	return desktopInfraInstance("fritzbox", strings.ToLower(strings.TrimSpace(cfg.FritzBox.Host)),
		fmt.Sprint(cfg.FritzBox.Port), fmt.Sprint(cfg.FritzBox.HTTPS))
}

func desktopInfraInstance(kind string, values ...string) string {
	return kind + "_" + desktopInfraConfigKey(values...)
}

func desktopInfraConfigKey(values ...string) string {
	hash := sha256.New()
	for _, value := range values {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)[:16])
}

func desktopInfraRegistryIcon(deviceType string) string {
	switch strings.ToLower(strings.TrimSpace(deviceType)) {
	case "server", "host", "hypervisor":
		return "tower-server"
	case "router", "firewall", "modem", "gateway":
		return "router"
	case "switch":
		return "managed-switch"
	case "access point", "access-point", "ap":
		return "access-point"
	case "nas", "storage":
		return "nas"
	case "workstation", "desktop", "pc":
		return "workstation"
	case "laptop", "notebook":
		return "laptop"
	case "printer":
		return "printer"
	case "camera":
		return "camera"
	default:
		return "generic-device"
	}
}

func desktopInfraText(value string, maxRunes int) string {
	value = strings.ToValidUTF8(value, "�")
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > maxRunes {
		value = string(runes[:maxRunes])
	}
	return value
}

func desktopInfraPower(kind, raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	switch kind {
	case "docker":
		switch raw {
		case "running", "restarting":
			return infra.PowerRunning
		case "created", "exited", "dead", "removing":
			return infra.PowerStopped
		case "paused":
			return infra.PowerPaused
		default:
			return infra.PowerUnknown
		}
	case "proxmox":
		switch raw {
		case "running":
			return infra.PowerRunning
		case "stopped":
			return infra.PowerStopped
		case "paused", "suspended":
			return infra.PowerPaused
		default:
			return infra.PowerUnknown
		}
	default:
		return ""
	}
}

func desktopInfraFritzPower(active bool) string {
	if active {
		return infra.PowerRunning
	}
	return infra.PowerStopped
}

func desktopInfraValidIP(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 64 || net.ParseIP(value) == nil {
		return ""
	}
	return value
}

func desktopInfraValidMAC(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := net.ParseMAC(value)
	if err != nil {
		return ""
	}
	return strings.ToUpper(parsed.String())
}

func desktopInfraStrings(values []string, maxCount, maxRunes int) []string {
	result := make([]string, 0, min(maxCount, len(values)))
	for _, value := range values {
		value = desktopInfraText(value, maxRunes)
		if value == "" {
			continue
		}
		result = append(result, value)
		if len(result) >= maxCount {
			break
		}
	}
	return result
}

func desktopInfraTags(value string) []string {
	parts := strings.Split(value, ",")
	return desktopInfraStrings(parts, 64, 64)
}

func desktopInfraBytes(value int64) string {
	if value <= 0 {
		return ""
	}
	const gib = 1024 * 1024 * 1024
	return fmt.Sprintf("%.1f GiB", float64(value)/gib)
}
