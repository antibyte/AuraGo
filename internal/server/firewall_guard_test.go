package server

import (
	"testing"

	"aurago/internal/config"
)

func TestFirewallGuardSudoPasswordOnlyWithoutDirectAccess(t *testing.T) {
	cases := []struct {
		name        string
		sudoEnabled bool
		accessOK    bool
		want        bool
	}{
		{"sudo disabled", false, false, false},
		{"root or NOPASSWD access", true, true, false},
		{"sudo disabled with access", false, true, false},
		{"sudo needed for iptables", true, false, true},
	}
	for _, tc := range cases {
		cfg := &config.Config{}
		cfg.Agent.SudoEnabled = tc.sudoEnabled
		cfg.Runtime.FirewallAccessOK = tc.accessOK
		if got := firewallGuardNeedsSudoPassword(cfg); got != tc.want {
			t.Errorf("%s: firewallGuardNeedsSudoPassword() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
