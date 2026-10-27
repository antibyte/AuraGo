package tools

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vishen/go-chromecast/cast"
)

const (
	// CastMediaMaxBytes is the largest local file copied into cast staging.
	CastMediaMaxBytes = 1 << 30
	// CastMediaRetention is how long a staged cast file may remain on disk.
	CastMediaRetention = 3 * time.Hour
)

const (
	chromecastDiscoveryCacheTTL = 30 * time.Second
	chromecastCapVideoOut       = 1
	chromecastCapAudioOut       = 4
	chromecastCapMultizoneGroup = 32
)

var (
	chromecastPlaybackConfirmTimeout = 4 * time.Second
	chromecastPlaybackPollInterval   = 250 * time.Millisecond
)

// castController is the Cast receiver surface AuraGo uses. Tests replace the dial seam.
type castController interface {
	Load(filenameOrUrl string, startTime int, contentType string, transcode, detach, forceDetach bool) error
	Pause() error
	Unpause() error
	SeekToTime(value float32) error
	SetVolume(value float32) error
	SetMuted(value bool) error
	StopMedia() error
	Status() (*cast.Application, *cast.Media, *cast.Volume)
	Update() error
	Close(stopMedia bool) error
}

var connectChromecastController = connectChromecast
var synthesizeChromecastSpeech = TTSSynthesize

// ChromecastAdvertiseHost returns the host a Cast device uses to fetch AuraGo media.
// An empty explicit host uses fallback. Loopback, unspecified, and empty results are errors.
func ChromecastAdvertiseHost(explicit, fallback string) (string, error) {
	host := strings.TrimSpace(explicit)
	if host == "" {
		host = strings.TrimSpace(fallback)
	}
	if chromecastNeedsLANReachableHost(host) {
		return "", fmt.Errorf("chromecast advertise host %q is not reachable from a Cast device", host)
	}
	return host, nil
}

// AuthorizeChromecastDial allows a dial only to a known private unicast address.
// Port 0 means 8009. The dial may use 8009 or the device's stored port.
// A stored port of 0 means 8009. Stored port 22 is not rewritten to 8009.
func AuthorizeChromecastDial(addr string, port int, known []ChromecastDevice) error {
	ip := net.ParseIP(strings.TrimSpace(addr))
	if ip == nil || !chromecastPrivateUnicast(ip) {
		return fmt.Errorf("chromecast address %q is not a known private device", strings.TrimSpace(addr))
	}
	if port == 0 {
		port = 8009
	}
	matched, ok := chromecastKnownByIP(ip, known)
	if !ok {
		return fmt.Errorf("chromecast address %q is not a known device", ip.String())
	}
	stored := matched.Port
	if stored == 0 {
		stored = 8009
	}
	if port != 8009 && port != stored {
		return fmt.Errorf("chromecast port %d is not allowed for %s", port, ip.String())
	}
	return nil
}

func chromecastPrivateUnicast(ip net.IP) bool {
	if ip == nil || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	return chromecastInternalIP(ip) && !chromecastForbiddenIP(ip)
}

func chromecastKnownByIP(ip net.IP, known []ChromecastDevice) (ChromecastDevice, bool) {
	if ip == nil {
		return ChromecastDevice{}, false
	}
	for _, device := range known {
		candidate := net.ParseIP(strings.TrimSpace(device.Addr))
		if candidate != nil && candidate.Equal(ip) {
			return device, true
		}
	}
	return ChromecastDevice{}, false
}

func chromecastKnownGroup(addr string, known []ChromecastDevice) bool {
	ip := net.ParseIP(strings.TrimSpace(addr))
	device, ok := chromecastKnownByIP(ip, known)
	return ok && device.Group
}

func chromecastMutationRefused(deviceAddr string, ccCfg ChromecastConfig) string {
	if ccCfg.ReadOnly {
		return jsonErr("Chromecast is read-only")
	}
	if chromecastKnownGroup(deviceAddr, ccCfg.KnownDevices) {
		return jsonErr("Chromecast groups are labeled and not controllable")
	}
	return ""
}

func chromecastReachableMediaHost(host string) (string, error) {
	host = strings.TrimSpace(host)
	if chromecastNeedsLANReachableHost(host) {
		return "", fmt.Errorf("chromecast media host %q is not reachable from a Cast device", host)
	}
	return host, nil
}

// chromecastPublicMediaURL drops the ticket query and fragment from a model-facing URL.
func chromecastPublicMediaURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// The receiver keeps the previous session until LOAD replaces it.
func chromecastLoadAndConfirm(app castController, ctx context.Context, mediaURL, contentType string) error {
	beforeSession, beforeContent := chromecastMediaIdentity(app)
	if err := app.Load(mediaURL, 0, contentType, false, false, true); err != nil {
		return err
	}
	return confirmChromecastPlayback(ctx, app, mediaURL, beforeSession, beforeContent)
}

