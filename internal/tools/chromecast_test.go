package tools

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestDiscoverChromecastDevicesParsesFriendlyName(t *testing.T) {
	orig := chromecastMDNSQuery
	chromecastMDNSQuery = func(serviceType string, timeout time.Duration, logger *slog.Logger) ([]*mdnsEntry, error) {
		return []*mdnsEntry{
			{
				Name: "Google-Home-Mini-b39e08d8ca5bd6baa7ed277fd1bb1437._googlecast._tcp.local.",
				IPs:  []string{"192.168.6.130"},
				Port: 8009,
				TXTs: []string{"fn=Arbeitszimmer", "md=Google Home Mini"},
			},
		}, nil
	}
	defer func() {
		chromecastMDNSQuery = orig
	}()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	devices, err := DiscoverChromecastDevices(logger)
	if err != nil {
		t.Fatalf("DiscoverChromecastDevices returned error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].FriendlyName != "Arbeitszimmer" {
		t.Fatalf("FriendlyName = %q, want %q", devices[0].FriendlyName, "Arbeitszimmer")
	}
	if devices[0].Name != "Google-Home-Mini-b39e08d8ca5bd6baa7ed277fd1bb1437" {
		t.Fatalf("Name = %q", devices[0].Name)
	}
}

func TestFindChromecastDeviceByNameMatchesFriendlyName(t *testing.T) {
	devices := []ChromecastDevice{
		{
			Name:         "Google-Home-Mini-b39e08d8ca5bd6baa7ed277fd1bb1437",
			FriendlyName: "Arbeitszimmer",
			Addr:         "192.168.6.130",
			Port:         8009,
		},
	}

	device, ok := FindChromecastDeviceByName(devices, "Arbeitszimmer")
	if !ok {
		t.Fatal("expected friendly-name match")
	}
	if device.Addr != "192.168.6.130" {
		t.Fatalf("Addr = %q, want 192.168.6.130", device.Addr)
	}
}
