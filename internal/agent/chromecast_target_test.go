package agent

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/inventory"
	"aurago/internal/security"
	"aurago/internal/tools"
	"net/url"
)

func TestMergeChromecastKnownDevicesKeepsInventoryPortAndGroupLabel(t *testing.T) {
	merged := mergeChromecastKnownDevices(
		[]inventory.DeviceRecord{{Name: "Wohnzimmer", IPAddress: "192.168.1.20", Port: 22}},
		[]tools.ChromecastDevice{{
			Addr:         "192.168.1.20",
			Port:         8009,
			Group:        true,
			FriendlyName: "Wohnzimmer",
			UUID:         "cast-id",
		}, {
			Addr: "not-an-ip",
			Port: 8009,
		}},
	)
	if len(merged) != 1 {
		t.Fatalf("merged = %+v", merged)
	}
	if merged[0].Port != 22 || !merged[0].Group || merged[0].UUID != "cast-id" || merged[0].Name != "Wohnzimmer" {
		t.Fatalf("device = %+v", merged[0])
	}
}

func TestPrepareChromecastLocalMediaURLUsesAdvertiseHostAndRejectsLoopback(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, "agent_workspace", "workdir")
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workdir, "song.mp3"), []byte("fake mp3"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workdir
	cfg.Directories.DataDir = dataDir
	cfg.Server.Host = "127.0.0.1"
	cfg.Chromecast.TTSPort = 8090
	cfg.Chromecast.AdvertiseHost = "10.1.2.3"

	req := chromecastArgs{LocalPath: "workdir/song.mp3"}
	if err := prepareChromecastLocalMediaURL(cfg, &req); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Host != "10.1.2.3:8090" || !security.ValidCastMediaTicket(parsed, time.Now()) {
		t.Fatalf("URL = %q", req.URL)
	}

	cfg.Chromecast.AdvertiseHost = ""
	localIPCache.Delete("127.0.0.1")
	origDial := localIPDial
	localIPDial = func(network, address string) (net.Conn, error) {
		return nil, fmt.Errorf("no route")
	}
	defer func() {
		localIPDial = origDial
		localIPCache.Delete("127.0.0.1")
	}()
	loopback := chromecastArgs{LocalPath: "workdir/song.mp3"}
	if err := prepareChromecastLocalMediaURL(cfg, &loopback); err == nil {
		t.Fatal("expected loopback advertise host to fail")
	}
	if loopback.URL != "" {
		t.Fatalf("published loopback URL %q", loopback.URL)
	}
}

func TestPrepareChromecastLocalMediaURLRejectsOversizedFile(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, "agent_workspace", "workdir")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(workdir, "huge.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(tools.CastMediaMaxBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	file.Close()

	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workdir
	cfg.Directories.DataDir = filepath.Join(root, "data")
	cfg.Server.Host = "192.168.6.238"
	cfg.Chromecast.TTSPort = 8090
	req := chromecastArgs{LocalPath: "workdir/huge.mp3"}
	err = prepareChromecastLocalMediaURL(cfg, &req)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatal(err)
	}
}
