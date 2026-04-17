// Package tools – mdns_avahi: delegate mDNS discovery to avahi-browse when
// available. On Linux hosts running avahi-daemon, the daemon consumes a
// fraction of mDNS packets on port 5353 via SO_REUSEPORT load-balancing,
// making an independent Go-based listener unreliable. avahi-browse queries
// the Avahi cache via D-Bus and returns complete, immediate results.
package tools

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// avahiBrowseAvailable returns true if the avahi-browse binary exists in PATH.
func avahiBrowseAvailable() bool {
	_, err := exec.LookPath("avahi-browse")
	return err == nil
}

// mdnsQueryViaAvahi runs `avahi-browse -rtp <serviceType>` and parses the
// machine-readable output. Returns entries compatible with mdnsQueryServices.
//
// Output format (one record per line, semicolon-separated):
//
//	+;iface;proto;name;type;domain
//	=;iface;proto;name;type;domain;host;ip;port;"txt1" "txt2" ...
//
// Only lines starting with "=" contain resolved details.
func mdnsQueryViaAvahi(serviceType string, timeout time.Duration, logger *slog.Logger) ([]*mdnsEntry, error) {
	// Ensure service type is in the form "_googlecast._tcp" (no trailing dot, no .local.)
	st := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(serviceType, "."), ".local"), ".")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// -r = resolve, -t = terminate after cache exhausted, -p = parseable
	cmd := exec.CommandContext(ctx, "avahi-browse", "-rtp", st)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("avahi-browse pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("avahi-browse start: %w", err)
	}

	logger.Info("mdns: using avahi-browse for discovery", "service", st)

	// De-duplicate entries across IPv4/IPv6 and interface re-announcements.
	// Key: name (unique per device registration)
	entries := make(map[string]*mdnsEntry)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		if !strings.HasPrefix(line, "=") {
			continue
		}
		fields := strings.Split(line, ";")
		// Expected 10 fields: =;iface;proto;name;type;domain;host;ip;port;txt
		if len(fields) < 10 {
			continue
		}
		proto := fields[2]
		name := fields[3]
		svcType := fields[4]
		domain := fields[5]
		host := fields[6]
		ip := fields[7]
		portStr := fields[8]
		txt := fields[9]

		// Only IPv4 for chromecast compatibility (vishen/go-chromecast uses IPv4).
		// Also accept IPv6 as secondary but prefer IPv4 in the output.
		if ip == "" {
			continue
		}

		port, _ := strconv.Atoi(portStr)

		// Build FQDN name matching the native scanner's format:
		//   "Google-Home-Mini-xxxx._googlecast._tcp.local."
		fqdnName := name + "." + svcType + "." + domain + "."

		e, ok := entries[fqdnName]
		if !ok {
			e = &mdnsEntry{
				Name: fqdnName,
				Host: host + ".",
				Port: port,
			}
			entries[fqdnName] = e
		}
		// Prefer IPv4 as the first IP.
		if proto == "IPv4" {
			// Put IPv4 first so e.IPs[0] is the usable address.
			e.IPs = append([]string{ip}, e.IPs...)
		} else {
			e.IPs = append(e.IPs, ip)
		}

		// Parse TXT records (space-separated quoted strings).
		if txt != "" && len(e.TXTs) == 0 {
			e.TXTs = parseAvahiTXT(txt)
		}
		logger.Info("mdns: avahi entry", "name", fqdnName, "ip", ip, "proto", proto, "port", port)
	}

	// Wait for avahi-browse to exit (it will on -t after cache drain, or on ctx timeout).
	waitErr := cmd.Wait()
	if waitErr != nil && ctx.Err() == nil {
		// Non-timeout error — but we may still have valid partial results.
		logger.Warn("mdns: avahi-browse exited with error", "error", waitErr, "lines", lineCount)
	}

	result := make([]*mdnsEntry, 0, len(entries))
	for _, e := range entries {
		// De-duplicate IPs (might happen if same IP came on multiple interfaces)
		e.IPs = dedupStrings(e.IPs)
		result = append(result, e)
	}
	logger.Info("mdns: avahi-browse discovery complete", "entries_found", len(result), "lines_parsed", lineCount)
	return result, nil
}

// parseAvahiTXT splits a string of quoted TXT records, e.g.
// `"key1=value1" "key2=value2"` into ["key1=value1", "key2=value2"].
func parseAvahiTXT(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' && !inQuote:
			inQuote = true
		case c == '"' && inQuote:
			inQuote = false
			out = append(out, cur.String())
			cur.Reset()
		case inQuote:
			cur.WriteByte(c)
		}
	}
	return out
}

func dedupStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
