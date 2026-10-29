package tools

import (
	"aurago/internal/security"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vishen/go-chromecast/application"
)

// ChromecastConfig holds Chromecast integration settings.
type ChromecastConfig struct {
	Context            context.Context `json:"-"` // Request owner; nil retains the standalone API lifetime.
	ServerHost         string          // Hostname/IP of the AuraGo server (for media playback URLs)
	ServerPort         int             // Port of the AuraGo media server
	MediaHostAllowlist []string        // Explicit private media hosts/IPs/CIDRs allowed for Chromecast playback
	KnownDevices       []ChromecastDevice
	ReadOnly           bool
}

type ChromecastDevice struct {
	Name         string `json:"name"`
	FriendlyName string `json:"friendly_name,omitempty"`
	Addr         string `json:"addr"`
	Port         int    `json:"port"`
	UUID         string `json:"uuid,omitempty"`
	Model        string `json:"model,omitempty"`
	Capabilities string `json:"capabilities,omitempty"`
	Group        bool   `json:"group,omitempty"`
	AudioOnly    bool   `json:"audio_only,omitempty"`
}

var chromecastMDNSQuery = mdnsQueryServices
var chromecastHTTPClientFactory = func(cfg ChromecastConfig) *http.Client {
	return newChromecastMediaHTTPClient(cfg, 5*time.Second)
}

// DiscoverChromecastDevices scans the local network for Chromecast devices via mDNS.
func DiscoverChromecastDevices(logger *slog.Logger) ([]ChromecastDevice, error) {
	logger.Info("Starting Chromecast discovery via mDNS")

	entries, err := chromecastMDNSQuery("_googlecast._tcp", 10*time.Second, logger)
	if err != nil {
		return nil, fmt.Errorf("mDNS discovery failed: %w", err)
	}

	var devices []ChromecastDevice
	for _, e := range entries {
		// Skip entries from _googlezone._tcp (not Chromecast devices)
		if strings.Contains(e.Name, "_googlezone") {
			continue
		}
		port := e.Port
		if port == 0 {
			port = 8009
		}
		model := chromecastTXTValue(e.TXTs, "md")
		capabilities := chromecastTXTValue(e.TXTs, "ca")
		group, audioOnly := chromecastDeviceLabels(model, capabilities)
		devices = append(devices, ChromecastDevice{
			Name:         strings.TrimSuffix(e.Name, "._googlecast._tcp.local."),
			FriendlyName: chromecastTXTValue(e.TXTs, "fn"),
			Addr:         chromecastPreferredIPv4(e.IPs),
			Port:         port,
			UUID:         chromecastTXTValue(e.TXTs, "id"),
			Model:        model,
			Capabilities: capabilities,
			Group:        group,
			AudioOnly:    audioOnly,
		})
	}

	return devices, nil
}

// ChromecastDiscover scans the local network for Chromecast devices via mDNS and returns JSON.
func ChromecastDiscover(logger *slog.Logger) string {
	devices, err := CachedDiscoverChromecastDevices(logger)
	if err != nil {
		return jsonErr(err.Error())
	}

	if len(devices) == 0 {
		return jsonOK(map[string]interface{}{
			"message": "No Chromecast devices found",
			"devices": []ChromecastDevice{},
		})
	}

	return jsonOK(map[string]interface{}{
		"count":   len(devices),
		"devices": devices,
	})
}

// FindChromecastDeviceByName matches either the mDNS service name or the device's friendly name.
func FindChromecastDeviceByName(devices []ChromecastDevice, requested string) (ChromecastDevice, bool) {
	target := canonicalChromecastName(requested)
	if target == "" {
		return ChromecastDevice{}, false
	}

	for _, device := range devices {
		for _, candidate := range []string{device.FriendlyName, device.Name} {
			if canonicalChromecastName(candidate) == target {
				return device, true
			}
		}
	}

	matchIdx := -1
	for i, device := range devices {
		for _, candidate := range []string{device.FriendlyName, device.Name} {
			normalized := canonicalChromecastName(candidate)
			if normalized == "" {
				continue
			}
			if strings.Contains(normalized, target) || strings.Contains(target, normalized) {
				if matchIdx != -1 {
					return ChromecastDevice{}, false
				}
				matchIdx = i
				break
			}
		}
	}
	if matchIdx == -1 {
		return ChromecastDevice{}, false
	}
	return devices[matchIdx], true
}

