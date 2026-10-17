package ruview

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

const containerName = "aurago-ruview"

type nativeProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error // Published by closing done; readers must wait for done first.
	kill func() error
}

func configCheck(c *config.Config) error {
	return config.ValidateRuViewConfig(c.RuView, c.Runtime.IsDocker)
}

func (m *Manager) Start(ctx context.Context) error {
	return m.start(ctx, nil)
}

// Startup carries its original generation so a preceding stop/config change wins.
func (m *Manager) start(ctx context.Context, expected context.Context) (startErr error) {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.mu.Lock()
	m.reapNativeLocked()
	generation := m.generation
	m.mu.Unlock()
	if ctx.Err() != nil || generation.Err() != nil || expected != nil && expected != generation {
		return context.Canceled
	}
	ctx, cancel := m.bound(ctx)
	defer cancel()
	c := m.config()
	if !c.RuView.Enabled || c.RuView.ReadOnly {
		return fmt.Errorf("RuView start requires enabled, writable integration")
	}
	if c.RuView.Mode == "external" {
		return fmt.Errorf("external RuView is managed by its operator")
	}
	m.mu.Lock()
	active := m.managed
	m.mu.Unlock()
	if active {
		if expected != nil {
			return nil
		}
		return fmt.Errorf("RuView is already started; stop it before reconfiguration")
	}
	m.mu.Lock()
	m.status = Status{DataState: "starting"}
	m.observed = time.Time{}
	m.mu.Unlock()
	defer func() {
		if startErr == nil {
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if generation == m.generation && generation.Err() == nil {
			m.status = Status{DataState: "disconnected", Error: security.Scrub(startErr.Error())}
		}
	}()
	if err := configCheck(c); err != nil {
		return err
	}
	dir, err := filepath.Abs(filepath.Join(c.Directories.DataDir, "ruview", "data"))
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if i, err := os.Lstat(dir); err != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("invalid RuView data directory")
	}
	token, err := m.managedToken()
	if err != nil {
		return err
	}
	if c.RuView.Mode == "docker" {
		err = m.startDocker(ctx, c, dir, token)
	} else {
		if err := ctx.Err(); err != nil {
			return err
		}
		err = m.startNative(c, dir, token)
	}
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.managed = true
	m.runningConfig = c
	current := generation == m.generation && generation.Err() == nil && ctx.Err() == nil
	m.mu.Unlock()
	if !current {
		cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := m.stopRuntime(cleanup); err != nil {
			return fmt.Errorf("RuView start cancelled; cleanup failed: %w", err)
		}
		return context.Canceled
	}
	m.noteStop(true, c.RuView.Mode)
	return nil
}

func (m *Manager) managedToken() (string, error) {
	if m.vault == nil {
		return "", fmt.Errorf("RuView requires the secrets Vault")
	}
	if token, err := m.vault.ReadSecret(ManagedTokenKey); err == nil && len(token) >= 32 {
		security.RegisterSensitive(token)
		return token, nil
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b[:])
	security.RegisterSensitive(token)
	if err := m.vault.WriteSecret(ManagedTokenKey, token); err != nil {
		return "", err
	}
	return token, nil
}

func runtimeArgs(c *config.Config, dir, bind string, port int) []string {
	udpPort := c.RuView.UDPPort
	if c.RuView.Mode == "docker" {
		udpPort = 5005
	}
	args := []string{"--source", c.RuView.Source, "--http-port", strconv.Itoa(port), "--ws-port", strconv.Itoa(port + 1), "--udp-port", strconv.Itoa(udpPort), "--bind-addr", bind, "--data-dir", dir, "--no-edge-registry"}
	return args
}

func runtimeEnv(c *config.Config, token string, bind string) []string {
	env := []string{"RUVIEW_API_TOKEN=" + token, "RUVIEW_BIND_ADDR=" + bind, "SENSING_ALLOWED_HOSTS=localhost,127.0.0.1,aurago-ruview", "RUST_LOG=off"}
	if c.RuView.UDPEnabled {
		if c.RuView.Mode == "native" {
			bind = c.RuView.UDPBind
		}
		env = append(env, "RUVIEW_UDP_ALLOW="+strings.Join(c.RuView.UDPAllow, ","), "RUVIEW_UDP_BIND="+bind)
	} else {
		env = append(env, "RUVIEW_UDP_BIND=127.0.0.1")
	}
	return env
}

