package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// MACLookupResult holds the result of a MAC address lookup.
type MACLookupResult struct {
	Status    string `json:"status"`
	IPAddress string `json:"ip_address,omitempty"`
	MAC       string `json:"mac_address,omitempty"`
	Source    string `json:"source,omitempty"`
	Message   string `json:"message,omitempty"`
}

// LookupMACAddress looks up the MAC address for a given IP using the OS ARP table.
// It works without elevated privileges on all supported platforms.
//
//   - Linux: reads /proc/net/arp
//   - Windows / macOS: parses `arp -a <ip>` output
//
// Returns a JSON string. Status is "success", "not_found", or "error".
func LookupMACAddress(ip, _ string) string {
	// Validate IP address to prevent command injection.
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return jsonMAC(MACLookupResult{Status: "error", Message: "invalid IP address"})
	}
	ip = parsed.String() // normalised form

	switch runtime.GOOS {
	case "linux":
		return lookupLinux(ip)
	default:
		return lookupArpCommand(ip)
	}
}

// lookupLinux reads /proc/net/arp directly (no subprocess, no root).
func lookupLinux(ip string) string {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		// Fall back to arp command if /proc is unavailable (e.g. some containers).
		return lookupArpCommand(ip)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Skip header line: "IP address       HW type  Flags       HW address            Mask     Device"
	scanner.Scan()
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		entryIP := fields[0]
		flags := fields[2]
		mac := fields[3]

		if entryIP != ip {
			continue
		}
		// Flags 0x0 means incomplete (no reply yet); skip incomplete entries.
		if flags == "0x0" || mac == "00:00:00:00:00:00" {
			continue
		}
		return jsonMAC(MACLookupResult{
			Status:    "success",
			IPAddress: ip,
			MAC:       strings.ToUpper(mac),
			Source:    "proc_net_arp",
		})
	}
	return jsonMAC(MACLookupResult{
		Status:    "not_found",
		IPAddress: ip,
		Message:   fmt.Sprintf("IP %s not found in ARP cache. Make sure the device is reachable on the local network.", ip),
	})
}

// lookupArpCommand parses `arp -a <ip>` on Windows/macOS.
func lookupArpCommand(ip string) string {
	// ip is already validated by net.ParseIP, but use it as-is for the argument.
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("arp", "-a", ip)
	case "darwin":
		cmd = exec.Command("arp", ip)
	default:
		cmd = exec.Command("arp", "-a", ip)
	}

	out, err := cmd.Output()
	if err != nil {
		return jsonMAC(MACLookupResult{
			Status:    "not_found",
			IPAddress: ip,
			Message:   fmt.Sprintf("ARP lookup failed: %v", err),
		})
	}

	mac := parseARPOutput(string(out), ip)
	if mac == "" {
		return jsonMAC(MACLookupResult{
			Status:    "not_found",
			IPAddress: ip,
			Message:   fmt.Sprintf("IP %s not found in ARP cache.", ip),
		})
	}
	return jsonMAC(MACLookupResult{
		Status:    "success",
		IPAddress: ip,
		MAC:       strings.ToUpper(mac),
		Source:    "arp_command",
	})
}

// macPattern matches common MAC address formats:
//   - AA:BB:CC:DD:EE:FF  (Linux/macOS colon-separated)
//   - aa-bb-cc-dd-ee-ff  (Windows hyphen-separated)
var macPattern = regexp.MustCompile(`(?i)([0-9a-f]{2}[:\-][0-9a-f]{2}[:\-][0-9a-f]{2}[:\-][0-9a-f]{2}[:\-][0-9a-f]{2}[:\-][0-9a-f]{2})`)

func parseARPOutput(output, targetIP string) string {
	for _, line := range strings.Split(output, "\n") {
		// Check if this line mentions the target IP.
		if !strings.Contains(line, targetIP) {
			continue
		}
		m := macPattern.FindString(line)
		if m != "" {
			// Normalise separators to colons.
			return strings.ReplaceAll(m, "-", ":")
		}
	}
	return ""
}

func jsonMAC(r MACLookupResult) string {
	b, _ := json.Marshal(r)
	return string(b)
}