func chromecastMediaIdentity(app castController) (int, string) {
	if app == nil {
		return 0, ""
	}
	_, media, _ := app.Status()
	if media == nil {
		return 0, ""
	}
	return media.MediaSessionId, strings.TrimSpace(media.Media.ContentId)
}

func chromecastPlaybackBelongsToLoad(media *cast.Media, mediaURL string, beforeSession int, beforeContent string) bool {
	if media == nil {
		return false
	}
	content := strings.TrimSpace(media.Media.ContentId)
	if media.MediaSessionId != 0 && media.MediaSessionId != beforeSession {
		return true
	}
	return content != "" && content == mediaURL && content != strings.TrimSpace(beforeContent)
}

func confirmChromecastPlayback(ctx context.Context, app castController, mediaURL string, beforeSession int, beforeContent string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	deadline := time.Now().Add(chromecastPlaybackConfirmTimeout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := app.Update(); err != nil {
			return fmt.Errorf("playback status: %w", err)
		}
		_, media, _ := app.Status()
		if chromecastPlaybackBelongsToLoad(media, mediaURL, beforeSession, beforeContent) {
			state := strings.ToUpper(strings.TrimSpace(media.PlayerState))
			idle := strings.ToUpper(strings.TrimSpace(media.IdleReason))
			switch state {
			case "PLAYING", "BUFFERING", "PAUSED":
				return nil
			case "IDLE":
				switch idle {
				case "FINISHED":
					return nil
				case "ERROR":
					return fmt.Errorf("playback rejected: IDLE %s", idle)
				case "CANCELLED", "CANCELED":
					return fmt.Errorf("playback rejected: IDLE %s", idle)
				}
			}
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("playback was not confirmed")
		}
		timer := time.NewTimer(chromecastPlaybackPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

type chromecastDiscoveryCache struct {
	mu      sync.Mutex
	at      time.Time
	devices []ChromecastDevice
	err     error
	valid   bool
}

var chromecastDiscoveryCacheState chromecastDiscoveryCache

// ResetChromecastDiscoveryCache drops the 30-second discovery cache.
func ResetChromecastDiscoveryCache() {
	chromecastDiscoveryCacheState.mu.Lock()
	chromecastDiscoveryCacheState.at = time.Time{}
	chromecastDiscoveryCacheState.devices = nil
	chromecastDiscoveryCacheState.err = nil
	chromecastDiscoveryCacheState.valid = false
	chromecastDiscoveryCacheState.mu.Unlock()
}

// CachedDiscoverChromecastDevices returns a copy of the last discovery for 30 seconds.
// Errors are cached too. DiscoverChromecastDevices itself stays uncached.
func CachedDiscoverChromecastDevices(logger *slog.Logger) ([]ChromecastDevice, error) {
	chromecastDiscoveryCacheState.mu.Lock()
	defer chromecastDiscoveryCacheState.mu.Unlock()
	if chromecastDiscoveryCacheState.valid && time.Since(chromecastDiscoveryCacheState.at) < chromecastDiscoveryCacheTTL {
		return append([]ChromecastDevice(nil), chromecastDiscoveryCacheState.devices...), chromecastDiscoveryCacheState.err
	}
	devices, err := DiscoverChromecastDevices(logger)
	copied := append([]ChromecastDevice(nil), devices...)
	chromecastDiscoveryCacheState.devices = append([]ChromecastDevice(nil), copied...)
	chromecastDiscoveryCacheState.err = err
	chromecastDiscoveryCacheState.at = time.Now()
	chromecastDiscoveryCacheState.valid = true
	return append([]ChromecastDevice(nil), copied...), err
}

func chromecastPreferredIPv4(ips []string) string {
	fallback := ""
	for _, raw := range ips {
		ip := net.ParseIP(strings.TrimSpace(raw))
		if ip == nil {
			continue
		}
		ip4 := ip.To4()
		if ip4 == nil || ip4.IsLoopback() || ip4.IsLinkLocalUnicast() {
			continue
		}
		if chromecastPrivateUnicast(ip4) {
			return ip4.String()
		}
		if fallback == "" {
			fallback = ip4.String()
		}
	}
	return fallback
}

func chromecastCapabilityBits(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	bits, err := strconv.Atoi(raw)
	if err != nil || bits < 0 {
		return 0
	}
	return bits
}

func chromecastDeviceLabels(model, capabilities string) (group, audioOnly bool) {
	model = strings.ToLower(model)
	bits := chromecastCapabilityBits(capabilities)
	group = strings.Contains(model, "group") || bits&chromecastCapMultizoneGroup != 0
	audioOnly = (!group && strings.Contains(model, "audio")) || (bits&chromecastCapAudioOut != 0 && bits&chromecastCapVideoOut == 0)
	return group, audioOnly
}

// PruneExpiredCastMedia removes staged cast files older than CastMediaRetention.
func PruneExpiredCastMedia(dir string, now time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var first error
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		if now.Sub(info.ModTime()) <= CastMediaRetention {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && first == nil {
			first = err
		}
	}
	return first
}