func (m *Manager) startNative(c *config.Config, dir, token string) error {
	a, err := nativeArtifact()
	if err != nil {
		return err
	}
	root, err := filepath.Abs(filepath.Join(c.Directories.DataDir, "ruview", "runtime", Revision, a.Platform))
	if err != nil {
		return err
	}
	for _, file := range a.Files {
		if err := verifyFile(root, file); err != nil {
			return fmt.Errorf("RuView native package is not installed or failed verification: %w", err)
		}
	}
	name := "sensing-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	cmd := exec.Command(filepath.Join(root, name), runtimeArgs(c, dir, "127.0.0.1", c.RuView.HTTPPort)...)
	cmd.Dir = dir
	// Keep provider/Vault environment out of the sensing process. mac_wifi is
	// found only in this verified package; OS utilities retain their system path.
	path := root + string(os.PathListSeparator) + "/usr/bin:/bin:/usr/sbin:/sbin"
	if runtime.GOOS == "windows" {
		path = root + ";" + filepath.Join(os.Getenv("SystemRoot"), "System32")
	}
	cmd.Env = append([]string{"PATH=" + path, "SystemRoot=" + os.Getenv("SystemRoot"), "HOME=" + dir, "TMPDIR=" + os.TempDir(), "TEMP=" + os.TempDir()}, runtimeEnv(c, token, "127.0.0.1")...)
	p, err := launchNative(cmd)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.process = p
	m.mu.Unlock()
	return nil
}

func owner(c *config.Config) string {
	root, _ := filepath.Abs(c.Directories.DataDir)
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:16])
}

