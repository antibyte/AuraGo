package acestep

import (
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestVulkanOptionAndDeviceConfinement(t *testing.T) {
	if err := (config.LocalMusicConfig{Backend: "vulkan"}).Validate(); err != nil {
		t.Fatal(err)
	}
	image := "ghcr.io/antibyte/aurago-acestep-vulkan@sha256:" + strings.Repeat("a", 64)
	if !imagePattern.MatchString(image) || imagePattern.MatchString(strings.Replace(image, "ghcr.io", "evil.test", 1)) {
		t.Fatal("Vulkan digest allowlist is incorrect")
	}
	host := baseHostConfig()
	device := Device{ID: "vulkan:1", Backend: "vulkan", FreeGB: 10, Verified: true,
		RenderNodes: []string{"/dev/dri/renderD129"}, Groups: []string{"993", "render", "0"}}
	if err := addGPU(host, "vulkan", &device); err != nil {
		t.Fatal(err)
	}
	paths := host["Devices"].([]map[string]string)
	if len(paths) != 1 || paths[0]["PathOnHost"] != "/dev/dri/renderD129" || host["DeviceRequests"] != nil {
		t.Fatalf("excessive Vulkan device access: %v", host)
	}
	if selected, err := selectDevice([]Device{device}, "vulkan:1"); err != nil || selected.ID != device.ID {
		t.Fatalf("manual Vulkan selection failed: %+v %v", selected, err)
	}
	device.Verified = false
	if _, err := selectDevice([]Device{device}, "auto"); err == nil {
		t.Fatal("unverified Vulkan GPU accepted")
	}
}
