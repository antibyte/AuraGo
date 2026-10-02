package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"aurago/internal/bluetooth"
	"aurago/internal/config"
)

type bluetoothActionRequest struct {
	Operation   string `json:"operation"`
	Address     string `json:"address"`
	PIN         string `json:"pin"`
	Wait        *bool  `json:"wait"`
	Interactive bool   `json:"interactive"`
}

var bluetoothInteractionIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func bluetoothManagerUnavailable(w http.ResponseWriter) {
	bluetoothJSONError(w, &bluetooth.CodedError{Code: bluetooth.ErrorUnavailable, Message: "Bluetooth manager is unavailable."}, 0)
}

func bluetoothInvalid(w http.ResponseWriter, message string) {
	bluetoothJSONError(w, &bluetooth.CodedError{Code: bluetooth.ErrorInvalidArgument, Message: message}, 0)
}

func handleBluetoothStatus(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			bluetoothJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.Bluetooth == nil {
			bluetoothManagerUnavailable(w)
			return
		}
		snapshot := s.Bluetooth.Snapshot(r.Context())
		cfg := s.ConfigSnapshot()
		response := map[string]interface{}{
			"status":         s.Bluetooth.Status(),
			"devices":        snapshot.Devices,
			"playback":       s.Bluetooth.PlaybackStatus(),
			"revision":       snapshot.Revision,
			"present":        snapshot.Present,
			"reason":         snapshot.Reason,
			"adapter":        snapshot.Adapter,
			"discovery":      snapshot.Discovery,
			"discoverable":   snapshot.Discoverable,
			"interaction_id": snapshot.InteractionID,
		}
		if cfg != nil {
			response["permissions"] = map[string]interface{}{
				"enabled":          cfg.Bluetooth.Enabled,
				"readonly":         cfg.Bluetooth.ReadOnly,
				"allow_playback":   cfg.Bluetooth.AllowPlayback,
				"default_device":   cfg.Bluetooth.DefaultDevice,
				"audio_backend":    cfg.Bluetooth.AudioBackend,
				"scan_timeout_sec": cfg.Bluetooth.ScanTimeoutSeconds,
			}
		}
		writeJSON(w, response)
	})
}

func handleBluetoothReprobe(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			bluetoothJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		status, err := refreshBluetoothRuntime(r.Context(), s)
		if err != nil {
			bluetoothJSONError(w, err, 0)
			return
		}
		writeJSON(w, map[string]interface{}{"status": status})
	})
}

func handleBluetoothDiscover(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			bluetoothJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.Bluetooth == nil {
			bluetoothJSONError(w, &bluetooth.CodedError{Code: bluetooth.ErrorUnavailable, Message: "Bluetooth manager is unavailable."}, 0)
			return
		}
		var body struct {
			TimeoutSeconds int `json:"timeout_seconds"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body)
		}
		timeout := time.Duration(body.TimeoutSeconds) * time.Second
		devices, err := s.Bluetooth.Discover(r.Context(), timeout)
		if err != nil {
			bluetoothJSONError(w, err, 0)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok", "devices": devices})
	})
}

func handleBluetoothDeviceAction(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			bluetoothJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.Bluetooth == nil {
			bluetoothJSONError(w, &bluetooth.CodedError{Code: bluetooth.ErrorUnavailable, Message: "Bluetooth manager is unavailable."}, 0)
			return
		}
		var request bluetoothActionRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&request); err != nil {
			bluetoothInvalid(w, "Invalid Bluetooth action request.")
			return
		}
		wait := request.Wait == nil || *request.Wait
		// The optional PIN is intentionally transient: it goes straight to BlueZ
		// and is never persisted or logged.
		err := s.Bluetooth.RunDeviceOperation(r.Context(), bluetooth.ActorOperator, bluetooth.DeviceRequest{
			Operation: request.Operation, Address: request.Address, PIN: request.PIN, Interactive: request.Interactive,
		}, wait)
		request.PIN = ""
		if err != nil {
			bluetoothJSONError(w, err, 0)
			return
		}
		if !wait {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "accepted"})
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok", "devices": s.Bluetooth.Snapshot(r.Context()).Devices})
	})
}

func handleBluetoothPower(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			bluetoothJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.Bluetooth == nil {
			bluetoothManagerUnavailable(w)
			return
		}
		var body struct {
			Powered *bool `json:"powered"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil || body.Powered == nil {
			bluetoothInvalid(w, "powered is required.")
			return
		}
		if err := s.Bluetooth.SetPowered(r.Context(), bluetooth.ActorOperator, *body.Powered); err != nil {
			bluetoothJSONError(w, err, 0)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok"})
	})
}