// ChromecastPlay plays a media URL on a Chromecast device.
// streamType "live" is reported to the caller only. The receiver load stays buffered.
func ChromecastPlay(deviceAddr string, devicePort int, mediaURL, contentType, streamType string, ccCfg ChromecastConfig, logger *slog.Logger) string {
	if deviceAddr == "" {
		return jsonErr("'device_addr' is required (use 'discover' first)")
	}
	if mediaURL == "" {
		return jsonErr("'url' is required")
	}
	if refused := chromecastMutationRefused(deviceAddr, ccCfg); refused != "" {
		return refused
	}
	if err := AuthorizeChromecastDial(deviceAddr, devicePort, ccCfg.KnownDevices); err != nil {
		return jsonErr("Failed to connect: " + err.Error())
	}
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	if err := validateChromecastMediaURL(mediaURL, ccCfg); err != nil {
		return jsonErr("Media URL is not reachable: " + err.Error())
	}

	app, err := connectChromecastController(deviceAddr, devicePort, ccCfg)
	if err != nil {
		return jsonErr("Failed to connect: " + err.Error())
	}
	defer app.Close(false)

	// forceDetach=true: return after LOAD is sent, then confirm the receiver state.
	// Load() without forceDetach waits until playback ends, including live streams.
	if err := chromecastLoadAndConfirm(app, ccCfg.Context, mediaURL, contentType); err != nil {
		return jsonErr("Failed to load media: " + err.Error())
	}

	result := map[string]interface{}{
		"message": fmt.Sprintf("Playing %s on %s", chromecastPublicMediaURL(mediaURL), deviceAddr),
	}
	if strings.EqualFold(strings.TrimSpace(streamType), "live") {
		result["stream_type"] = "live"
	}
	return jsonOK(result)
}

func validateChromecastMediaURL(mediaURL string, ccCfg ChromecastConfig) error {
	if err := validateChromecastMediaURLPolicy(mediaURL, ccCfg); err != nil {
		return err
	}
	resp, err := chromecastProbeMedia(ccCfg, mediaURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if contentType != "" && strings.Contains(contentType, "text/html") {
		return fmt.Errorf("YouTube pages are unsupported and a direct media file is required")
	}
	return nil
}

func chromecastProbeMedia(ccCfg ChromecastConfig, mediaURL string) (*http.Response, error) {
	client := chromecastHTTPClientFactory(ccCfg)
	req, err := newToolHTTPRequest(ccCfg.Context, http.MethodHead, mediaURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotImplemented {
		return resp, nil
	}
	resp.Body.Close()
	req, err = newToolHTTPRequest(ccCfg.Context, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", "bytes=0-0")
	return client.Do(req)
}

func validateChromecastMediaURLPolicy(rawURL string, ccCfg ChromecastConfig) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("disallowed URL scheme %q: only http and https are permitted", parsed.Scheme)
	}
	host := chromecastCanonicalHost(parsed.Hostname())
	if host == "" {
		return fmt.Errorf("URL has no host")
	}
	port := chromecastURLPort(parsed)
	if chromecastForbiddenHost(host) {
		return chromecastSSRFError(host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if chromecastForbiddenIP(ip) {
			return chromecastSSRFError(ip.String())
		}
		if chromecastAuraGoMediaURL(parsed, ccCfg) {
			return nil
		}
		if chromecastInternalIP(ip) && !chromecastAllowlistMatchesHost(host, port, ccCfg.MediaHostAllowlist) && !chromecastAllowlistMatchesIP(ip, port, ccCfg.MediaHostAllowlist) {
			return chromecastSSRFError(ip.String())
		}
		return nil
	}
	if chromecastAuraGoMediaURL(parsed, ccCfg) {
		return nil
	}
	if chromecastAllowlistMatchesHost(host, port, ccCfg.MediaHostAllowlist) {
		return nil
	}
	return nil
}

func newChromecastMediaHTTPClient(ccCfg ChromecastConfig, timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 15 * time.Second}

	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		targetAddr, _, err := validatedChromecastDialTarget(ctx, addr, ccCfg)
		if err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, targetAddr)
	}
	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		targetAddr, serverName, err := validatedChromecastDialTarget(ctx, addr, ccCfg)
		if err != nil {
			return nil, err
		}
		rawConn, err := dialer.DialContext(ctx, network, targetAddr)
		if err != nil {
			return nil, err
		}
		tlsConn := tls.Client(rawConn, &tls.Config{
			ServerName: serverName,
			MinVersion: tls.VersionTLS12,
		})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			rawConn.Close()
			return nil, err
		}
		return tlsConn, nil
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return validateChromecastMediaURLPolicy(req.URL.String(), ccCfg)
	}
	return client
}

