// Package tools – port_scanner: TCP connect port scanner using stdlib.
package tools

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// portEntry represents one scanned port result.
type portEntry struct {
	Port    int    `json:"port"`
	Status  string `json:"status"` // "open" or "closed"
	Service string `json:"service,omitempty"`
	Banner  string `json:"banner,omitempty"`
}

// portScanResult is the JSON payload returned by ScanPorts.
type portScanResult struct {
	Status       string      `json:"status"`
	Host         string      `json:"host"`
	OpenPorts    []portEntry `json:"open_ports,omitempty"`
	ClosedCount  int         `json:"closed_count"`
	TotalScanned int         `json:"total_scanned"`
	Message      string      `json:"message,omitempty"`
}

// commonPorts is the top-100 most commonly used TCP ports.
var commonPorts = []int{
	21, 22, 23, 25, 53, 80, 110, 111, 135, 139,
	143, 443, 445, 465, 514, 587, 636, 993, 995, 1080,
	1433, 1434, 1521, 1723, 2049, 2082, 2083, 2086, 2087, 3000,
	3306, 3389, 4443, 5000, 5001, 5432, 5900, 5901, 6379, 6443,
	7000, 7443, 8000, 8006, 8008, 8080, 8081, 8082, 8083, 8088,
	8090, 8096, 8123, 8200, 8291, 8443, 8444, 8445, 8800, 8880,
	8888, 8889, 9000, 9001, 9090, 9091, 9100, 9200, 9443, 9993,
	10000, 11211, 15672, 18080, 19999, 25565, 27017, 28015, 32400, 51820,
}

// wellKnownServices maps port numbers to service names.
var wellKnownServices = map[int]string{
	21: "FTP", 22: "SSH", 23: "Telnet", 25: "SMTP", 53: "DNS",
	80: "HTTP", 110: "POP3", 111: "RPCBind", 135: "MSRPC", 139: "NetBIOS",
	143: "IMAP", 443: "HTTPS", 445: "SMB", 465: "SMTPS", 514: "Syslog",
	587: "Submission", 636: "LDAPS", 993: "IMAPS", 995: "POP3S",
	1433: "MSSQL", 1521: "Oracle", 1723: "PPTP", 2049: "NFS",
	3000: "Grafana", 3306: "MySQL", 3389: "RDP", 5000: "UPnP",
	5432: "PostgreSQL", 5900: "VNC", 6379: "Redis", 6443: "K8s-API",
	8006: "Proxmox", 8080: "HTTP-Alt", 8096: "Jellyfin", 8123: "HA",
	8200: "Vault", 8443: "HTTPS-Alt", 8888: "HTTP-Alt2", 9000: "Portainer",
	9090: "Prometheus", 9200: "Elasticsearch", 11211: "Memcached",
	15672: "RabbitMQ", 25565: "Minecraft", 27017: "MongoDB",
	32400: "Plex", 51820: "WireGuard",
}

// maxScanPorts is the maximum number of ports allowed per scan.
const maxScanPorts = 1024

// maxConcurrency controls the goroutine pool size for parallel scanning.
const maxConcurrency = 50

// ScanPorts performs a TCP connect scan on the target host.
// portRange can be: "80", "80,443,8080", "1-1024", or "common" (top well-known ports).
// timeoutMs is the per-port timeout in milliseconds (100–5000, default: 1000).
func ScanPorts(targetHost, portRange string, timeoutMs int) string {
	encode := func(r portScanResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if targetHost == "" {
		return encode(portScanResult{Status: "error", Message: "host is required"})
	}

	if timeoutMs <= 0 || timeoutMs > 5000 {
		timeoutMs = 1000
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond

	ports, err := parsePorts(portRange)
	if err != nil {
		return encode(portScanResult{Status: "error", Host: targetHost, Message: err.Error()})
	}
	if len(ports) > maxScanPorts {
		return encode(portScanResult{Status: "error", Host: targetHost, Message: fmt.Sprintf("too many ports: %d (max %d)", len(ports), maxScanPorts)})
	}

	// Concurrent scan with bounded goroutine pool
	type scanJob struct {
		port   int
		result portEntry
	}
	results := make([]scanJob, len(ports))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)

	for i, port := range ports {
		results[i] = scanJob{port: port}
		wg.Add(1)
		go func(idx, p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			addr := net.JoinHostPort(targetHost, fmt.Sprintf("%d", p))
			conn, err := net.DialTimeout("tcp", addr, timeout)
			entry := portEntry{Port: p}
			if err != nil {
				entry.Status = "closed"
			} else {
				entry.Status = "open"
				if svc, ok := wellKnownServices[p]; ok {
					entry.Service = svc
				}
				// Attempt banner grab
				_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				buf := make([]byte, 256)
				n, _ := conn.Read(buf)
				if n > 0 {
					banner := strings.TrimSpace(string(buf[:n]))
					// Sanitise: replace control chars
					banner = strings.Map(func(r rune) rune {
						if r < 32 && r != '\n' && r != '\r' {
							return '.'
						}
						return r
					}, banner)
					if len(banner) > 128 {
						banner = banner[:128]
					}
					entry.Banner = banner
				}
				conn.Close()
			}
			results[idx] = scanJob{port: p, result: entry}
		}(i, port)
	}
	wg.Wait()

	var openPorts []portEntry
	closedCount := 0
	for _, r := range results {
		if r.result.Status == "open" {
			openPorts = append(openPorts, r.result)
		} else {
			closedCount++
		}
	}
	sort.Slice(openPorts, func(i, j int) bool { return openPorts[i].Port < openPorts[j].Port })

	return encode(portScanResult{
		Status:       "success",
		Host:         targetHost,
		OpenPorts:    openPorts,
		ClosedCount:  closedCount,
		TotalScanned: len(ports),
	})
}

// parsePorts converts a port range string into a sorted slice of port numbers.
func parsePorts(portRange string) ([]int, error) {
	portRange = strings.TrimSpace(strings.ToLower(portRange))
	if portRange == "" || portRange == "common" {
		result := make([]int, len(commonPorts))
		copy(result, commonPorts)
		return result, nil
	}

	portSet := make(map[int]bool)
	parts := strings.Split(portRange, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid port range start: %s", bounds[0])
			}
			end, err := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid port range end: %s", bounds[1])
			}
			if start < 1 || end > 65535 || start > end {
				return nil, fmt.Errorf("invalid port range: %d-%d", start, end)
			}
			for p := start; p <= end; p++ {
				portSet[p] = true
			}
		} else {
			p, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port number: %s", part)
			}
			if p < 1 || p > 65535 {
				return nil, fmt.Errorf("port out of range: %d", p)
			}
			portSet[p] = true
		}
	}

	result := make([]int, 0, len(portSet))
	for p := range portSet {
		result = append(result, p)
	}
	sort.Ints(result)
	return result, nil
}
