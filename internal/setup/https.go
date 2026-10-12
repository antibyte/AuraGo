package setup

import (
	"bytes"
	"fmt"
	"os"

	"aurago/internal/config"
	"gopkg.in/yaml.v3"
)

// initialHTTPSConfig is only used when config.yaml does not exist. Keeping this
// out of Load preserves the effective HTTP settings of older installations.
func initialHTTPSConfig(data []byte, docker bool) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse initial config: %w", err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("initial config must be a YAML mapping")
	}
	server := initialConfigField(doc.Content[0], "server")
	if server.Kind != yaml.MappingNode {
		*server = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
	host := initialConfigField(server, "host")
	if host.Value == "" {
		_ = host.Encode("127.0.0.1")
	}
	bindHost := host.Value
	if override, ok := os.LookupEnv("AURAGO_SERVER_HOST"); ok {
		bindHost = override
	}
	port := 8443
	if !docker {
		var err error
		port, err = config.SelectInitialHTTPSPort(bindHost)
		if err != nil {
			return nil, err
		}
	}
	https := initialConfigField(server, "https")
	if https.Kind != yaml.MappingNode {
		*https = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
	for key, value := range map[string]interface{}{"enabled": true, "cert_mode": "selfsigned", "https_port": port, "http_port": 0} {
		if err := initialConfigField(https, key).Encode(value); err != nil {
			return nil, fmt.Errorf("encode initial HTTPS config: %w", err)
		}
	}
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return nil, fmt.Errorf("encode initial config: %w", err)
	}
	return out.Bytes(), nil
}

func initialConfigField(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	value := &yaml.Node{}
	parent.Content = append(parent.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
	return value
}