func validatedChromecastDialTarget(ctx context.Context, addr string, ccCfg ChromecastConfig) (networkAddr string, serverName string, err error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("invalid target address %q: %w", addr, err)
	}
	host = chromecastCanonicalHost(host)
	serverName = host
	if chromecastForbiddenHost(host) {
		return "", "", chromecastSSRFError(host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if err := validateChromecastDialIP(ip, host, port, ccCfg); err != nil {
			return "", "", err
		}
		return net.JoinHostPort(ip.String(), port), serverName, nil
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", "", fmt.Errorf("hostname resolution failed for %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return "", "", fmt.Errorf("hostname resolution failed for %q: no A/AAAA records found", host)
	}

	var selected net.IP
	for _, addr := range addrs {
		ip := addr.IP
		if err := validateChromecastDialIP(ip, host, port, ccCfg); err != nil {
			return "", "", err
		}
		if selected == nil {
			selected = ip
		}
	}
	return net.JoinHostPort(selected.String(), port), serverName, nil
}

func validateChromecastDialIP(ip net.IP, host, port string, ccCfg ChromecastConfig) error {
	if chromecastForbiddenIP(ip) {
		return chromecastSSRFError(ip.String())
	}
	if !chromecastInternalIP(ip) {
		return nil
	}
	if chromecastAuraGoHostPort(host, port, ccCfg) || chromecastAllowlistMatchesIP(ip, port, ccCfg.MediaHostAllowlist) || chromecastAllowlistMatchesHost(host, port, ccCfg.MediaHostAllowlist) {
		return nil
	}
	return chromecastSSRFError(ip.String())
}

func chromecastAuraGoMediaURL(parsed *url.URL, ccCfg ChromecastConfig) bool {
	if parsed == nil || !chromecastAuraGoHostPort(parsed.Hostname(), chromecastURLPort(parsed), ccCfg) {
		return false
	}
	path := parsed.EscapedPath()
	return strings.HasPrefix(path, "/tts/") || strings.HasPrefix(path, "/cast-media/")
}

func chromecastAuraGoHostPort(host, port string, ccCfg ChromecastConfig) bool {
	configuredHost := chromecastCanonicalHost(ccCfg.ServerHost)
	if configuredHost == "" || ccCfg.ServerPort <= 0 {
		return false
	}
	if chromecastForbiddenHost(configuredHost) {
		return false
	}
	if ip := net.ParseIP(configuredHost); ip != nil && chromecastForbiddenIP(ip) {
		return false
	}
	return chromecastCanonicalHost(host) == configuredHost && port == fmt.Sprint(ccCfg.ServerPort)
}

