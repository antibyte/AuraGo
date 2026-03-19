package fritzbox

import "testing"

func TestValidatePort_Valid(t *testing.T) {
	for _, p := range []string{"1", "80", "443", "8080", "65535"} {
		if err := validatePort(p); err != nil {
			t.Errorf("validatePort(%q) = %v, want nil", p, err)
		}
	}
}

func TestValidatePort_Invalid(t *testing.T) {
	cases := []string{"0", "-1", "65536", "abc", "", "99999"}
	for _, p := range cases {
		if err := validatePort(p); err == nil {
			t.Errorf("validatePort(%q) = nil, want error", p)
		}
	}
}

func TestValidatePortForwardEntry_Valid(t *testing.T) {
	e := PortForwardEntry{
		ExternalPort:   "8080",
		InternalPort:   "80",
		Protocol:       "TCP",
		InternalClient: "192.168.1.100",
	}
	if err := ValidatePortForwardEntry(e); err != nil {
		t.Errorf("ValidatePortForwardEntry valid entry: %v", err)
	}
}

func TestValidatePortForwardEntry_UDP(t *testing.T) {
	e := PortForwardEntry{
		ExternalPort:   "53",
		InternalPort:   "53",
		Protocol:       "udp", // lowercase should be accepted
		InternalClient: "10.0.0.1",
	}
	if err := ValidatePortForwardEntry(e); err != nil {
		t.Errorf("ValidatePortForwardEntry UDP entry: %v", err)
	}
}

func TestValidatePortForwardEntry_InvalidProtocol(t *testing.T) {
	e := PortForwardEntry{
		ExternalPort:   "80",
		InternalPort:   "80",
		Protocol:       "ICMP",
		InternalClient: "192.168.1.1",
	}
	if err := ValidatePortForwardEntry(e); err == nil {
		t.Error("expected error for invalid protocol ICMP")
	}
}

func TestValidatePortForwardEntry_InvalidExternalPort(t *testing.T) {
	e := PortForwardEntry{
		ExternalPort:   "0",
		InternalPort:   "80",
		Protocol:       "TCP",
		InternalClient: "192.168.1.1",
	}
	if err := ValidatePortForwardEntry(e); err == nil {
		t.Error("expected error for external port 0")
	}
}

func TestValidatePortForwardEntry_InvalidInternalPort(t *testing.T) {
	e := PortForwardEntry{
		ExternalPort:   "80",
		InternalPort:   "99999",
		Protocol:       "TCP",
		InternalClient: "192.168.1.1",
	}
	if err := ValidatePortForwardEntry(e); err == nil {
		t.Error("expected error for internal port 99999")
	}
}

func TestValidatePortForwardEntry_InvalidIP(t *testing.T) {
	e := PortForwardEntry{
		ExternalPort:   "80",
		InternalPort:   "80",
		Protocol:       "TCP",
		InternalClient: "not-an-ip",
	}
	if err := ValidatePortForwardEntry(e); err == nil {
		t.Error("expected error for invalid IP")
	}
}

func TestValidatePortForwardEntry_IPv6(t *testing.T) {
	e := PortForwardEntry{
		ExternalPort:   "443",
		InternalPort:   "443",
		Protocol:       "TCP",
		InternalClient: "::1",
	}
	if err := ValidatePortForwardEntry(e); err != nil {
		t.Errorf("ValidatePortForwardEntry IPv6: %v", err)
	}
}

func TestValidatePortForwardEntry_EmptyPort(t *testing.T) {
	e := PortForwardEntry{
		ExternalPort:   "",
		InternalPort:   "80",
		Protocol:       "TCP",
		InternalClient: "192.168.1.1",
	}
	if err := ValidatePortForwardEntry(e); err == nil {
		t.Error("expected error for empty external port")
	}
}
