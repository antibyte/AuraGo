package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

type whoisResult struct {
	Status     string            `json:"status"`
	Message    string            `json:"message,omitempty"`
	Domain     string            `json:"domain,omitempty"`
	Registrar  string            `json:"registrar,omitempty"`
	Created    string            `json:"created,omitempty"`
	Expires    string            `json:"expires,omitempty"`
	Updated    string            `json:"updated,omitempty"`
	Status_    []string          `json:"domain_status,omitempty"`
	NameServer []string          `json:"name_servers,omitempty"`
	DNSSEC     string            `json:"dnssec,omitempty"`
	Raw        string            `json:"raw,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

// WhoisLookup performs a WHOIS query for the given domain.
func WhoisLookup(domain string, includeRaw bool) string {
	if domain == "" {
		return whoisJSON(whoisResult{Status: "error", Message: "domain is required"})
	}

	// Normalize domain: strip protocol, path, port
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	if idx := strings.IndexAny(domain, "/:"); idx != -1 {
		domain = domain[:idx]
	}
	domain = strings.ToLower(domain)

	// Determine WHOIS server based on TLD
	server := getWhoisServer(domain)

	raw, err := queryWhois(server, domain)
	if err != nil {
		return whoisJSON(whoisResult{Status: "error", Message: fmt.Sprintf("WHOIS query failed: %v", err)})
	}

	result := parseWhoisResponse(raw, domain)
	if includeRaw {
		result.Raw = raw
	}
	return whoisJSON(result)
}

func whoisJSON(r whoisResult) string {
	b, _ := json.Marshal(r)
	return string(b)
}

func queryWhois(server, domain string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", server+":43")
	if err != nil {
		return "", fmt.Errorf("connect to %s: %w", server, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return "", fmt.Errorf("set deadline: %w", err)
	}

	if _, err := fmt.Fprintf(conn, "%s\r\n", domain); err != nil {
		return "", fmt.Errorf("send query: %w", err)
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if sb.Len() == 0 {
		return "", fmt.Errorf("empty response from %s", server)
	}

	return sb.String(), nil
}

func parseWhoisResponse(raw, domain string) whoisResult {
	r := whoisResult{
		Status: "success",
		Domain: domain,
		Extra:  make(map[string]string),
	}

	var statuses []string
	var nameServers []string

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ">>>") {
			continue
		}

		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if val == "" {
			continue
		}

		keyLower := strings.ToLower(key)

		switch {
		case keyLower == "registrar" || keyLower == "registrar name" || keyLower == "sponsoring registrar":
			if r.Registrar == "" {
				r.Registrar = val
			}
		case keyLower == "creation date" || keyLower == "created" || keyLower == "registration date" || keyLower == "created on":
			if r.Created == "" {
				r.Created = val
			}
		case keyLower == "registry expiry date" || keyLower == "expiration date" || keyLower == "registrar registration expiration date" || keyLower == "expires" || keyLower == "expires on":
			if r.Expires == "" {
				r.Expires = val
			}
		case keyLower == "updated date" || keyLower == "last updated" || keyLower == "last modified":
			if r.Updated == "" {
				r.Updated = val
			}
		case keyLower == "domain status":
			statuses = append(statuses, val)
		case keyLower == "name server" || keyLower == "nserver":
			nameServers = append(nameServers, strings.ToLower(strings.Fields(val)[0]))
		case keyLower == "dnssec":
			r.DNSSEC = val
		case keyLower == "registrant organization" || keyLower == "registrant name" || keyLower == "registrant":
			r.Extra["registrant"] = val
		case keyLower == "registrant country":
			r.Extra["country"] = val
		}
	}

	r.Status_ = statuses
	r.NameServer = nameServers

	return r
}

// whoisServers maps TLDs to their WHOIS servers.
var whoisServers = map[string]string{
	"com":   "whois.verisign-grs.com",
	"net":   "whois.verisign-grs.com",
	"org":   "whois.pir.org",
	"info":  "whois.afilias.net",
	"io":    "whois.nic.io",
	"dev":   "whois.nic.google",
	"app":   "whois.nic.google",
	"de":    "whois.denic.de",
	"uk":    "whois.nic.uk",
	"co.uk": "whois.nic.uk",
	"fr":    "whois.nic.fr",
	"nl":    "whois.sidn.nl",
	"eu":    "whois.eu",
	"ch":    "whois.nic.ch",
	"at":    "whois.nic.at",
	"be":    "whois.dns.be",
	"se":    "whois.iis.se",
	"no":    "whois.norid.no",
	"dk":    "whois.dk-hostmaster.dk",
	"it":    "whois.nic.it",
	"es":    "whois.nic.es",
	"pl":    "whois.dns.pl",
	"cz":    "whois.nic.cz",
	"us":    "whois.nic.us",
	"ca":    "whois.cira.ca",
	"au":    "whois.auda.org.au",
	"jp":    "whois.jprs.jp",
	"cn":    "whois.cnnic.cn",
	"ru":    "whois.tcinet.ru",
	"br":    "whois.registro.br",
	"in":    "whois.registry.in",
	"xyz":   "whois.nic.xyz",
	"me":    "whois.nic.me",
	"tv":    "whois.nic.tv",
	"cc":    "ccwhois.verisign-grs.com",
}

func getWhoisServer(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return "whois.iana.org"
	}

	// Try compound TLD first (e.g., co.uk)
	if len(parts) >= 3 {
		compound := parts[len(parts)-2] + "." + parts[len(parts)-1]
		if server, ok := whoisServers[compound]; ok {
			return server
		}
	}

	tld := parts[len(parts)-1]
	if server, ok := whoisServers[tld]; ok {
		return server
	}

	return "whois.iana.org"
}
