package config

import (
	"fmt"
	"regexp"
)

// GogConfig keeps gog accounts independent of the native Google integration.
type GogConfig struct {
	Enabled  bool         `yaml:"enabled" json:"enabled"`
	ReadOnly bool         `yaml:"readonly" json:"readonly"`
	Accounts []GogAccount `yaml:"accounts" json:"accounts"`
}

type GogAccount struct {
	ID            string               `yaml:"id" json:"id"`
	Name          string               `yaml:"name" json:"name"`
	ClientID      string               `yaml:"client_id" json:"client_id"`
	Enabled       bool                 `yaml:"enabled" json:"enabled"`
	AgentAccess   bool                 `yaml:"agent_access" json:"agent_access"`
	DesktopAccess bool                 `yaml:"desktop_access" json:"desktop_access"`
	Services      map[string]GogGrants `yaml:"services" json:"services"`
}

type GogGrants struct {
	Read   bool `yaml:"read" json:"read"`
	Create bool `yaml:"create" json:"create"`
	Update bool `yaml:"update" json:"update"`
	Delete bool `yaml:"delete" json:"delete"`
	Send   bool `yaml:"send" json:"send"`
}

var GogServices = []string{"gmail", "calendar", "drive", "docs", "sheets", "slides", "contacts", "tasks"}
var gogAccountID = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,47}$`)

func ValidGogAccountID(id string) bool { return gogAccountID.MatchString(id) }

func ValidateGogConfig(c GogConfig) error {
	if len(c.Accounts) > 20 {
		return fmt.Errorf("gog: at most 20 accounts are supported")
	}
	seen := map[string]bool{}
	for _, a := range c.Accounts {
		if !ValidGogAccountID(a.ID) || seen[a.ID] {
			return fmt.Errorf("gog: invalid or duplicate account ID")
		}
		seen[a.ID] = true
		if len(a.Name) > 120 || len(a.ClientID) > 256 {
			return fmt.Errorf("gog: account field too long")
		}
		for service, g := range a.Services {
			found := false
			for _, known := range GogServices {
				if service == known {
					found = true
				}
			}
			if !found || (service != "gmail" && g.Send) {
				return fmt.Errorf("gog: invalid service permission")
			}
		}
	}
	return nil
}

func (c GogConfig) Account(id string) (GogAccount, bool) {
	for _, a := range c.Accounts {
		if a.ID == id {
			return a, true
		}
	}
	return GogAccount{}, false
}
