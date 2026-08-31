package config

import "testing"

func TestValidateVirtualComputersAgentControlCIDRs(t *testing.T) {
	valid := VirtualComputersAgentControl{DefaultTemplate: "desktop"}
	valid.Network.DefaultProfile = "internet_lan"
	valid.Network.AllowedPrivateCIDRs = []string{"10.42.0.0/16", "192.168.1.8/32"}
	if err := ValidateVirtualComputersAgentControl(valid); err != nil {
		t.Fatalf("valid agent control rejected: %v", err)
	}

	for _, cidr := range []string{"8.8.8.0/24", "10.0.0.0/7", "169.254.0.0/16", "fc00::/7", "invalid"} {
		candidate := valid
		candidate.Network.AllowedPrivateCIDRs = []string{cidr}
		if err := ValidateVirtualComputersAgentControl(candidate); err == nil {
			t.Fatalf("CIDR %q unexpectedly accepted", cidr)
		}
	}
}

func TestNormalizeVirtualComputersAgentControlDefaults(t *testing.T) {
	var control VirtualComputersAgentControl
	normalizeVirtualComputersAgentControl(&control)
	if control.DefaultTemplate != "desktop" || control.MaxActiveWorkspaces != 2 || control.IdleTTLSeconds != 600 {
		t.Fatalf("workspace defaults = %+v", control)
	}
	if control.MaxWorkspaceSeconds != 7200 || control.MaxJobSeconds != 3600 || control.MaxJobOutputMB != 4 {
		t.Fatalf("workspace limits = %+v", control)
	}
	if control.Network.DefaultProfile != "internet_lan" || control.Credentials.GrantTTLSeconds != 900 {
		t.Fatalf("workspace network/credentials defaults = %+v", control)
	}
}

func TestValidateVirtualComputersAgentControlLimits(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*VirtualComputersAgentControl){
		"idle lease": func(control *VirtualComputersAgentControl) { control.IdleTTLSeconds = 901 },
		"output":     func(control *VirtualComputersAgentControl) { control.MaxJobOutputMB = 65 },
		"jobs":       func(control *VirtualComputersAgentControl) { control.JobsPerWorkspace = 3 },
		"browsers":   func(control *VirtualComputersAgentControl) { control.BrowserSessionsPerWorkspace = 2 },
		"grant ttl":  func(control *VirtualComputersAgentControl) { control.Credentials.GrantTTLSeconds = 3601 },
	} {
		t.Run(name, func(t *testing.T) {
			control := VirtualComputersAgentControl{DefaultTemplate: "desktop"}
			control.Network.DefaultProfile = "internet_lan"
			mutate(&control)
			if err := ValidateVirtualComputersAgentControl(control); err == nil {
				t.Fatal("out-of-range agent control setting unexpectedly accepted")
			}
		})
	}
}
