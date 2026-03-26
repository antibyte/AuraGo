package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"path/filepath"
	"time"

	"aurago/internal/config"
)

const piperContainerName = "aurago-piper-tts"

// EnsurePiperRunning ensures a managed Piper TTS container is running.
// Follows the same idempotent pattern as EnsureGotenbergRunning / EnsureOllamaEmbeddingsRunning.
func EnsurePiperRunning(cfg *config.Config, logger *slog.Logger) {
	p := cfg.TTS.Piper
	if !p.Enabled {
		logger.Info("[Piper TTS] Skipping auto-start (disabled in config)")
		return
	}
	if p.Voice == "" {
		logger.Warn("[Piper TTS] No voice configured, skipping container auto-start")
		return
	}

	dockerCfg := DockerConfig{Host: cfg.Docker.Host}
	port := p.ContainerPort
	if port <= 0 {
		port = 10200
	}
	portStr := fmt.Sprintf("%d", port)

	image := p.Image
	if image == "" {
		image = "rhasspy/wyoming-piper:latest"
	}

	dataPath := p.DataPath
	if dataPath == "" {
		dataPath = "data/piper"
	}
	absData, err := filepath.Abs(dataPath)
	if err != nil {
		logger.Error("[Piper TTS] Failed to resolve data path", "path", dataPath, "error", err)
		return
	}

	// Check if ANY piper container is already serving on our port
	listData, listCode, listErr := dockerRequest(dockerCfg, "GET",
		fmt.Sprintf(`/containers/json?filters={"status":["running"],"ancestor":[%q]}`, image), "")
	if listErr == nil && listCode == 200 {
		var containers []map[string]interface{}
		if json.Unmarshal(listData, &containers) == nil && len(containers) > 0 {
			logger.Info("[Piper TTS] Container already running (external)", "count", len(containers))
			waitForPiperReady(port, logger)
			return
		}
	}

	// Inspect our managed container
	data, code, inspectErr := dockerRequest(dockerCfg, "GET", "/containers/"+piperContainerName+"/json", "")
	if inspectErr != nil {
		logger.Warn("[Piper TTS] Docker unavailable, skipping auto-start", "error", inspectErr)
		return
	}

	if code == 200 {
		// Container exists — check if running
		var info map[string]interface{}
		if json.Unmarshal(data, &info) == nil {
			if state, ok := info["State"].(map[string]interface{}); ok {
				if running, _ := state["Running"].(bool); running {
					logger.Info("[Piper TTS] Container already running")
					waitForPiperReady(port, logger)
					return
				}
			}
		}
		// Exists but stopped — start it
		_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+piperContainerName+"/start", "")
		if startErr != nil || (startCode != 204 && startCode != 304) {
			logger.Error("[Piper TTS] Failed to start existing container", "code", startCode, "error", startErr)
			return
		}
		logger.Info("[Piper TTS] Container started")
		waitForPiperReady(port, logger)
		return
	}

	if code != 404 {
		logger.Warn("[Piper TTS] Unexpected Docker inspect response, skipping auto-start", "code", code)
		return
	}

	// Container does not exist — pull image if needed, then create and start
	logger.Info("[Piper TTS] Pulling image", "image", image)
	_, pullCode, pullErr := dockerRequest(dockerCfg, "POST", "/images/create?fromImage="+url.QueryEscape(image), "")
	if pullErr != nil {
		logger.Warn("[Piper TTS] Image pull failed, trying to create container anyway", "error", pullErr)
	} else if pullCode != 200 {
		logger.Warn("[Piper TTS] Image pull returned unexpected status", "code", pullCode)
	}

	payload := map[string]interface{}{
		"Image": image,
		"Cmd":   []string{"--voice", p.Voice},
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"PortBindings": map[string]interface{}{
				"10200/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": portStr}},
			},
			"Binds": []string{absData + ":/data"},
		},
		"ExposedPorts": map[string]interface{}{
			"10200/tcp": struct{}{},
		},
	}
	body, _ := json.Marshal(payload)
	_, createCode, createErr := dockerRequest(dockerCfg, "POST", "/containers/create?name="+piperContainerName, string(body))
	if createErr != nil || createCode != 201 {
		logger.Error("[Piper TTS] Failed to create container", "code", createCode, "error", createErr)
		return
	}

	_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+piperContainerName+"/start", "")
	if startErr != nil || (startCode != 204 && startCode != 304) {
		logger.Error("[Piper TTS] Failed to start new container", "code", startCode, "error", startErr)
		return
	}
	logger.Info("[Piper TTS] Container created and started", "image", image, "port", port, "voice", p.Voice)

	waitForPiperReady(port, logger)
}

// waitForPiperReady polls the Piper Wyoming server until it responds or times out.
func waitForPiperReady(port int, logger *slog.Logger) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			logger.Info("[Piper TTS] Wyoming server is ready", "port", port)
			return
		}
		time.Sleep(2 * time.Second)
	}
	logger.Warn("[Piper TTS] Timed out waiting for Wyoming server", "port", port)
}

// PiperHealth checks if the Piper container is reachable and returns a status JSON string.
func PiperHealth(port int) map[string]interface{} {
	if port <= 0 {
		port = 10200
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := WyomingConnect(addr)
	if err != nil {
		return map[string]interface{}{
			"status":  "stopped",
			"message": err.Error(),
		}
	}
	defer conn.Close()

	voices, err := WyomingDescribe(conn)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("describe failed: %v", err),
		}
	}

	voiceNames := make([]string, 0, len(voices))
	for _, v := range voices {
		voiceNames = append(voiceNames, v.Name)
	}

	return map[string]interface{}{
		"status":           "running",
		"voices_available": len(voices),
		"voices":           voiceNames,
	}
}

// PiperListVoices connects to the Piper Wyoming server and returns available voices.
func PiperListVoices(port int) ([]WyomingVoice, error) {
	if port <= 0 {
		port = 10200
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := WyomingConnect(addr)
	if err != nil {
		return nil, fmt.Errorf("piper connect for voice list: %w", err)
	}
	defer conn.Close()
	return WyomingDescribe(conn)
}