func chromecastURLPort(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}
	if port := parsed.Port(); port != "" {
		return port
	}
	switch parsed.Scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func chromecastAllowlistMatchesHost(host, port string, allowlist []string) bool {
	host = chromecastCanonicalHost(host)
	for _, entry := range allowlist {
		entry = strings.TrimSpace(entry)
		if entry == "" || strings.Contains(entry, "/") {
			continue
		}
		entryHost, entryPort, hasPort := chromecastSplitAllowlistHost(entry)
		if entryHost == "" || entryHost != host {
			continue
		}
		if hasPort && entryPort != port {
			continue
		}
		return true
	}
	return false
}

func chromecastAllowlistMatchesIP(ip net.IP, port string, allowlist []string) bool {
	for _, entry := range allowlist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			if _, ipNet, err := net.ParseCIDR(entry); err == nil && ipNet.Contains(ip) {
				return true
			}
			continue
		}
		entryHost, entryPort, hasPort := chromecastSplitAllowlistHost(entry)
		entryIP := net.ParseIP(entryHost)
		if entryIP == nil || !entryIP.Equal(ip) {
			continue
		}
		if hasPort && entryPort != port {
			continue
		}
		return true
	}
	return false
}

func chromecastSplitAllowlistHost(entry string) (host, port string, hasPort bool) {
	entry = strings.TrimSpace(entry)
	if h, p, err := net.SplitHostPort(entry); err == nil {
		return chromecastCanonicalHost(h), p, true
	}
	if i := strings.LastIndex(entry, ":"); i > 0 && !strings.Contains(entry[:i], ":") {
		return chromecastCanonicalHost(entry[:i]), strings.TrimSpace(entry[i+1:]), true
	}
	return chromecastCanonicalHost(entry), "", false
}

func chromecastCanonicalHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.Trim(host, "[]")
	return strings.TrimSuffix(host, ".")
}

func chromecastNeedsLANReachableHost(host string) bool {
	normalized := chromecastCanonicalHost(host)
	switch normalized {
	case "", "0.0.0.0", "localhost":
		return true
	}
	if ip := net.ParseIP(normalized); ip != nil {
		return ip.IsLoopback() || ip.IsUnspecified()
	}
	return false
}

func chromecastForbiddenHost(host string) bool {
	host = chromecastCanonicalHost(host)
	return host == "localhost"
}

func chromecastInternalIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, cidr := range chromecastInternalCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func chromecastForbiddenIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, cidr := range chromecastForbiddenCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func chromecastSSRFError(target string) error {
	return fmt.Errorf("access to internal address %s is blocked (SSRF protection)", target)
}

func mustChromecastCIDR(cidr string) *net.IPNet {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return ipNet
}

var chromecastForbiddenCIDRs = []*net.IPNet{
	mustChromecastCIDR("0.0.0.0/8"),
	mustChromecastCIDR("127.0.0.0/8"),
	mustChromecastCIDR("169.254.0.0/16"),
	mustChromecastCIDR("::/128"),
	mustChromecastCIDR("::1/128"),
	mustChromecastCIDR("fe80::/10"),
}

var chromecastInternalCIDRs = []*net.IPNet{
	mustChromecastCIDR("0.0.0.0/8"),
	mustChromecastCIDR("10.0.0.0/8"),
	mustChromecastCIDR("100.64.0.0/10"),
	mustChromecastCIDR("127.0.0.0/8"),
	mustChromecastCIDR("169.254.0.0/16"),
	mustChromecastCIDR("172.16.0.0/12"),
	mustChromecastCIDR("192.168.0.0/16"),
	mustChromecastCIDR("::/128"),
	mustChromecastCIDR("::1/128"),
	mustChromecastCIDR("fc00::/7"),
	mustChromecastCIDR("fe80::/10"),
}

