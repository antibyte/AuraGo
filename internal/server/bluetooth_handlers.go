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
	"strings"
	"time"

	"aurago/internal/bluetooth"
	"aurago/internal/config"
)

type bluetoothActionRequest struct {
	Operation string `json:"operation"`
	Address   string `json:"address"`
	PIN       string `json:"pin"`
}

func handleBluetoothStatus(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			bluetoothJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.Bluetooth == nil {
			bluetoothJSONError(w, &bluetooth.CodedError{Code: bluetooth.ErrorUnavailable, Message: "Bluetooth manager is unavailable."}, 0)
			return
		}
		status := s.Bluetooth.Status()
		devices := []bluetooth.Device{}
		if status.Usable {
			ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
			listed, err := s.Bluetooth.List(ctx)
			cancel()
			if err == nil {
				devices = listed
			}
		}
		cfg := s.ConfigSnapshot()
		response := map[string]interface{}{
			"status":   status,
			"devices":  devices,
			"playback": s.Bluetooth.PlaybackStatus(),
		}
		if cfg != nil {
			response["permissions"] = map[string]interface{}{
				"enabled":         cfg.Bluetooth.Enabled,
				"readonly":        cfg.Bluetooth.ReadOnly,
				"allow_playback":  cfg.Bluetooth.AllowPlayback,
				"default_device":  cfg.Bluetooth.DefaultDevice,
				"audio_backend":   cfg.Bluetooth.AudioBackend,
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
			bluetoothJSONError(w, &bluetooth.CodedError{Code: bluetooth.ErrorInvalidArgument, Message: "Invalid Bluetooth action request."}, 0)
			return
		}
		var err error
		switch strings.ToLower(strings.TrimSpace(request.Operation)) {
		case "pair":
			// The optional PIN is intentionally transient: it is passed directly
			// to BlueZ and is never persisted or logged.
			err = s.Bluetooth.Pair(r.Context(), request.Address, request.PIN)
			request.PIN = ""
		case "connect":
			err = s.Bluetooth.Connect(r.Context(), request.Address)
		case "disconnect":
			err = s.Bluetooth.Disconnect(r.Context(), request.Address)
		default:
			err = &bluetooth.CodedError{Code: bluetooth.ErrorInvalidArgument, Message: "Operation must be pair, connect, or disconnect."}
		}
		if err != nil {
			bluetoothJSONError(w, err, 0)
			return
		}
		devices, _ := s.Bluetooth.List(r.Context())
		writeJSON(w, map[string]interface{}{"status": "ok", "devices": devices})
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
		playback, err := s.Bluetooth.Play(r.Context(), source, body.Device)
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
	s.Cfg.Runtime.Bluetooth = status
	s.replaceConfigSnapshot(s.Cfg)
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
		case bluetooth.ErrorDeviceNotFound:
			status = http.StatusNotFound
		case bluetooth.ErrorDeviceAmbiguous, bluetooth.ErrorPairingInteractionRequired, bluetooth.ErrorDeviceNotPaired:
			status = http.StatusConflict
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
