package rtlsdr

import (
	"aurago/internal/dockerutil"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const RuntimeImage = "ghcr.io/antibyte/aurago-rtl-sdr:1"

type Device struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Serial        string `json:"serial"`
	Node          string `json:"-"`
	Vendor        string `json:"vendor"`
	Product       string `json:"product"`
	Driver        string `json:"driver"`
	Group         int    `json:"-"`
	GroupWritable bool   `json:"group_writable"`
}
type RuntimeConfig struct {
	Enabled, ReadOnly, DockerEnabled, DockerReadOnly bool
	InDocker                                         bool
	Device, DockerHost                               string
}
type RuntimeState struct {
	Platform string `json:"platform"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
	Device   string `json:"device,omitempty"`
}

// Manager exposes only an owner-only Unix socket. It never mounts the Docker
// socket into the receiver and grants access to one enumerated USB node only.
type Manager struct {
	*Worker
	mu                            sync.Mutex
	stateMu                       sync.Mutex
	setupMu                       sync.Mutex
	setupCancel                   context.CancelFunc
	setupWG                       sync.WaitGroup
	closed                        bool
	directory, runDirectory, name string
	config                        func() RuntimeConfig
	state                         RuntimeState
	deviceID, deviceNode          string
}

func NewManager(directory string, cfg func() RuntimeConfig) (*Manager, error) {
	path, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	run := filepath.Join(path, "run")
	if err = os.MkdirAll(run, 0700); err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(path))
	name := "aurago-rtl-sdr-" + hex.EncodeToString(digest[:6])
	return &Manager{Worker: NewWorker(filepath.Join(run, "worker.sock")), directory: path, runDirectory: run, name: name, config: cfg, state: RuntimeState{Platform: runtime.GOOS, Status: "not_ready"}}, nil
}
func (m *Manager) Status() RuntimeState { m.stateMu.Lock(); defer m.stateMu.Unlock(); return m.state }
func (m *Manager) setState(status, code, device string) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.state = RuntimeState{Platform: runtime.GOOS, Status: status, Error: code, Device: device}
}
func (m *Manager) Prepare() error {
	if err := m.allowed(); err != nil {
		return err
	}
	m.setupMu.Lock()
	defer m.setupMu.Unlock()
	if m.closed {
		return ErrUnavailable
	}
	if m.setupCancel != nil {
		return ErrBusy
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	m.setupCancel = cancel
	m.setState("preparing", "", "")
	m.setupWG.Add(1)
	go func() {
		defer m.setupWG.Done()
		defer cancel()
		_ = m.Ensure(ctx)
		m.setupMu.Lock()
		m.setupCancel = nil
		m.setupMu.Unlock()
	}()
	return nil
}
func (m *Manager) Close() {
	m.setupMu.Lock()
	m.closed = true
	if m.setupCancel != nil {
		m.setupCancel()
	}
	m.setupMu.Unlock()
	m.setupWG.Wait()
	m.Worker.client.CloseIdleConnections()
}
func (m *Manager) allowed() error {
	c := m.config()
	if runtime.GOOS != "linux" {
		return errors.New("sdr_linux_required")
	}
	if !c.Enabled {
		return ErrDisabled
	}
	if c.ReadOnly || c.DockerReadOnly {
		return ErrReadOnly
	}
	if !c.DockerEnabled {
		return errors.New("sdr_docker_disabled")
	}
	if os.Getuid() == 0 {
		return errors.New("sdr_service_user")
	}
	if !strings.HasPrefix(dockerutil.NormalizeHost(c.DockerHost), "unix://") {
		return errors.New("sdr_local_docker_required")
	}
	return nil
}
func (m *Manager) Ensure(ctx context.Context) (err error) {
	return m.ensure(ctx, false)
}
func (m *Manager) ensure(ctx context.Context, audioOnly bool) (err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	defer func() {
		if err != nil {
			m.setState("not_ready", runtimeError(err), "")
		}
	}()
	if err = m.allowed(); err != nil {
		return err
	}
	c := m.config()
	devices := Devices()
	var selected *Device
	for i := range devices {
		if devices[i].ID == c.Device || (c.Device == "" && len(devices) == 1) {
			selected = &devices[i]
			break
		}
	}
	if selected == nil && !audioOnly {
		return errors.New("sdr_device_missing")
	}
	if selected != nil && !selected.GroupWritable && !audioOnly {
		return errors.New("sdr_usb_permissions")
	}
	if audioOnly {
		selected = nil
	}
	key, deviceID, serial := RuntimeImage, "", ""
	if selected != nil {
		key = selected.ID + "|" + selected.Node + "|" + strconv.Itoa(selected.Group) + "|" + RuntimeImage
		deviceID = selected.ID
		serial = selected.Serial
		m.stateMu.Lock()
		m.deviceID, m.deviceNode = selected.ID, selected.Node
		m.stateMu.Unlock()
	}
	// A changed bus address invalidates the device cgroup rule after hotplug.
	client := dockerutil.NewClient(c.DockerHost, 30*time.Second)
	defer client.CloseIdleConnections()
	dataPath, runPath := m.directory, m.runDirectory
	if c.InDocker {
		var current struct{ Mounts []runtimeMount }
		containerID := strings.TrimSpace(os.Getenv("HOSTNAME"))
		if containerID == "" {
			return errors.New("sdr_data_mount_required")
		}
		if _, e := client.DoJSON(ctx, "GET", "containers/"+url.PathEscape(containerID)+"/json", nil, &current); e != nil {
			return errors.New("sdr_data_mount_required")
		}
		if dataPath, err = runtimeHostPath(m.directory, current.Mounts); err != nil {
			return err
		}
		if runPath, err = runtimeHostPath(m.runDirectory, current.Mounts); err != nil {
			return err
		}
	}
	var inspect struct {
		ID     string
		Config struct{ Labels map[string]string }
		State  struct{ Running bool }
	}
	status, inspectErr := client.DoJSON(ctx, "GET", "containers/"+m.name+"/json", nil, &inspect)
	if inspectErr != nil && status != 404 {
		return errors.New("sdr_docker_unavailable")
	}
	if inspectErr == nil {
		if inspect.Config.Labels["io.aurago.rtl-sdr.owner"] != m.directory {
			return errors.New("sdr_container_conflict")
		}
		if (!audioOnly && inspect.Config.Labels["io.aurago.rtl-sdr.device"] != key) || !inspect.State.Running {
			if err = m.allowed(); err != nil {
				return err
			}
			if _, err = client.DoJSON(ctx, "DELETE", "containers/"+m.name+"?force=true", nil, nil); err != nil {
				return errors.New("sdr_container_failed")
			}
			inspect.ID = ""
		}
	}
	if inspect.ID == "" {
		m.setState("preparing", "", deviceID)
		status, _ = client.DoJSON(ctx, "GET", "images/"+url.PathEscape(RuntimeImage)+"/json", nil, nil)
		if status != 200 {
			if err = m.pull(ctx, client); err != nil {
				return err
			}
		}
		if err = m.allowed(); err != nil {
			return err
		}
		body := map[string]any{"Image": RuntimeImage, "User": fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "Labels": map[string]string{"aurago.managed": "rtl-sdr", "io.aurago.rtl-sdr.owner": m.directory, "io.aurago.rtl-sdr.device": key}, "Env": []string{"RTL_SDR_SERIAL=" + serial}, "HostConfig": map[string]any{
			"NetworkMode": "none", "ReadonlyRootfs": true, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges:true"},
			"Binds": []string{dockerutil.FormatBindMount(dataPath, "/data", "ro"), dockerutil.FormatBindMount(runPath, "/run/aurago-rtlsdr", "rw")}, "Tmpfs": map[string]string{"/tmp": "rw,nosuid,nodev,size=268435456,mode=1777"},
			"Memory": int64(1536 << 20), "PidsLimit": 256, "RestartPolicy": map[string]string{"Name": "no"}, "LogConfig": map[string]any{"Type": "json-file", "Config": map[string]string{"max-size": "2m", "max-file": "2"}},
		}}
		if selected != nil {
			host := body["HostConfig"].(map[string]any)
			host["GroupAdd"] = []string{strconv.Itoa(selected.Group)}
			host["Devices"] = []any{map[string]string{"PathOnHost": selected.Node, "PathInContainer": selected.Node, "CgroupPermissions": "rw"}}
		}
		if _, err = client.DoJSON(ctx, "POST", "containers/create?name="+m.name, body, nil); err != nil {
			return errors.New("sdr_container_failed")
		}
		if _, err = client.DoJSON(ctx, "POST", "containers/"+m.name+"/start", nil, nil); err != nil {
			return errors.New("sdr_container_failed")
		}
	}
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		if info, e := m.Worker.Info(ctx); e == nil {
			if !audioOnly && (info.Device == "" || info.Tuner == "") {
				return errors.New("sdr_device_unavailable")
			}
			m.setState("ready", "", deviceID)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("sdr_worker_unavailable")
		case <-ticker.C:
		}
	}
}
func runtimeError(err error) string {
	switch err.Error() {
	case "sdr_linux_required", "sdr_device_missing", "sdr_device_unavailable", "sdr_usb_permissions", "sdr_docker_disabled", "sdr_docker_unavailable", "sdr_container_conflict", "sdr_container_failed", "sdr_worker_unavailable", "sdr_image_unavailable", "sdr_service_user", "sdr_local_docker_required", "sdr_data_mount_required":
		return err.Error()
	}
	if errors.Is(err, ErrReadOnly) {
		return ErrReadOnly.Error()
	}
	if errors.Is(err, ErrDisabled) {
		return ErrDisabled.Error()
	}
	return ErrUnavailable.Error()
}
func (m *Manager) pull(ctx context.Context, c *dockerutil.Client) error {
	req, err := http.NewRequestWithContext(ctx, "POST", dockerutil.Endpoint("images/create?fromImage="+url.QueryEscape(RuntimeImage)), nil)
	if err != nil {
		return err
	}
	response, err := c.HTTPClientWithTimeout(15 * time.Minute).Do(req)
	if err != nil {
		return errors.New("sdr_image_unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("sdr_image_unavailable")
	}
	dec := json.NewDecoder(response.Body)
	for dec.More() {
		var event struct {
			Error string `json:"error"`
		}
		if err = dec.Decode(&event); err != nil {
			return errors.New("sdr_image_unavailable")
		}
		if event.Error != "" {
			return errors.New("sdr_image_unavailable")
		}
	}
	return nil
}
func (m *Manager) Tune(ctx context.Context, t Tuning) error {
	if err := m.Ensure(ctx); err != nil {
		return err
	}
	return m.Worker.Tune(ctx, t)
}
func (m *Manager) Scan(ctx context.Context, progress func([]Station, string)) error {
	if err := m.Ensure(ctx); err != nil {
		return err
	}
	return m.Worker.Scan(ctx, progress)
}
func (m *Manager) WAV(ctx context.Context, id string, offset float64, seconds int) ([]byte, error) {
	if _, err := m.Worker.Info(ctx); err == nil {
		return m.Worker.WAV(ctx, id, offset, seconds)
	}
	if err := m.ensure(ctx, true); err != nil {
		return nil, err
	}
	return m.Worker.WAV(ctx, id, offset, seconds)
}

func (m *Manager) AudioSeconds(ctx context.Context, id string) (float64, error) {
	if _, err := m.Worker.Info(ctx); err != nil {
		if err = m.ensure(ctx, true); err != nil {
			return 0, err
		}
	}
	return m.Worker.AudioSeconds(ctx, id)
}

// Recreate only the owned container when the USB bus address changes. Existing
// recordings are finalized as partial; only a still-leased live session resumes.
func (m *Manager) Recover(ctx context.Context, tuning Tuning) error {
	if err := m.Ensure(ctx); err != nil {
		return err
	}
	info, err := m.Worker.Info(ctx)
	if err != nil {
		return err
	}
	if !info.Ready {
		return m.Worker.Tune(ctx, tuning)
	}
	return nil
}

// DeviceConnected also detects a changed bus address. The old container's device
// cgroup rule cannot follow a reinserted receiver even at the same physical port.
func (m *Manager) DeviceConnected() bool {
	m.stateMu.Lock()
	id, node := m.deviceID, m.deviceNode
	m.stateMu.Unlock()
	if node == "" {
		return true // No device has been selected yet; Tune reports setup errors.
	}
	for _, device := range Devices() {
		if device.ID == id && device.Node == node {
			_, err := os.Stat(node)
			return err == nil
		}
	}
	return false
}

type runtimeMount struct {
	Type, Source, Destination string
	RW                        bool
}

// Resolve only a persisted, writable mount of the current AuraGo container.
// Container overlay paths are not paths on the Docker daemon's host.
func runtimeHostPath(directory string, mounts []runtimeMount) (string, error) {
	directory = path.Clean(directory)
	best, resolved := 0, ""
	for _, mount := range mounts {
		if !mount.RW || (mount.Type != "bind" && mount.Type != "volume") || !path.IsAbs(mount.Source) || !path.IsAbs(mount.Destination) {
			continue
		}
		destination := path.Clean(mount.Destination)
		if directory != destination && !strings.HasPrefix(directory, strings.TrimSuffix(destination, "/")+"/") {
			continue
		}
		if len(destination) > best {
			best = len(destination)
			resolved = path.Join(mount.Source, strings.TrimPrefix(directory, destination))
		}
	}
	if resolved == "" {
		return "", errors.New("sdr_data_mount_required")
	}
	return resolved, nil
}
