package acestep

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurago/internal/dockerutil"
)

var imagePattern = regexp.MustCompile(`^ghcr\.io/antibyte/aurago-acestep-(cuda|rocm|xpu|cpu)@sha256:[0-9a-f]{64}$`)

type containerInfo struct {
	ID           string `json:"Id"`
	RestartCount int    `json:"RestartCount"`
	Config       struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Running  bool `json:"Running"`
		ExitCode int  `json:"ExitCode"`
	} `json:"State"`
}

func (m *Manager) inspectOwned(ctx context.Context, name string) (containerInfo, bool, error) {
	var info containerInfo
	code, err := m.docker.DoJSON(ctx, "GET", "containers/"+url.PathEscape(name)+"/json", nil, &info)
	if code == 404 {
		return info, false, nil
	}
	if err != nil {
		return info, false, err
	}
	if !dockerutil.ManagedBy(info.Config.Labels, Owner) {
		return info, false, fmt.Errorf("acestep_container_name_conflict")
	}
	return info, true, nil
}

func (m *Manager) stopOwned(ctx context.Context, name string) error {
	info, exists, err := m.inspectOwned(ctx, name)
	if err != nil {
		return err
	}
	if !exists || !info.State.Running {
		return nil
	}
	_, err = m.docker.DoJSON(ctx, "POST", "containers/"+info.ID+"/stop?t=10", nil, nil)
	return err
}

func (m *Manager) removeOwned(ctx context.Context, name string) error {
	info, exists, err := m.inspectOwned(ctx, name)
	if err != nil || !exists {
		return err
	}
	_, err = m.docker.DoJSON(ctx, "DELETE", "containers/"+info.ID+"?force=true", nil, nil)
	return err
}

func (m *Manager) pull(ctx context.Context, reference string) error {
	if !imagePattern.MatchString(reference) {
		return fmt.Errorf("acestep_release_not_published")
	}
	if code, _ := m.docker.DoJSON(ctx, "GET", "images/"+url.PathEscape(reference)+"/json", nil, nil); code == 200 {
		return nil
	}
	m.setState("downloading", "")
	req, err := http.NewRequestWithContext(ctx, "POST", dockerutil.Endpoint("images/create?fromImage="+url.QueryEscape(reference)), nil)
	if err != nil {
		return err
	}
	resp, err := m.docker.HTTPClientWithTimeout(2 * time.Hour).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("acestep_image_pull_failed")
	}
	dec := json.NewDecoder(resp.Body)
	for {
		var progress struct {
			Error string `json:"error"`
		}
		err = dec.Decode(&progress)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if progress.Error != "" {
			return fmt.Errorf("acestep_image_pull_failed")
		}
	}
	code, err := m.docker.DoJSON(ctx, "GET", "images/"+url.PathEscape(reference)+"/json", nil, nil)
	if err != nil || code != 200 {
		return fmt.Errorf("acestep_image_not_verified")
	}
	return nil
}

func baseHostConfig() map[string]any {
	return map[string]any{"CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges"}, "ReadonlyRootfs": true, "PidsLimit": 1024,
		"Tmpfs": map[string]string{"/tmp": "rw,nosuid,nodev,size=2g"}, "ShmSize": int64(1 << 30),
		"LogConfig": map[string]any{"Type": "json-file", "Config": map[string]string{"max-size": "5m", "max-file": "2"}}}
}

func addGPU(host map[string]any, backend string, device *Device) error {
	if backend == "cpu" {
		return nil
	}
	if backend == "cuda" {
		req := map[string]any{"Driver": "nvidia", "Capabilities": [][]string{{"gpu", "compute", "utility"}}}
		if device == nil {
			req["Count"] = -1
		} else {
			req["DeviceIDs"] = []string{strconv.Itoa(device.Index)}
		}
		host["DeviceRequests"] = []any{req}
		return nil
	}
	paths := []string{"/dev/dri"}
	if device != nil {
		paths = device.RenderNodes
		if len(paths) == 0 {
			return fmt.Errorf("acestep_render_node_unavailable")
		}
		host["GroupAdd"] = dockerutil.ParseNumericGroupIDs(strings.Join(device.Groups, ","))
	}
	if backend == "rocm" {
		paths = append(append([]string{}, paths...), "/dev/kfd")
	}
	var devices []map[string]string
	for _, path := range paths {
		if path != "/dev/dri" && path != "/dev/kfd" && !regexp.MustCompile(`^/dev/dri/renderD[0-9]+$`).MatchString(path) {
			return fmt.Errorf("acestep_invalid_gpu_path")
		}
		devices = append(devices, map[string]string{"PathOnHost": path, "PathInContainer": path, "CgroupPermissions": "rw"})
	}
	host["Devices"] = devices
	return nil
}