func handleBluetoothDiscovery(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			bluetoothJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.Bluetooth == nil {
			bluetoothManagerUnavailable(w)
			return
		}
		var body struct {
			Action         string `json:"action"`
			TimeoutSeconds int    `json:"timeout_seconds"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil {
			bluetoothInvalid(w, "Invalid discovery request.")
			return
		}
		switch body.Action {
		case "start":
			state, err := s.Bluetooth.StartDiscovery(r.Context(), bluetooth.ActorOperator, time.Duration(body.TimeoutSeconds)*time.Second)
			if err != nil {
				bluetoothJSONError(w, err, 0)
				return
			}
			writeJSON(w, map[string]interface{}{"status": "ok", "discovery": state})
		case "stop":
			_ = s.Bluetooth.StopDiscovery(r.Context(), bluetooth.ActorOperator)
			writeJSON(w, map[string]interface{}{"status": "ok"})
		default:
			bluetoothInvalid(w, "action must be start or stop.")
		}
	})
}

func handleBluetoothDiscoverable(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			bluetoothJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.Bluetooth == nil {
			bluetoothManagerUnavailable(w)
			return
		}
		var body struct {
			Enabled         *bool `json:"enabled"`
			DurationSeconds int   `json:"duration_seconds"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil || body.Enabled == nil {
			bluetoothInvalid(w, "enabled is required.")
			return
		}
		if !*body.Enabled {
			_ = s.Bluetooth.StopDiscoverable(r.Context(), bluetooth.ActorOperator)
			writeJSON(w, map[string]interface{}{"status": "ok"})
			return
		}
		if body.DurationSeconds != 0 && (body.DurationSeconds < 60 || body.DurationSeconds > 600) {
			bluetoothInvalid(w, "duration_seconds must be between 60 and 600.")
			return
		}
		state, err := s.Bluetooth.SetDiscoverable(r.Context(), bluetooth.ActorOperator, time.Duration(body.DurationSeconds)*time.Second)
		if err != nil {
			bluetoothJSONError(w, err, 0)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok", "discoverable": state})
	})
}

func handleBluetoothInteraction(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			bluetoothJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.Bluetooth == nil {
			bluetoothManagerUnavailable(w)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/bluetooth/interactions/")
		if !bluetoothInteractionIDPattern.MatchString(id) {
			bluetoothInvalid(w, "Invalid pairing request id.")
			return
		}
		if r.Method == http.MethodGet {
			view, err := s.Bluetooth.Interaction(id)
			if err != nil {
				bluetoothJSONError(w, err, 0)
				return
			}
			writeJSON(w, map[string]interface{}{"status": "ok", "interaction": view})
			return
		}
		var body struct {
			Accept bool   `json:"accept"`
			Value  string `json:"value"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil {
			bluetoothInvalid(w, "Invalid pairing answer.")
			return
		}
		err := s.Bluetooth.AnswerInteraction(id, body.Accept, body.Value)
		body.Value = ""
		if err != nil {
			bluetoothJSONError(w, err, 0)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok"})
	})
}

func handleBluetoothAudioTest(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			bluetoothJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.Bluetooth == nil {
			bluetoothJSONError(w, &bluetooth.CodedError{Code: bluetooth.ErrorUnavailable, Message: "Bluetooth manager is unavailable."}, 0)
			return
		}
		var body struct {
			Device string `json:"device"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body)
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			bluetoothJSONError(w, fmt.Errorf("configuration is unavailable"), http.StatusInternalServerError)
			return
		}
		source, err := ensureBluetoothTestTone(r.Context(), cfg.Directories.DataDir)
		if err != nil {
			bluetoothJSONError(w, err, http.StatusInternalServerError)
			return
		}
		playback, err := s.Bluetooth.Play(r.Context(), bluetooth.ActorOperator, source, body.Device)
		if err != nil {
			bluetoothJSONError(w, err, 0)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok", "playback": playback})
	})
}

