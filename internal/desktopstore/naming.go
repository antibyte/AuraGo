package desktopstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func DesktopAppID(appID string) string {
	return "store-" + normalizeAppID(appID)
}

func desktopAppIDForEntry(entry CatalogEntry) string {
	if strings.TrimSpace(entry.DesktopAppID) != "" {
		return strings.ToLower(strings.TrimSpace(entry.DesktopAppID))
	}
	return DesktopAppID(entry.ID)
}

// ContainerName returns the managed Docker container name for a store app.
func ContainerName(appID string) string {
	return "aurago-store-" + normalizeAppID(appID)
}

// NativeManagedContainerName returns the stable managed container name for a
// native Store runtime.
func NativeManagedContainerName(appID string) string {
	return "aurago-" + normalizeAppID(appID)
}

// CompanionContainerName returns the managed Docker container name for a Store
// app companion container.
func CompanionContainerName(appID, companionID string) string {
	return ContainerName(appID) + "-" + normalizeAppID(companionID)
}

// ManagedLaunchpadLinkID returns the stable Launchpad link ID for a store app.
func ManagedLaunchpadLinkID(appID string) string {
	return "store-" + normalizeAppID(appID)
}

func DefaultPortAllocator(ctx context.Context, preferred int) (int, error) {
	return allocateDynamicHostPort(ctx)
}

func allocateDynamicHostPort(ctx context.Context) (int, error) {
	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp4", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func portAvailable(ctx context.Context, port int) bool {
	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func hostWithoutPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.Contains(host, ":") {
		if parsed, _, err := net.SplitHostPort(host); err == nil {
			return parsed
		}
	}
	return host
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func newID(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}

func randomHex(size int) string {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ensureDir(dir string) error {
	return os.MkdirAll(filepath.Clean(dir), 0o755)
}

type missingDockerAdapter struct{}

func (missingDockerAdapter) PullImage(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) CreateContainer(context.Context, ContainerSpec) (string, error) {
	return "", fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) CopyToContainer(context.Context, string, string, map[string]string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) StartContainer(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) StopContainer(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) RestartContainer(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) RemoveContainer(context.Context, string, bool) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) RemoveVolume(context.Context, string, bool) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) CreateNetwork(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) RemoveNetwork(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) InspectContainer(context.Context, string) (ContainerState, error) {
	return ContainerState{}, fmt.Errorf("Docker adapter is not configured")
}
