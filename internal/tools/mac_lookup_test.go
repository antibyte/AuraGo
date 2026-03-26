package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLookupMACAddress_InvalidIP(t *testing.T) {
	result := LookupMACAddress("not-an-ip", "")
	var r MACLookupResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected status=error for invalid IP, got %q", r.Status)
	}
}

func TestLookupMACAddress_ReturnsJSON(t *testing.T) {
	// Use a loopback or clearly non-routable IP. The result should always be valid JSON
	// regardless of whether the ARP cache has an entry or not.
	result := LookupMACAddress("192.168.255.254", "")
	if !strings.HasPrefix(result, "{") {
		t.Fatalf("expected JSON, got: %s", result)
	}
	var r MACLookupResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("bad JSON for valid IP: %v", err)
	}
	// Status must be one of the three defined values.
	switch r.Status {
	case "success", "not_found", "error":
	default:
		t.Errorf("unexpected status %q", r.Status)
	}
}

func TestParseARPOutput_Linux(t *testing.T) {
	output := `192.168.1.1      0x1  0x2  aa:bb:cc:dd:ee:ff  *  eth0`
	mac := parseARPOutput(output, "192.168.1.1")
	if strings.ToUpper(mac) != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected AA:BB:CC:DD:EE:FF, got %q", mac)
	}
}

func TestParseARPOutput_Windows(t *testing.T) {
	output := `
Interface: 192.168.1.10 --- 0x5
  Internet Address      Physical Address      Type
  192.168.1.1           aa-bb-cc-dd-ee-ff     dynamic
  192.168.1.20          11-22-33-44-55-66     static
`
	mac := parseARPOutput(output, "192.168.1.1")
	if strings.ToUpper(mac) != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected AA:BB:CC:DD:EE:FF, got %q", mac)
	}
}

func TestParseARPOutput_NotFound(t *testing.T) {
	output := `192.168.1.1      0x1  0x2  aa:bb:cc:dd:ee:ff  *  eth0`
	mac := parseARPOutput(output, "192.168.1.99")
	if mac != "" {
		t.Errorf("expected empty string for missing IP, got %q", mac)
	}
}

func TestParseARPOutput_macOS(t *testing.T) {
	output := "192.168.1.1 (192.168.1.1) at aa:bb:cc:dd:ee:ff on en0 ifscope [ethernet]"
	mac := parseARPOutput(output, "192.168.1.1")
	if strings.ToUpper(mac) != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected AA:BB:CC:DD:EE:FF, got %q", mac)
	}
}