// ChromecastSpeak generates TTS audio and plays it on a Chromecast device.
func ChromecastSpeak(deviceAddr string, devicePort int, text string, ttsCfg TTSConfig, ccCfg ChromecastConfig, logger *slog.Logger) string {
	if deviceAddr == "" {
		return jsonErr("'device_addr' is required")
	}
	if text == "" {
		return jsonErr("'text' is required")
	}
	if refused := chromecastMutationRefused(deviceAddr, ccCfg); refused != "" {
		return refused
	}
	if err := AuthorizeChromecastDial(deviceAddr, devicePort, ccCfg.KnownDevices); err != nil {
		return jsonErr("Failed to connect to Chromecast: " + err.Error())
	}
	host, err := chromecastReachableMediaHost(ccCfg.ServerHost)
	if err != nil {
		return jsonErr(err.Error())
	}

	if ttsCfg.Result == nil {
		ttsCfg.Result = &TTSSynthesisMeta{}
	}
	filename, err := synthesizeChromecastSpeech(ttsCfg, text)
	if err != nil {
		return jsonErr("TTS failed: " + err.Error())
	}
	contentType := "audio/mpeg"
	if mime, ok := CastMediaMIMEType(filename); ok {
		contentType = mime
	}
	audioURL := security.SignCastMediaURL(fmt.Sprintf("http://%s:%d/tts/%s", host, ccCfg.ServerPort, url.PathEscape(filename)), security.CastTTSTicketLifetime)
	if audioURL == "" {
		return jsonErr("create cast media ticket")
	}

	app, err := connectChromecastController(deviceAddr, devicePort, ccCfg)
	if err != nil {
		return jsonErr("Failed to connect to Chromecast: " + err.Error())
	}
	defer app.Close(false)

	if err := chromecastLoadAndConfirm(app, ccCfg.Context, audioURL, contentType); err != nil {
		return jsonErr("Failed to cast audio: " + err.Error())
	}

	spoken := map[string]interface{}{
		"message": fmt.Sprintf("Speaking on %s: %s", deviceAddr, text),
		"audio":   chromecastPublicMediaURL(audioURL),
	}
	if ttsCfg.Result.FallbackProvider != "" {
		spoken["fallback_provider"] = ttsCfg.Result.FallbackProvider
	}
	if ttsCfg.Result.Truncated {
		spoken["truncated"] = true
	}
	return jsonOK(spoken)
}

// ChromecastStop stops playback on a Chromecast device.
func ChromecastStop(deviceAddr string, devicePort int, ccCfg ChromecastConfig, logger *slog.Logger) string {
	if deviceAddr == "" {
		return jsonErr("'device_addr' is required")
	}
	if refused := chromecastMutationRefused(deviceAddr, ccCfg); refused != "" {
		return refused
	}
	app, err := connectChromecastController(deviceAddr, devicePort, ccCfg)
	if err != nil {
		return jsonErr("Failed to connect: " + err.Error())
	}
	defer app.Close(false)
	if err := app.StopMedia(); err != nil {
		return jsonErr("Failed to stop: " + err.Error())
	}
	return jsonOK(map[string]interface{}{"message": "Playback stopped"})
}

// ChromecastPause pauses playback on a Chromecast device.
func ChromecastPause(deviceAddr string, devicePort int, ccCfg ChromecastConfig, logger *slog.Logger) string {
	return chromecastSimpleMutation(deviceAddr, devicePort, ccCfg, "Failed to pause", "Playback paused", func(app castController) error {
		return app.Pause()
	})
}

// ChromecastResume resumes playback on a Chromecast device.
func ChromecastResume(deviceAddr string, devicePort int, ccCfg ChromecastConfig, logger *slog.Logger) string {
	return chromecastSimpleMutation(deviceAddr, devicePort, ccCfg, "Failed to resume", "Playback resumed", func(app castController) error {
		return app.Unpause()
	})
}

