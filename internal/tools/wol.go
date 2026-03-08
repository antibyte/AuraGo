package tools

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// SendWakeOnLAN sends a Wake-on-LAN magic packet to the given MAC address.
// The broadcast address defaults to 255.255.255.255:9 (port 9 is the discard port, standard for WOL).
// An optional target broadcast IP can be provided (e.g. "192.168.1.255") to limit the
// broadcast to a specific subnet.
func SendWakeOnLAN(macStr string, broadcastIP string) error {
	mac, err := parseMACAddress(macStr)
	if err != nil {
		return fmt.Errorf("invalid MAC address %q: %w", macStr, err)
	}

	packet := buildMagicPacket(mac)

	bcastAddr := "255.255.255.255"
	if broadcastIP != "" {
		bcastAddr = broadcastIP
	}

	conn, err := net.Dial("udp", bcastAddr+":9")
	if err != nil {
		return fmt.Errorf("failed to open UDP socket: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write(packet); err != nil {
		return fmt.Errorf("failed to send magic packet: %w", err)
	}

	return nil
}

// parseMACAddress normalises a MAC string (e.g. "AA:BB:CC:DD:EE:FF" or "aa-bb-cc-dd-ee-ff")
// and returns the raw 6 bytes.
func parseMACAddress(s string) ([]byte, error) {
	// Normalise separators
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, ":", "")
	s = strings.TrimSpace(s)

	if len(s) != 12 {
		return nil, fmt.Errorf("MAC address must be 12 hex digits, got %d", len(s))
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("hex decode error: %w", err)
	}

	return b, nil
}

// buildMagicPacket constructs the 102-byte WOL magic packet:
// 6 × 0xFF followed by 16 repetitions of the 6-byte MAC address.
func buildMagicPacket(mac []byte) []byte {
	packet := make([]byte, 0, 102)

	// Header: 6 bytes of 0xFF
	for i := 0; i < 6; i++ {
		packet = append(packet, 0xFF)
	}

	// Payload: MAC repeated 16 times
	for i := 0; i < 16; i++ {
		packet = append(packet, mac...)
	}

	return packet
}