func handleBluetoothAudioStop(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			bluetoothJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.Bluetooth == nil {
			bluetoothJSONError(w, &bluetooth.CodedError{Code: bluetooth.ErrorUnavailable, Message: "Bluetooth manager is unavailable."}, 0)
			return
		}
		if err := s.Bluetooth.Stop(); err != nil {
			bluetoothJSONError(w, err, 0)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok", "playback": s.Bluetooth.PlaybackStatus()})
	})
}

func refreshBluetoothRuntime(parent context.Context, s *Server) (bluetooth.Status, error) {
	if s == nil || s.Bluetooth == nil {
		return bluetooth.Status{}, &bluetooth.CodedError{Code: bluetooth.ErrorUnavailable, Message: "Bluetooth manager is unavailable."}
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return bluetooth.Status{}, fmt.Errorf("configuration is unavailable")
	}
	s.Bluetooth.Configure(config.BluetoothRuntimeOptions(cfg))
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	status := s.Bluetooth.Reprobe(ctx)
	cancel()

	s.CfgMu.Lock()
	if current := s.ConfigSnapshot(); current != nil {
		updated := *current
		updated.Runtime.Bluetooth = status
		s.replaceConfigSnapshot(&updated)
	}
	s.CfgMu.Unlock()
	return status, nil
}

func ensureBluetoothTestTone(ctx context.Context, dataDir string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", fmt.Errorf("data directory is not configured")
	}
	directory := filepath.Join(dataDir, "bluetooth")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create Bluetooth data directory: %w", err)
	}
	path := filepath.Join(directory, "test-tone.wav")
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return path, nil
	}
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=660:duration=1.2",
		"-ac", "2", "-ar", "48000", "-y", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("generate Bluetooth test tone: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return path, nil
}

func bluetoothJSONError(w http.ResponseWriter, err error, explicitStatus int) {
	status := explicitStatus
	code := bluetooth.ErrorCode(err)
	if status == 0 {
		switch code {
		case bluetooth.ErrorUnavailable, bluetooth.ErrorAudioTargetUnavailable:
			status = http.StatusServiceUnavailable
		case bluetooth.ErrorReadOnly, bluetooth.ErrorPlaybackDisabled:
			status = http.StatusForbidden
		case bluetooth.ErrorDeviceNotFound, bluetooth.ErrorInteractionNotFound:
			status = http.StatusNotFound
		case bluetooth.ErrorDeviceAmbiguous, bluetooth.ErrorPairingInteractionRequired, bluetooth.ErrorDeviceNotPaired,
			bluetooth.ErrorPairingRejected, bluetooth.ErrorPairingFailed, bluetooth.ErrorOperationBusy,
			bluetooth.ErrorPoweredOff, bluetooth.ErrorBlocked, bluetooth.ErrorInteractionExpired,
			bluetooth.ErrorProfileUnavailable:
			status = http.StatusConflict
		case bluetooth.ErrorDeviceUnreachable:
			status = http.StatusGatewayTimeout
		case bluetooth.ErrorInvalidArgument:
			status = http.StatusBadRequest
		default:
			status = http.StatusInternalServerError
		}
	}
	if code == "" {
		code = "BLUETOOTH_ERROR"
	}
	message := "Bluetooth operation failed."
	if err != nil {
		message = err.Error()
	}
	var coded *bluetooth.CodedError
	if errors.As(err, &coded) && coded.Message != "" {
		message = coded.Message
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "error",
		"code":    code,
		"message": message,
	})
}