// probeContainer runs only the image's fixed diagnostic command, never user commands.
func (m *Manager) probeContainer(ctx context.Context, backend string, groups []string) ([]Device, error) {
	reference := manifest().Images[backend]
	if err := m.pull(ctx, reference); err != nil {
		return nil, err
	}
	name := ContainerName + "-probe"
	if err := m.removeOwned(ctx, name); err != nil {
		return nil, err
	}
	host := baseHostConfig()
	host["NetworkMode"] = "none"
	host["GroupAdd"] = dockerutil.ParseNumericGroupIDs(strings.Join(groups, ","))
	if err := addGPU(host, backend, nil); err != nil {
		return nil, err
	}
	spec := map[string]any{"Image": reference, "User": "65532:65532", "Cmd": []string{"probe"}, "Labels": dockerutil.ManagedLabels(Owner, "music", "probe", ""), "HostConfig": host}
	if _, err := m.docker.DoJSON(ctx, "POST", "containers/create?name="+name, spec, nil); err != nil {
		return nil, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = m.removeOwned(cleanup, name)
	}()
	if _, err := m.docker.DoJSON(ctx, "POST", "containers/"+name+"/start", nil, nil); err != nil {
		return nil, err
	}
	deadline, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	for {
		info, _, err := m.inspectOwned(deadline, name)
		if err != nil {
			return nil, err
		}
		if !info.State.Running {
			if info.State.ExitCode != 0 {
				return nil, fmt.Errorf("acestep_gpu_probe_failed")
			}
			break
		}
		select {
		case <-deadline.Done():
			return nil, deadline.Err()
		case <-time.After(time.Second):
		}
	}
	req, _ := http.NewRequestWithContext(deadline, "GET", dockerutil.Endpoint("containers/"+name+"/logs?stdout=true&stderr=false"), nil)
	resp, err := m.docker.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	// Non-TTY Docker logs contain an eight-byte multiplexing header per frame.
	var output bytes.Buffer
	for len(raw) >= 8 {
		n := int(binary.BigEndian.Uint32(raw[4:8]))
		if n > len(raw)-8 || n < 0 {
			return nil, fmt.Errorf("acestep_invalid_probe_log")
		}
		if raw[0] == 1 {
			output.Write(raw[8 : 8+n])
		}
		raw = raw[8+n:]
	}
	var result struct {
		Devices []Device `json:"devices"`
		Groups  []string `json:"groups"`
	}
	if err = json.Unmarshal(bytes.TrimSpace(output.Bytes()), &result); err != nil {
		return nil, fmt.Errorf("acestep_invalid_probe")
	}
	if len(result.Devices) == 0 && len(groups) == 0 && len(result.Groups) > 0 && (backend == "rocm" || backend == "xpu") {
		return m.probeContainer(ctx, backend, result.Groups)
	}
	return result.Devices, nil
}

func (m *Manager) detect(ctx context.Context, w desired) (Device, error) {
	m.setState("probing", "")
	var info struct {
		OSType        string
		Architecture  string
		KernelVersion string
		Runtimes      map[string]json.RawMessage
	}
	if _, err := m.docker.DoJSON(ctx, "GET", "info", nil, &info); err != nil {
		return Device{}, fmt.Errorf("docker_unavailable")
	}
	if info.OSType != "linux" || info.Architecture != "x86_64" && info.Architecture != "amd64" {
		return Device{}, fmt.Errorf("acestep_requires_linux_amd64_engine")
	}
	backends := []string{w.Local.Backend}
	if w.Local.Backend == "auto" {
		backends = []string{"cuda", "rocm", "xpu"}
	}
	var devices []Device
	for _, backend := range backends {
		if strings.Contains(strings.ToLower(info.KernelVersion), "microsoft") && backend != "cuda" && backend != "cpu" {
			continue
		}
		if backend == "cuda" && info.Runtimes["nvidia"] == nil && !strings.Contains(strings.ToLower(info.KernelVersion), "microsoft") && w.Local.Backend == "auto" {
			continue
		}
		probed, err := m.probeContainer(ctx, backend, nil)
		if err != nil {
			if w.Local.Backend != "auto" {
				return Device{}, err
			}
			continue
		}
		for _, d := range probed {
			if d.Backend == backend && d.Verified && (backend == "cpu" || d.FreeGB-*w.Local.VRAMReserveGB >= 4) {
				devices = append(devices, d)
			}
		}
	}
	m.mu.Lock()
	m.status.Devices = append([]Device{}, devices...)
	m.mu.Unlock()
	return selectDevice(devices, w.Local.Device)
}

