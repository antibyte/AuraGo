package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
)

func Go2RTCBaseImageFor(cfg config.Go2RTCConfig) string {
	if cfg.Hardware.Backend != "" && cfg.Hardware.Backend != "software" && (cfg.Image == "" || cfg.Image == config.Go2RTCDefaultImage || cfg.Image == config.Go2RTCHardwareImage) {
		return config.Go2RTCHardwareImage
	}
	if cfg.Image == "" {
		return config.Go2RTCDefaultImage
	}
	return cfg.Image
}

func validateGo2RTCGPUHost(docker DockerConfig, cfg config.Go2RTCConfig) error {
	if cfg.Hardware.Backend == "" || cfg.Hardware.Backend == "software" {
		return nil
	}
	data, code, err := dockerRequest(docker, http.MethodGet, "/info", "")
	if err != nil || code != http.StatusOK {
		return fmt.Errorf("GPU transcoding requires readable Docker host information")
	}
	var info struct {
		OSType       string
		Architecture string
	}
	if json.Unmarshal(data, &info) != nil || info.OSType != "linux" || (info.Architecture != "x86_64" && info.Architecture != "amd64") {
		return fmt.Errorf("GPU transcoding requires a Linux amd64 Docker host")
	}
	return nil
}

func addGo2RTCGPUDevices(host, payload map[string]interface{}, cfg config.Go2RTCConfig) {
	if cfg.Hardware.Backend != "cuda" {
		env, _ := payload["Env"].([]string)
		payload["Env"] = append(env, "NVIDIA_VISIBLE_DEVICES=void")
	}
	switch cfg.Hardware.Backend {
	case "vaapi":
		host["Devices"] = []map[string]string{{"PathOnHost": cfg.Hardware.RenderNode, "PathInContainer": "/dev/dri/renderD128", "CgroupPermissions": "rw"}}
		host["GroupAdd"] = []string{cfg.Hardware.RenderGroup}
	case "cuda":
		host["DeviceRequests"] = []map[string]interface{}{{"Driver": "nvidia", "DeviceIDs": []string{cfg.Hardware.DeviceID}, "Capabilities": [][]string{{"gpu", "video", "compute", "utility"}}}}
		env, _ := payload["Env"].([]string)
		payload["Env"] = append(env, "NVIDIA_VISIBLE_DEVICES="+cfg.Hardware.DeviceID, "NVIDIA_DRIVER_CAPABILITIES=video,compute,utility")
	}
}

type Go2RTCHardwareResult struct {
	Backend    string `json:"backend"`
	Verified   bool   `json:"verified"`
	VerifiedAt string `json:"verified_at,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
}

// CheckHardware is an explicit administrator action; ordinary status reads do
// not launch an encoder probe or create/reconfigure a container.
func (m *Go2RTCManager) CheckHardware(ctx context.Context) (Go2RTCHardwareResult, error) {
	cfg := m.Config()
	backend := cfg.Hardware.Backend
	if backend == "" {
		backend = "software"
	}
	result := Go2RTCHardwareResult{Backend: backend}
	if backend == "software" {
		result.Verified = true
		return result, nil
	}
	var probe struct {
		Sources []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"sources"`
	}
	if err := m.requestJSON(ctx, http.MethodGet, "/api/ffmpeg/hardware", nil, &probe, 1<<20); err != nil {
		m.mediaMu.Lock()
		m.hardwareVerified = false
		m.hardwareVerifiedAt = ""
		m.mediaMu.Unlock()
		m.clearHardwareVerification(cfg)
		result.ErrorCode = "hardware_probe_unavailable"
		return result, fmt.Errorf("camera hardware probe unavailable")
	}
	for _, source := range probe.Sources {
		if source.Name == "OK" && strings.Contains(source.URL, "#video=h264#hardware="+backend) {
			result.Verified = true
		}
	}
	_, hardwareKey := m.hardwareCache(cfg)
	m.mediaMu.Lock()
	if m.hardwareKey != hardwareKey {
		m.mediaMu.Unlock()
		result.Verified = false
		result.ErrorCode = "hardware_changed"
		return result, fmt.Errorf("hardware configuration changed during probe")
	}
	m.hardwareVerified = result.Verified
	m.hardwareVerifiedAt = ""
	if result.Verified {
		m.hardwareVerifiedAt = time.Now().UTC().Format(time.RFC3339)
		result.VerifiedAt = m.hardwareVerifiedAt
	}
	m.mediaMu.Unlock()
	if !result.Verified {
		m.clearHardwareVerification(cfg)
		result.ErrorCode = "hardware_encoder_unavailable"
		return result, fmt.Errorf("selected hardware H.264 encoder is unavailable")
	}
	m.saveHardwareVerification(cfg, result.VerifiedAt)
	return result, nil
}

func go2RTCHardwareKey(cfg config.Go2RTCHardwareConfig) string {
	data, _ := json.Marshal(cfg)
	return string(data)
}

func (m *Go2RTCManager) HardwareStatus() Go2RTCHardwareResult {
	cfg := m.Config()
	backend := cfg.Hardware.Backend
	if backend == "" {
		backend = "software"
	}
	m.mediaMu.Lock()
	defer m.mediaMu.Unlock()
	return Go2RTCHardwareResult{Backend: backend, Verified: backend == "software" || m.hardwareVerified, VerifiedAt: m.hardwareVerifiedAt}
}

func (m *Go2RTCManager) hardwareCache(cfg config.Go2RTCConfig) (string, string) {
	m.mu.RLock()
	dir, host := m.dataDir, m.docker.Host
	m.mu.RUnlock()
	hash := sha256.Sum256([]byte(go2RTCHardwareKey(cfg.Hardware) + "\x00" + Go2RTCImageFor(cfg) + "\x00" + host))
	return filepath.Join(dir, "go2rtc", "hardware.json"), hex.EncodeToString(hash[:])
}
func (m *Go2RTCManager) saveHardwareVerification(cfg config.Go2RTCConfig, at string) {
	path, key := m.hardwareCache(cfg)
	data, _ := json.Marshal(map[string]string{"key": key, "verified_at": at})
	if os.MkdirAll(filepath.Dir(path), 0755) == nil {
		if err := config.WriteFileAtomic(path, data, 0600); err != nil {
			m.logger.Warn("[go2rtc] Hardware verification cache could not be saved")
		}
	}
}
func (m *Go2RTCManager) loadHardwareVerification() {
	cfg := m.Config()
	path, key := m.hardwareCache(cfg)
	info, err := os.Stat(path)
	if err != nil || info.Size() > 4096 {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var record map[string]string
	if json.Unmarshal(data, &record) != nil || record["key"] != key {
		return
	}
	if _, err := time.Parse(time.RFC3339, record["verified_at"]); err != nil {
		return
	}
	m.mediaMu.Lock()
	m.hardwareVerified = true
	m.hardwareVerifiedAt = record["verified_at"]
	m.mediaMu.Unlock()
}

func (m *Go2RTCManager) clearHardwareVerification(cfg config.Go2RTCConfig) {
	path, _ := m.hardwareCache(cfg)
	_ = os.Remove(path)
}
