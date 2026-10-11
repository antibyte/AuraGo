package config

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// SelectInitialHTTPSPort chooses a bindable port for a NEW native installation.
// Runtime loading and upgrades must never use this to change a saved port.
func SelectInitialHTTPSPort(host string) (int, error) {
	return selectInitialHTTPSPort(host, net.Listen)
}

// SaveInitialNetworkSettings patches the explicitly supplied server fields in
// one atomic write. Config.Save intentionally only persists runtime settings.
func SaveInitialNetworkSettings(path string, values map[string]interface{}) error {
	configSaveMu.Lock()
	defer configSaveMu.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read network config: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse network config: %w", err)
	}
	for field, value := range values {
		switch field {
		case "host", "https.enabled", "https.cert_mode", "https.domain", "https.email", "https.https_port", "https.http_port":
		default:
			return fmt.Errorf("unsupported initial network field %q", field)
		}
		if err := setYAMLPathValue(&doc, append([]string{"server"}, strings.Split(field, ".")...), value); err != nil {
			return fmt.Errorf("set network field: %w", err)
		}
	}
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return fmt.Errorf("encode network config: %w", err)
	}
	if err := WriteFileAtomic(path, out.Bytes(), 0600); err != nil {
		return fmt.Errorf("write network config: %w", err)
	}
	return nil
}

func selectInitialHTTPSPort(host string, listen func(string, string) (net.Listener, error)) (int, error) {
	var failures []error
	for _, port := range []int{443, 8443} {
		listener, err := listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err == nil {
			_ = listener.Close()
			return port, nil
		}
		failures = append(failures, err)
	}
	return 0, fmt.Errorf("cannot enable HTTPS on %q: port 443: %v; port 8443: %v; free one of these ports or explicitly configure another HTTPS port", host, failures[0], failures[1])
}