func selectDevice(devices []Device, selected string) (Device, error) {
	devices = append([]Device(nil), devices...)
	sort.SliceStable(devices, func(i, j int) bool { return devices[i].FreeGB > devices[j].FreeGB })
	for _, d := range devices {
		if d.Verified && (selected == "auto" || selected == d.ID) {
			return d, nil
		}
	}
	return Device{}, fmt.Errorf("acestep_no_compatible_gpu")
}

func (m *Manager) createRuntime(ctx context.Context, w desired, device Device, conservative bool) error {
	for _, name := range []string{ModelVolume, CacheVolume} {
		var info struct{ Labels map[string]string }
		code, err := m.docker.DoJSON(ctx, "GET", "volumes/"+name, nil, &info)
		if code == 404 {
			_, err = m.docker.DoJSON(ctx, "POST", "volumes/create", map[string]any{"Name": name, "Labels": dockerutil.ManagedLabels(Owner, "music", "data", "")}, nil)
		} else if err == nil && !dockerutil.ManagedBy(info.Labels, Owner) {
			return fmt.Errorf("acestep_volume_name_conflict")
		}
		if err != nil {
			return err
		}
	}
	host := baseHostConfig()
	host["RestartPolicy"] = map[string]string{"Name": "unless-stopped"}
	host["Mounts"] = []map[string]any{{"Type": "volume", "Source": ModelVolume, "Target": "/app/checkpoints"}, {"Type": "volume", "Source": CacheVolume, "Target": "/app/.cache"}}
	m.baseURL = "http://127.0.0.1:" + listenPort
	if w.InDocker {
		host["NetworkMode"] = "aurago-app"
		m.baseURL = "http://" + ContainerName + ":8001"
	} else {
		host["NetworkMode"] = "bridge"
		host["PortBindings"] = map[string]any{"8001/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": listenPort}}}
	}
	if err := addGPU(host, device.Backend, &device); err != nil {
		return err
	}
	env := []string{"ACESTEP_API_KEY=" + m.key, "AURAGO_DEVICE_INDEX=" + strconv.Itoa(device.Index), "AURAGO_VRAM_RESERVE_GB=" + strconv.FormatFloat(*w.Local.VRAMReserveGB, 'f', 3, 64), "AURAGO_CONSERVATIVE=" + strconv.FormatBool(conservative)}
	if device.Backend == "cuda" {
		env[1] = "AURAGO_DEVICE_INDEX=0"
	}
	reference := manifest().Images[device.Backend]
	env = append(env, "AURAGO_IMAGE_PIN="+reference)
	env = append(env, "AURAGO_DEVICE_ID="+device.ID)
	m.mu.Lock()
	force := m.forceQualification
	m.mu.Unlock()
	env = append(env, "AURAGO_REQUALIFY="+strconv.FormatBool(force))
	spec := map[string]any{"Image": reference, "User": "65532:65532", "Env": env, "Labels": dockerutil.ManagedLabels(Owner, "music", "runtime", fingerprint(w)), "HostConfig": host, "ExposedPorts": map[string]any{"8001/tcp": map[string]any{}}}
	if _, err := m.docker.DoJSON(ctx, "POST", "containers/create?name="+ContainerName, spec, nil); err != nil {
		return err
	}
	_, err := m.docker.DoJSON(ctx, "POST", "containers/"+ContainerName+"/start", nil, nil)
	return err
}

