package server

import (
	"bufio"
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/inventory"
)

type quickConnectDevice struct {
	inventory.DeviceRecord
	QuickConnect bool `json:"quick_connect"`
	// FileAccess is set for RDP devices only (Files tab over SFTP or SMB).
	FileAccess *inventory.FileAccess `json:"file_access,omitempty"`
}

// quickConnectDevices retains configured connections, including offline hosts.
// Legacy discovery rows need a protocol banner: their stored SSH flag may have
// come from the old importer rather than an actual remote-access service.
func quickConnectDevices(ctx context.Context, devices []inventory.DeviceRecord) []quickConnectDevice {
	result := make([]quickConnectDevice, len(devices))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(16, len(devices)) {
		workers.Go(func() {
			for index := range jobs {
				result[index].QuickConnect = quickConnectProtocolAvailable(ctx, devices[index])
			}
		})
	}
	for index, device := range devices {
		result[index].DeviceRecord = device
		if device.Protocol != inventory.ProtocolSSH && device.Protocol != inventory.ProtocolVNC && device.Protocol != inventory.ProtocolRDP {
			continue
		}
		if strings.TrimSpace(device.Username) != "" || device.CredentialID != "" || device.VaultSecretID != "" {
			result[index].QuickConnect = true
			continue
		}
		select {
		case jobs <- index:
		case <-ctx.Done():
		}
	}
	close(jobs)
	workers.Wait()
	return result
}

// Read only the server's greeting; never authenticate, send commands or use Vault
// secrets. An open TCP port alone does not establish SSH/VNC support.
func quickConnectProtocolAvailable(ctx context.Context, device inventory.DeviceRecord) bool {
	host := strings.TrimSpace(device.IPAddress)
	port := device.Port
	if port <= 0 {
		port = 22
		if device.Protocol == inventory.ProtocolVNC {
			port = 5900
		}
		if device.Protocol == inventory.ProtocolRDP {
			port = 3389
		}
	}
	if host == "" || port > 65535 || ctx.Err() != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetReadDeadline(deadline); err != nil {
		return false
	}
	if device.Protocol == inventory.ProtocolRDP {
		return quickConnectRDPProbe(conn)
	}
	if device.Protocol == inventory.ProtocolVNC {
		var banner [12]byte
		if _, err := io.ReadFull(conn, banner[:]); err != nil {
			return false
		}
		if string(banner[:4]) != "RFB " || banner[7] != '.' || banner[11] != '\n' {
			return false
		}
		for _, index := range []int{4, 5, 6, 8, 9, 10} {
			if banner[index] < '0' || banner[index] > '9' {
				return false
			}
		}
		return true
	}
	reader := bufio.NewReader(io.LimitReader(conn, 4096))
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return false
		}
		if strings.HasPrefix(line, "SSH-") {
			return len(line) <= 255 && (strings.HasPrefix(line, "SSH-2.0-") || strings.HasPrefix(line, "SSH-1.99-"))
		}
	}
}

// rdpProbeRequest is an X.224 Connection Request carrying RDP_NEG_REQ
// (TLS|CredSSP). It authenticates nothing.
var rdpProbeRequest = []byte{0x03, 0x00, 0x00, 0x13, 0x0e, 0xe0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x08, 0x00, 0x03, 0x00, 0x00, 0x00}

// quickConnectRDPProbe sends the minimal X.224 request and accepts a TPKT reply
// whose X.224 code is Connection Confirm (0xD0).
func quickConnectRDPProbe(conn net.Conn) bool {
	if _, err := conn.Write(rdpProbeRequest); err != nil {
		return false
	}
	var reply [6]byte
	if _, err := io.ReadFull(conn, reply[:]); err != nil {
		return false
	}
	return reply[0] == 0x03 && reply[1] == 0x00 && reply[5] == 0xD0
}
