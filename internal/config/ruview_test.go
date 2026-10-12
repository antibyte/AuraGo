package config

import "testing"

func TestRuViewDefaultsAndNetworkBoundary(t *testing.T) {
	c := DefaultRuViewConfig()
	if c.Enabled || c.AgentAccess || !c.ReadOnly {
		t.Fatal("unsafe defaults")
	}
	if err := ValidateRuViewConfig(c, false); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"0.0.0.0", "127.0.0.1", "169.254.169.254", "8.8.8.8", "::1", "10.1.2.3/4"} {
		c.UDPEnabled = true
		c.UDPBind = address
		c.UDPAllow = []string{"10.1.2.3"}
		if ValidateRuViewConfig(c, false) == nil {
			t.Errorf("accepted bind %s", address)
		}
	}
	c.UDPBind = "192.168.1.2"
	c.UDPAllow = []string{"192.168.0.0/16"}
	if err := ValidateRuViewConfig(c, false); err != nil {
		t.Fatal(err)
	}
	c.UDPAllow = []string{"10.0.0.0/7"}
	if ValidateRuViewConfig(c, false) == nil {
		t.Fatal("accepted oversized CIDR")
	}
	c = DefaultRuViewConfig()
	c.Mode = "external"
	c.Enabled = true
	c.URL = "http://127.0.0.1:3022"
	if ValidateRuViewConfig(c, false) == nil {
		t.Fatal("implicit LAN grant")
	}
	c.AllowPrivate = true
	if err := ValidateRuViewConfig(c, false); err != nil {
		t.Fatal(err)
	}
	c.URL = "http://user:secret@example.com"
	if ValidateRuViewConfig(c, false) == nil {
		t.Fatal("credentials in URL")
	}
}
