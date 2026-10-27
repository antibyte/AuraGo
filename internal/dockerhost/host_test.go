package dockerhost

import (
	"errors"
	"strings"
	"testing"
)

func validTCPHost() Host {
	return Host{Name: "NAS", Kind: KindTCP, Address: "nas.lan", Port: 2375, StacksDir: "/opt/stacks", HostSocketPath: "/var/run/docker.sock", Color: "teal"}
}

func TestValidateHost(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*Host)
		wantField string // "" = valid
	}{
		{"valid tcp", func(h *Host) {}, ""},
		{"valid tls with default port", func(h *Host) { h.Kind, h.Port = KindTLS, 0 }, ""},
		{"valid mtls", func(h *Host) { h.Kind = KindMTLS }, ""},
		{"valid ssh", func(h *Host) { h.Kind, h.SSHUser = KindSSH, "root" }, ""},
		{"valid IPv4", func(h *Host) { h.Address = "10.0.0.5" }, ""},
		{"valid bare IPv6", func(h *Host) { h.Address = "fd00::5" }, ""},
		{"valid bracketed IPv6", func(h *Host) { h.Address = "[fd00::5]" }, ""},
		{"valid name with underscore", func(h *Host) { h.Address = "my_nas.local" }, ""},
		{"empty name", func(h *Host) { h.Name = "" }, "name"},
		{"name of 65 characters", func(h *Host) { h.Name = strings.Repeat("a", 65) }, "name"},
		{"name of 64 multibyte characters", func(h *Host) { h.Name = strings.Repeat("\xc3\xa4", 64) }, ""},
		{"unknown kind", func(h *Host) { h.Kind = "http" }, "kind"},
		{"the local kind is built in", func(h *Host) { h.Kind = KindLocal }, "kind"},
		{"empty address", func(h *Host) { h.Address = "" }, "address"},
		{"address with scheme", func(h *Host) { h.Address = "tcp://nas.lan" }, "address"},
		{"address with path", func(h *Host) { h.Address = "nas.lan/docker" }, "address"},
		{"address with port", func(h *Host) { h.Address = "nas.lan:2375" }, "address"},
		{"address with user info", func(h *Host) { h.Address = "root@nas.lan" }, "address"},
		{"address with blank", func(h *Host) { h.Address = "nas lan" }, "address"},
		{"label starting with hyphen", func(h *Host) { h.Address = "-nas.lan" }, "address"},
		{"empty label", func(h *Host) { h.Address = "nas..lan" }, "address"},
		{"unclosed bracket", func(h *Host) { h.Address = "[fd00::5" }, "address"},
		{"bracketed IPv4", func(h *Host) { h.Address = "[10.0.0.5]" }, "address"},
		{"negative port", func(h *Host) { h.Port = -1 }, "port"},
		{"port above 65535", func(h *Host) { h.Port = 65536 }, "port"},
		{"relative stacks dir", func(h *Host) { h.StacksDir = "stacks" }, "stacks_dir"},
		{"stacks dir with ..", func(h *Host) { h.StacksDir = "/opt/../etc" }, "stacks_dir"},
		{"stacks dir is the root", func(h *Host) { h.StacksDir = "/" }, "stacks_dir"},
		{"stacks dir with backslash", func(h *Host) { h.StacksDir = `/opt\stacks` }, "stacks_dir"},
		{"empty stacks dir", func(h *Host) { h.StacksDir = "" }, "stacks_dir"},
		{"relative socket path", func(h *Host) { h.HostSocketPath = "docker.sock" }, "host_socket_path"},
		{"socket path with ..", func(h *Host) { h.HostSocketPath = "/run/../docker.sock" }, "host_socket_path"},
		{"unknown color", func(h *Host) { h.Color = "magenta" }, "color"},
		{"ssh without user", func(h *Host) { h.Kind, h.SSHUser = KindSSH, "" }, "ssh_user"},
		{"ssh user with blank", func(h *Host) { h.Kind, h.SSHUser = KindSSH, "ro ot" }, "ssh_user"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := validTCPHost()
			tc.mutate(&h)
			err := ValidateHost(h)
			if tc.wantField == "" {
				if err != nil {
					t.Fatalf("ValidateHost = %v, want valid", err)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tc.wantField {
				t.Fatalf("ValidateHost = %v, want a ValidationError for %q", err, tc.wantField)
			}
		})
	}
}

func TestHostNormalizedAppliesDefaultsAndLeavesTraversalForValidation(t *testing.T) {
	got := Host{Name: "  NAS ", Kind: " TLS ", Address: " nas.lan ", Color: " Teal ", SSHUser: "root", StacksDir: "", HostSocketPath: " /var/run//docker.sock "}.Normalized()
	if got.Name != "NAS" || got.Kind != KindTLS || got.Address != "nas.lan" || got.Color != "teal" {
		t.Fatalf("trimmed fields = %+v", got)
	}
	if got.StacksDir != DefaultStacksDir || got.HostSocketPath != "/var/run/docker.sock" {
		t.Fatalf("paths = %q, %q; want the default stacks dir and the cleaned socket path", got.StacksDir, got.HostSocketPath)
	}
	if got.SSHUser != "" {
		t.Fatalf("SSHUser = %q, want it dropped for a tls host", got.SSHUser)
	}
	if ssh := (Host{Kind: "ssh", SSHUser: " root "}).Normalized(); ssh.SSHUser != "root" {
		t.Fatalf("ssh user = %q, want root", ssh.SSHUser)
	}
	if bad := (Host{StacksDir: "/opt/../etc"}).Normalized(); bad.StacksDir != "/opt/../etc" {
		t.Fatalf("a path with .. segments must reach ValidateHost as typed, got %q", bad.StacksDir)
	}
}