// ChromecastSeek seeks to position seconds. positionSet is required; 0 is the start.
func ChromecastSeek(deviceAddr string, devicePort int, position float64, positionSet bool, ccCfg ChromecastConfig, logger *slog.Logger) string {
	if !positionSet {
		return jsonErr("'position' is required")
	}
	if position < 0 {
		return jsonErr("'position' must be zero or greater")
	}
	return chromecastSimpleMutation(deviceAddr, devicePort, ccCfg, "Failed to seek", "Playback seeked", func(app castController) error {
		return app.SeekToTime(float32(position))
	})
}

// ChromecastMute sets the mute flag. mutedSet is required so an omitted flag is not silence.
func ChromecastMute(deviceAddr string, devicePort int, muted, mutedSet bool, ccCfg ChromecastConfig, logger *slog.Logger) string {
	if !mutedSet {
		return jsonErr("'muted' is required")
	}
	message := "Muted"
	if !muted {
		message = "Unmuted"
	}
	return chromecastSimpleMutation(deviceAddr, devicePort, ccCfg, "Failed to set mute", message, func(app castController) error {
		return app.SetMuted(muted)
	})
}

func chromecastSimpleMutation(deviceAddr string, devicePort int, ccCfg ChromecastConfig, failPrefix, okMessage string, fn func(castController) error) string {
	if deviceAddr == "" {
		return jsonErr("'device_addr' is required")
	}
	if refused := chromecastMutationRefused(deviceAddr, ccCfg); refused != "" {
		return refused
	}
	app, err := connectChromecastController(deviceAddr, devicePort, ccCfg)
	if err != nil {
		return jsonErr("Failed to connect: " + err.Error())
	}
	defer app.Close(false)
	if err := fn(app); err != nil {
		return jsonErr(failPrefix + ": " + err.Error())
	}
	return jsonOK(map[string]interface{}{"message": okMessage})
}

// ChromecastVolume sets volume on a Chromecast device (0.0–1.0).
// levelSet distinguishes an explicit 0 from a missing volume.
func ChromecastVolume(deviceAddr string, devicePort int, level float64, levelSet bool, ccCfg ChromecastConfig, logger *slog.Logger) string {
	if deviceAddr == "" {
		return jsonErr("'device_addr' is required")
	}
	if !levelSet {
		return jsonErr("'volume' is required")
	}
	if refused := chromecastMutationRefused(deviceAddr, ccCfg); refused != "" {
		return refused
	}
	app, err := connectChromecastController(deviceAddr, devicePort, ccCfg)
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
func ChromecastStatus(deviceAddr string, devicePort int, ccCfg ChromecastConfig, logger *slog.Logger) string {
	if deviceAddr == "" {
		return jsonErr("'device_addr' is required")
	}
	app, err := connectChromecastController(deviceAddr, devicePort, ccCfg)
	if err != nil {
		return jsonErr("Failed to connect: " + err.Error())
	}
	defer app.Close(false)

	castApp, media, castVol := app.Status()
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
	if media != nil {
		info["player_state"] = media.PlayerState
		info["position"] = media.CurrentTime
		info["duration"] = media.Media.Duration
		if title := strings.TrimSpace(media.Media.Metadata.Title); title != "" {
			info["title"] = title
		}
	}
	return jsonOK(info)
}

// connectChromecast creates a connection to a Chromecast device.
func connectChromecast(addr string, port int, ccCfg ChromecastConfig) (castController, error) {
	if port == 0 {
		port = 8009
	}
	if err := AuthorizeChromecastDial(addr, port, ccCfg.KnownDevices); err != nil {
		return nil, err
	}
	app := application.NewApplication(application.WithCacheDisabled(true), application.WithConnectionRetries(1))
	if err := app.Start(addr, port); err != nil {
		_ = app.Close(false)
		return nil, fmt.Errorf("failed to start connection: %w", err)
	}
	return app, nil
}

func chromecastTXTValue(txts []string, key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	for _, raw := range txts {
		parts := strings.SplitN(strings.TrimSpace(raw), "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.ToLower(strings.TrimSpace(parts[0])) != key {
			continue
		}
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func canonicalChromecastName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "'", "", "\"", "")
	return replacer.Replace(s)
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