func (m *Manager) waitReady(ctx context.Context, expectedImage string) error {
	for {
		var status runtimeStatus
		if err := m.request(ctx, "GET", "/aurago/status", nil, &status); err == nil {
			m.mu.Lock()
			m.status.DownloadedBytes = status.DownloadedBytes
			m.status.TotalBytes = status.TotalBytes
			m.mu.Unlock()
			if status.ErrorCode != "" {
				return fmt.Errorf("%s", status.ErrorCode)
			}
			if status.Ready {
				if status.ImagePin != expectedImage || !imagePattern.MatchString(status.ImagePin) {
					return fmt.Errorf("acestep_image_attestation_mismatch")
				}
				if !status.Profile.Device.Verified || status.Profile.Model == "" || status.Profile.Fingerprint == "" {
					return fmt.Errorf("acestep_invalid_attestation")
				}
				m.mu.Lock()
				m.status.Profile = &status.Profile
				m.status.Image = status.ImagePin
				m.mu.Unlock()
				return nil
			}
			if status.State == "downloading" || status.State == "loading" || status.State == "testing" {
				m.setState(status.State, "")
			}
		}
		info, exists, err := m.inspectOwned(ctx, ContainerName)
		if err != nil {
			return err
		}
		if !exists || !info.State.Running || info.RestartCount >= 3 {
			return fmt.Errorf("acestep_start_failed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (m *Manager) install(ctx context.Context, w desired) error {
	if len(manifest().Images) != 4 {
		return fmt.Errorf("acestep_release_not_published")
	}
	if err := m.runtimeKey(); err != nil {
		return err
	}
	previous := m.Status()
	oldURL := m.baseURL
	if oldURL == "" {
		oldURL = "http://127.0.0.1:" + listenPort
		if w.InDocker {
			oldURL = "http://" + ContainerName + ":8001"
		}
	}
	rollback := ContainerName + "-previous"
	// A remaining previous container marks a replacement interrupted before
	// qualification committed. Restore it before attempting another update.
	backup, hasBackup, err := m.inspectOwned(ctx, rollback)
	if err != nil {
		return err
	}
	if hasBackup {
		if err = m.removeOwned(ctx, ContainerName); err != nil {
			return err
		}
		if _, err = m.docker.DoJSON(ctx, "POST", "containers/"+backup.ID+"/rename?name="+ContainerName, nil, nil); err != nil {
			return err
		}
	}
	info, exists, err := m.inspectOwned(ctx, ContainerName)
	if err != nil {
		return err
	}
	if exists {
		if err = m.stopOwned(ctx, ContainerName); err != nil {
			return err
		}
	}
	// Release our own allocation before comparing available GPU memory. Other
	// local workloads remain untouched throughout profiling and rollback.
	device, err := m.detect(ctx, w)
	if err != nil {
		if exists && ctx.Err() == nil {
			restore, cancel := context.WithTimeout(m.ctx, 10*time.Minute)
			defer cancel()
			if _, startErr := m.docker.DoJSON(restore, "POST", "containers/"+info.ID+"/start", nil, nil); startErr == nil {
				m.baseURL = oldURL
				if readyErr := m.waitReady(restore, info.Config.Image); readyErr == nil {
					m.setState("ready", safeError(err))
				}
			}
		}
		return err
	}
	if exists {
		if _, err = m.docker.DoJSON(ctx, "POST", "containers/"+info.ID+"/rename?name="+rollback, nil, nil); err != nil {
			return err
		}
	}
	m.setState("starting", "")
	for attempt := 0; attempt < 2; attempt++ {
		err = m.createRuntime(ctx, w, device, attempt == 1)
		if err == nil {
			err = m.waitReady(ctx, manifest().Images[device.Backend])
		}
		if err == nil {
			break
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		removeErr := m.removeOwned(cleanup, ContainerName)
		cancel()
		if removeErr != nil {
			return fmt.Errorf("acestep_cleanup_failed")
		}
		if safeError(err) != "acestep_out_of_memory" {
			break
		}
	}
	if err != nil {
		if exists {
			restore, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			if _, renameErr := m.docker.DoJSON(restore, "POST", "containers/"+info.ID+"/rename?name="+ContainerName, nil, nil); renameErr == nil {
				if ctx.Err() != nil {
					return err
				}
				if _, startErr := m.docker.DoJSON(restore, "POST", "containers/"+info.ID+"/start", nil, nil); startErr == nil {
					m.baseURL = oldURL
					m.mu.Lock()
					m.status.Profile = previous.Profile
					m.mu.Unlock()
					if readyErr := m.waitReady(restore, info.Config.Image); readyErr == nil {
						m.setState("ready", safeError(err))
					}
				}
			}
		}
		return err
	}
	if exists {
		if err = m.removeOwned(ctx, rollback); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.status.Image = manifest().Images[device.Backend]
	m.mu.Unlock()
	return nil
}
