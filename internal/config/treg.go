package config

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// TregEndpointGrant binds an operator-selected permission to a catalog contract.
type TregEndpointGrant struct {
	EndpointID string `yaml:"endpoint_id" json:"endpoint_id"`
	Operation  string `yaml:"operation" json:"operation"`
	Method     string `yaml:"method" json:"method"`
	Path       string `yaml:"path" json:"path"`
}

type TregConfig struct {
	Enabled          bool                `yaml:"enabled" json:"enabled"`
	ReadOnly         bool                `yaml:"readonly" json:"readonly"`
	MaxCallCostMicro int64               `yaml:"max_call_cost_micro" json:"max_call_cost_micro"`
	AllowedEndpoints []TregEndpointGrant `yaml:"allowed_endpoints" json:"allowed_endpoints"`
	Token            string              `yaml:"-" json:"-" vault:"token"`
}

func DefaultTregConfig() TregConfig {
	return TregConfig{ReadOnly: true, MaxCallCostMicro: 1_000_000, AllowedEndpoints: []TregEndpointGrant{}}
}

func (c *TregConfig) UnmarshalYAML(node *yaml.Node) error {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "max_call_cost_micro" && node.Content[i+1].Tag != "!!int" {
			return fmt.Errorf("treg.max_call_cost_micro must be an integer")
		}
	}
	type plain TregConfig
	v := plain(DefaultTregConfig())
	if err := node.Decode(&v); err != nil {
		return fmt.Errorf("decode treg configuration: %w", err)
	}
	*c = TregConfig(v)
	return ValidateTregConfig(*c)
}

var tregEndpointID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,199}$`)

func ValidTregEndpointID(id string) bool { return tregEndpointID.MatchString(id) }

func ValidateTregConfig(c TregConfig) error {
	if c.MaxCallCostMicro < 0 || c.MaxCallCostMicro > 1_000_000_000_000 {
		return fmt.Errorf("treg.max_call_cost_micro must be between 0 and 1000000000000")
	}
	if len(c.AllowedEndpoints) > 256 {
		return fmt.Errorf("treg allows at most 256 endpoint grants")
	}
	seen := make(map[string]bool, len(c.AllowedEndpoints))
	for _, g := range c.AllowedEndpoints {
		if !ValidTregEndpointID(g.EndpointID) || seen[g.EndpointID] {
			return fmt.Errorf("treg endpoint IDs must be valid and unique")
		}
		seen[g.EndpointID] = true
		switch g.Operation {
		case "read", "create", "update", "delete":
		default:
			return fmt.Errorf("treg endpoint %s needs an explicit read/create/update/delete permission", g.EndpointID)
		}
		switch g.Method {
		case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE":
		default:
			return fmt.Errorf("treg endpoint %s has an invalid method", g.EndpointID)
		}
		if !strings.HasPrefix(g.Path, "/") || strings.HasPrefix(g.Path, "//") || strings.ContainsAny(g.Path, "\r\n\x00") || len(g.Path) > 2048 {
			return fmt.Errorf("treg endpoint %s has an invalid path template", g.EndpointID)
		}
	}
	return nil
}

// Grant checks the operator's exact permission, never the provider's description/kind.
func (c TregConfig) Grant(id, operation string) (TregEndpointGrant, error) {
	if !c.Enabled {
		return TregEndpointGrant{}, fmt.Errorf("treg is disabled")
	}
	for _, g := range c.AllowedEndpoints {
		if g.EndpointID != id {
			continue
		}
		if g.Operation != operation || (c.ReadOnly && g.Operation != "read") {
			return g, fmt.Errorf("treg endpoint permission or read-only policy denies this action")
		}
		return g, nil
	}
	return TregEndpointGrant{}, fmt.Errorf("treg endpoint is not approved")
}