func ownedContainer(ctx context.Context, d *dockerutil.Client, c *config.Config) (string, error) {
	var info struct {
		ID     string `json:"Id"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	code, err := d.DoJSON(ctx, "GET", "containers/"+containerName+"/json", nil, &info)
	if code == http.StatusNotFound {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect RuView container: %w", err)
	}
	if info.ID == "" || info.Config.Labels["aurago.managed"] != "ruview" || info.Config.Labels["aurago.ruview.owner"] != owner(c) {
		return "", fmt.Errorf("RuView container name belongs to another workload")
	}
	return info.ID, nil
}

func (m *Manager) startDocker(ctx context.Context, c *config.Config, dir, token string) error {
	if !c.Docker.Enabled || c.Docker.ReadOnly {
		return fmt.Errorf("managed RuView requires writable Docker integration")
	}
	image := Artifacts().DockerImage
	if !strings.Contains(image, "@sha256:") {
		return fmt.Errorf("no verified RuView Docker digest in this release")
	}
	d := dockerutil.NewClient(c.Docker.Host, 0)
	defer d.CloseIdleConnections()
	id, err := ownedContainer(ctx, d, c)
	if err != nil {
		return err
	}
	if id != "" {
		if code, stopErr := d.DoJSON(ctx, "POST", "containers/"+id+"/stop?t=10", nil, nil); stopErr != nil && code != 304 {
			return stopErr
		}
		if _, err = d.DoJSON(ctx, "DELETE", "containers/"+id, nil, nil); err != nil {
			return err
		}
	}
	if code, _ := d.DoJSON(ctx, "GET", "images/"+url.PathEscape(image)+"/json", nil, nil); code != 200 {
		req, err := http.NewRequestWithContext(ctx, "POST", dockerutil.Endpoint("images/create?fromImage="+url.QueryEscape(image)), nil)
		if err != nil {
			return err
		}
		resp, err := d.HTTPClientWithTimeout(10 * time.Minute).Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			detail := dockerutil.SanitizeOneLine(security.Scrub(dockerutil.EngineErrorMessage(dockerutil.ReadErrorBody(resp.Body))), 512)
			return fmt.Errorf("RuView image pull returned HTTP %d: %s", resp.StatusCode, detail)
		}
		if err = dockerutil.DrainJSONMessages(resp.Body); err != nil {
			return fmt.Errorf("pull RuView image: %w", err)
		}
	}
	host := map[string]any{"ReadonlyRootfs": true, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges:true"}, "Memory": int64(2 << 30), "NanoCpus": int64(2e9), "PidsLimit": 128, "Init": true, "Tmpfs": map[string]string{"/tmp": "rw,noexec,nosuid,size=128m"}, "RestartPolicy": map[string]string{"Name": "no"}, "LogConfig": map[string]any{"Type": "none"}}
	ports := map[string][]map[string]string{}
	if c.Runtime.IsDocker {
		network, mount, err := placement(ctx, d, dir)
		if err != nil {
			return err
		}
		host["NetworkMode"] = network
		host["Mounts"] = []any{mount}
	} else {
		host["Binds"] = []string{dockerutil.FormatBindMount(dir, "/data")}
		ports["3000/tcp"] = []map[string]string{{"HostIp": "127.0.0.1", "HostPort": strconv.Itoa(c.RuView.HTTPPort)}}
	}
	if c.RuView.UDPEnabled {
		ports["5005/udp"] = []map[string]string{{"HostIp": c.RuView.UDPBind, "HostPort": strconv.Itoa(c.RuView.UDPPort)}}
	}
	host["PortBindings"] = ports
	payload := map[string]any{"Image": image, "Entrypoint": []string{"/app/sensing-server"}, "Cmd": runtimeArgs(c, "/data", "0.0.0.0", 3000), "Env": runtimeEnv(c, token, "0.0.0.0"), "Labels": map[string]string{"aurago.managed": "ruview", "aurago.ruview.owner": owner(c), "org.opencontainers.image.revision": Revision}, "ExposedPorts": map[string]any{"3000/tcp": struct{}{}, "5005/udp": struct{}{}}, "HostConfig": host}
	var created struct {
		ID string `json:"Id"`
	}
	if _, err = d.DoJSON(ctx, "POST", "containers/create?name="+containerName, payload, &created); err != nil {
		return err
	}
	if created.ID == "" {
		return fmt.Errorf("RuView container creation returned no identity")
	}
	_, err = d.DoJSON(ctx, "POST", "containers/"+created.ID+"/start", nil, nil)
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = d.DoJSON(cleanup, "DELETE", "containers/"+created.ID, nil, nil)
	}
	return err
}

func placement(ctx context.Context, d *dockerutil.Client, dir string) (string, map[string]any, error) {
	self := dockerutil.OwnContainerID(os.ReadFile)
	if self == "" {
		hostname, _ := os.Hostname()
		self = dockerutil.DefaultContainerHostname(hostname)
	}
	if self == "" {
		return "", nil, fmt.Errorf("cannot identify the AuraGo container")
	}
	var i struct {
		NetworkSettings struct{ Networks map[string]any }
		Mounts          []struct{ Type, Source, Destination, Name string }
	}
	if _, err := d.DoJSON(ctx, "GET", "containers/"+url.PathEscape(self)+"/json", nil, &i); err != nil {
		return "", nil, err
	}
	var networks []string
	for n := range i.NetworkSettings.Networks {
		if !strings.Contains(n, "docker-control") && !strings.Contains(n, "docker_control") && n != "bridge" && n != "host" {
			networks = append(networks, n)
		}
	}
	sort.Strings(networks)
	if len(networks) == 0 {
		return "", nil, fmt.Errorf("RuView needs an AuraGo application network")
	}
	for _, mount := range i.Mounts {
		rel, e := filepath.Rel(mount.Destination, dir)
		if e != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if mount.Type == "bind" {
			return networks[0], map[string]any{"Type": "bind", "Source": filepath.Join(mount.Source, rel), "Target": "/data"}, nil
		}
		if mount.Type == "volume" && mount.Name != "" {
			return networks[0], map[string]any{"Type": "volume", "Source": mount.Name, "Target": "/data", "VolumeOptions": map[string]any{"Subpath": filepath.ToSlash(rel)}}, nil
		}
	}
	return "", nil, fmt.Errorf("RuView data directory is outside AuraGo's Docker data mount")
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	m.cancel()
	m.mu.Unlock()
	m.opMu.Lock()
	defer m.opMu.Unlock()
	return m.stopRuntime(ctx)
}

// stopRuntime requires opMu; the caller's context is independent of generation.
func (m *Manager) stopRuntime(ctx context.Context) (stopErr error) {
	m.mu.Lock()
	m.cancel()
	m.status = Status{DataState: "stopping"}
	m.observed = time.Time{}
	c := m.runningConfig
	p := m.process
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.cancel()
		m.generation, m.cancel = context.WithCancel(context.Background())
		m.observed = time.Time{}
		mode := ""
		if c != nil {
			mode = c.RuView.Mode
		}
		confirmed := stopErr == nil
		if stopErr != nil {
			m.status = Status{DataState: "disconnected", Error: "stop_failed"}
		} else {
			m.process = nil
			m.managed = false
			m.runningConfig = nil
			m.status = Status{DataState: "stopped"}
		}
		m.mu.Unlock()
		m.noteStop(confirmed, mode)
	}()
	if c == nil {
		c = m.config()
		if p == nil && (!c.RuView.Enabled || c.RuView.Mode != "docker" || !c.Docker.Enabled) {
			return nil
		}
	}
	if p != nil {
		select {
		case <-p.done:
		default:
			if err := p.kill(); err != nil {
				select {
				case <-p.done:
				default:
					return fmt.Errorf("stop RuView process: %w", err)
				}
			}
			select {
			case <-p.done:
			case <-ctx.Done():
				return fmt.Errorf("wait for RuView process: %w", ctx.Err())
			}
		}
	}
	if c.RuView.Mode == "docker" {
		d := dockerutil.NewClient(c.Docker.Host, 0)
		defer d.CloseIdleConnections()
		id, err := ownedContainer(ctx, d, c)
		if err != nil {
			return err
		}
		if id != "" {
			if code, err := d.DoJSON(ctx, "POST", "containers/"+id+"/stop?t=10", nil, nil); err != nil && code != 304 && code != 404 {
				return err
			}
		}
	}
	return nil
}
