package config

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"regexp"
)

// RTLSDRConfig enables the optional, receive-only Linux radio runtime.
// Zero values deliberately leave both hardware and agent access disabled.
type RTLSDRConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	ReadOnly   bool   `yaml:"read_only" json:"read_only"`
	AllowAgent bool   `yaml:"allow_agent" json:"allow_agent"`
	Device     string `yaml:"device" json:"device"`
	QuotaGB    int    `yaml:"quota_gb" json:"quota_gb"`
}

func (c RTLSDRConfig) Validate() error {
	if c.QuotaGB < 0 || c.QuotaGB > 1000 {
		return fmt.Errorf("rtl_sdr.quota_gb must be 1-1000, or 0 for the 10 GB default")
	}
	if c.Device != "" && !regexp.MustCompile(`^[0-9]{1,3}-[0-9.]{1,32}$`).MatchString(c.Device) {
		return fmt.Errorf("rtl_sdr.device must be an enumerated USB port identifier")
	}
	return nil
}
func (c *RTLSDRConfig) UnmarshalJSON(data []byte) error {
	type raw RTLSDRConfig
	v := raw(*c)
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	next := RTLSDRConfig(v)
	if err := next.Validate(); err != nil {
		return err
	}
	*c = next
	return nil
}
func (c *RTLSDRConfig) UnmarshalYAML(node *yaml.Node) error {
	type raw RTLSDRConfig
	var v raw
	if err := node.Decode(&v); err != nil {
		return err
	}
	next := RTLSDRConfig(v)
	if err := next.Validate(); err != nil {
		return err
	}
	*c = next
	return nil
}
