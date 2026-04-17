package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/vishen/go-chromecast/application"
)

// ChromecastConfig holds Chromecast integration settings.
type ChromecastConfig struct {
	ServerHost string // Hostname/IP of the AuraGo server (for TTS playback URLs)
	ServerPort int    // Port of the AuraGo server
}

type chromecastDevice struct {
	Name string `json:"name"`
	Addr string `json:"addr"`
	Port int    `json:"port"`
	UUID string `json:"uuid,omitempty"`
}

// ChromecastDiscover scans the local network for Chromecast devices via mDNS.
func ChromecastDiscover(logger *slog.Logger) string {
	logger.Info("Starting Chromecast discovery via mDNS")

	entries, err := mdnsQueryServices("_googlecast._tcp", 10*time.Second, logger)
	if err != nil {
		return jsonErr("mDNS discovery failed: " + err.Error())
	}

	var devices []chromecastDevice
	for _, e := range entries {
		// Skip entries from _googlezone._tcp (not Chromecast devices)
		if strings.Contains(e.Name, "_googlezone") {
			continue
		}
		ip := ""
		if len(e.IPs) > 0 {
			ip = e.IPs[0]
		}
		port := e.Port
		if port == 0 {
			port = 8009
		}
		devices = append(devices, chromecastDevice{
			Name: strings.TrimSuffix(e.Name, "._googlecast._tcp.local."),
			Addr: ip,
			Port: port,
			UUID: strings.Join(e.TXTs, ", "),
		})
	}

	if len(devices) == 0 {
		return jsonOK(map[string]interface{}{
			"message": "No Chromecast devices found",
			"devices": []chromecastDevice{},
		})
	}

	return jsonOK(map[string]interface{}{
		"count":   len(devices),
		"devices": devices,
	})
}

// ChromecastPlay plays a media URL on a Chromecast device.
func ChromecastPlay(deviceAddr string, devicePort int, mediaURL, contentType string, logger *slog.Logger) string {
	if deviceAddr == "" {
		return jsonErr("'device_addr' is required (use 'discover' first)")
	}
	if mediaURL == "" {
		return jsonErr("'url' is required")
	}
	if contentType == "" {
		contentType = "audio/mpeg"
	}

	app, err := connectChromecast(deviceAddr, devicePort)
	if err != nil {
		return jsonErr("Failed to connect: " + err.Error())
	}
	defer app.Close(false)

	// forceDetach=true: return immediately after sending the LOAD command.
	// Without this, Load() calls MediaWait() which blocks until playback ends —
	// an infinite wait for live radio streams.
	if err := app.Load(mediaURL, 0, contentType, false, false, true); err != nil {
		return jsonErr("Failed to load media: " + err.Error())
	}

	return jsonOK(map[string]interface{}{
		"message": fmt.Sprintf("Playing %s on %s", mediaURL, deviceAddr),
	})
}

// ChromecastSpeak generates TTS audio and plays it on a Chromecast device.
func ChromecastSpeak(deviceAddr string, devicePort int, text string, ttsCfg TTSConfig, ccCfg ChromecastConfig, logger *slog.Logger) string {
	if deviceAddr == "" {
		return jsonErr("'device_addr' is required")
	}
	if text == "" {
		return jsonErr("'text' is required")
	}

	// Generate TTS audio
	filename, err := TTSSynthesize(ttsCfg, text)
	if err != nil {
		return jsonErr("TTS failed: " + err.Error())
	}

	// Build URL the Chromecast can reach
	host := ccCfg.ServerHost
	if host == "" || host == "127.0.0.1" || host == "0.0.0.0" {
		// Try to find the local LAN IP
		if ip := getOutboundIP(); ip != "" {
			host = ip
		} else {
			host = "127.0.0.1"
		}
	}
	audioURL := fmt.Sprintf("http://%s:%d/tts/%s", host, ccCfg.ServerPort, filename)

	// Cast to device
	app, err := connectChromecast(deviceAddr, devicePort)
	if err != nil {
		return jsonErr("Failed to connect to Chromecast: " + err.Error())
	}
	defer app.Close(false)

	// forceDetach=true: return immediately after the LOAD command is sent.
	// Without this, Load() blocks until the audio finishes playing on the device.
	if err := app.Load(audioURL, 0, "audio/mpeg", false, false, true); err != nil {
		return jsonErr("Failed to cast audio: " + err.Error())
	}

	return jsonOK(map[string]interface{}{
		"message": fmt.Sprintf("Speaking on %s: %s", deviceAddr, text),
		"audio":   audioURL,
	})
}

// ChromecastStop stops playback on a Chromecast device.
func ChromecastStop(deviceAddr string, devicePort int, logger *slog.Logger) string {
	if deviceAddr == "" {
		return jsonErr("'device_addr' is required")
	}

	app, err := connectChromecast(deviceAddr, devicePort)
	if err != nil {
		return jsonErr("Failed to connect: " + err.Error())
	}
	defer app.Close(false)

	if err := app.StopMedia(); err != nil {
		return jsonErr("Failed to stop: " + err.Error())
	}

	return jsonOK(map[string]interface{}{"message": "Playback stopped"})
}

// ChromecastVolume sets volume on a Chromecast device (0.0–1.0).
func ChromecastVolume(deviceAddr string, devicePort int, level float64, logger *slog.Logger) string {
	if deviceAddr == "" {
		return jsonErr("'device_addr' is required")
	}

	app, err := connectChromecast(deviceAddr, devicePort)
	if err != nil {
		return jsonErr("Failed to connect: " + err.Error())
	}
	defer app.Close(false)

	if err := app.SetVolume(float32(level)); err != nil {
		return jsonErr("Failed to set volume: " + err.Error())
	}

	return jsonOK(map[string]interface{}{
		"message": fmt.Sprintf("Volume set to %.0f%%", level*100),
	})
}

// ChromecastStatus returns the current status of a Chromecast device.
func ChromecastStatus(deviceAddr string, devicePort int, logger *slog.Logger) string {
	if deviceAddr == "" {
		return jsonErr("'device_addr' is required")
	}

	app, err := connectChromecast(deviceAddr, devicePort)
	if err != nil {
		return jsonErr("Failed to connect: " + err.Error())
	}
	defer app.Close(false)

	castApp, _, castVol := app.Status()

	info := map[string]interface{}{
		"device": deviceAddr,
	}
	if castVol != nil {
		info["volume"] = castVol.Level
		info["muted"] = castVol.Muted
	}
	if castApp != nil {
		info["app"] = castApp.DisplayName
		info["app_id"] = castApp.AppId
	}

	return jsonOK(info)
}

// connectChromecast creates a connection to a Chromecast device.
func connectChromecast(addr string, port int) (*application.Application, error) {
	if port == 0 {
		port = 8009 // Default Chromecast port
	}

	app := application.NewApplication()
	if err := app.Start(addr, port); err != nil {
		return nil, fmt.Errorf("failed to start connection: %w", err)
	}

	return app, nil
}

// getOutboundIP returns the preferred outbound LAN IP of this machine.
func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// jsonOK builds a success JSON response.
func jsonOK(data map[string]interface{}) string {
	data["status"] = "success"
	out, _ := json.Marshal(data)
	return string(out)
}

// jsonErr builds an error JSON response.
func jsonErr(msg string) string {
	out, _ := json.Marshal(map[string]string{"status": "error", "message": msg})
	return string(out)
}

// ParseChromecastPort parses port from action params, defaulting to 8009.
func ParseChromecastPort(raw interface{}) int {
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if strings.TrimSpace(v) == "" {
			return 8009
		}
	}
	return 8009
}
