package dockerstacks

import "testing"

func TestParsePortSpec(t *testing.T) {
	cases := []struct {
		in   string
		want string // "ip|host|container|protocol"; "" = invalid
	}{
		{"80", "||80|tcp"},
		{"8080:80", "|8080|80|tcp"},
		{"127.0.0.1:8080:80/udp", "127.0.0.1|8080|80|udp"},
		{"127.0.0.1::80", "127.0.0.1||80|tcp"},
		{"[::1]:8080:80", "::1|8080|80|tcp"},
		{"8000-8010:8000-8010", "|8000-8010|8000-8010|tcp"},
		{"8000-8010:80", "|8000-8010|80|tcp"},
		{"80-90", "||80-90|tcp"},
		{"80/TCP", "||80|tcp"},
		{"8080:80/sctp", "|8080|80|sctp"},
		{"", ""},
		{"abc", ""},
		{"0:80", ""},
		{"70000:80", ""},
		{"80:", ""},
		{"1:2:3:4", ""},
		{"8080:80/icmp", ""},
		{"300.1.1.1:80:80", ""},
		{"8000-8010:8000-8020", ""},
		{"9000-8000:80", ""},
		{"[::1]:80", ""},
		{"[::1:80:80", ""},
		{" 80 ", "||80|tcp"},
	}
	for _, tc := range cases {
		got, err := parsePortSpec(tc.in)
		if tc.want == "" {
			if err == nil {
				t.Errorf("parsePortSpec(%q) = %+v, want an error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePortSpec(%q) failed: %v", tc.in, err)
			continue
		}
		if line := got.HostIP + "|" + got.hostText() + "|" + got.containerText() + "|" + got.Protocol; line != tc.want {
			t.Errorf("parsePortSpec(%q) = %q, want %q", tc.in, line, tc.want)
		}
	}
}

func TestPortSpecFixedHostPorts(t *testing.T) {
	for in, want := range map[string]bool{
		"8080:80": true, "8000-8010:8000-8010": true, "80": false, "8000-8010:80": false, "127.0.0.1::80": false,
	} {
		spec, err := parsePortSpec(in)
		if err != nil || spec.fixedHostPorts() != want {
			t.Errorf("%q: fixedHostPorts = %v (err %v), want %v", in, spec.fixedHostPorts(), err, want)
		}
	}
}

func TestParseVolumeShort(t *testing.T) {
	cases := []struct {
		in   string
		want string // "kind|source|target|ro"; "" = invalid
	}{
		{"data:/var/lib/data", "named|data|/var/lib/data|false"},
		{"./cfg:/etc/cfg:ro", "relative|./cfg|/etc/cfg|true"},
		{"../shared:/s", "relative|../shared|/s|false"},
		{"/srv/x:/x", "bind|/srv/x|/x|false"},
		{"~/x:/x:rw", "bind|~/x|/x|false"},
		{"/data", "anonymous||/data|false"},
		{`C:\data:/data`, `bind|C:\data|/data|false`},
		{"data:/x:rw,z", "named|data|/x|false"},
		{"data:/x:z,ro", "named|data|/x|true"},
		{"a:b:c:d", ""},
		{":/x", ""},
		{"data/sub:/x", ""},
		{"data:", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got, err := parseVolumeShort(tc.in)
		if tc.want == "" {
			if err == nil {
				t.Errorf("parseVolumeShort(%q) = %+v, want an error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseVolumeShort(%q) failed: %v", tc.in, err)
			continue
		}
		ro := "false"
		if got.ReadOnly {
			ro = "true"
		}
		if line := got.Kind + "|" + got.Source + "|" + got.Target + "|" + ro; line != tc.want {
			t.Errorf("parseVolumeShort(%q) = %q, want %q", tc.in, line, tc.want)
		}
	}
}

func TestVolumeFromLong(t *testing.T) {
	cases := []struct {
		typ, source, target string
		ro                  bool
		want                string // "kind|source|target"; "" = invalid
	}{
		{"volume", "data", "/d", false, "named|data|/d"},
		{"", "data", "/d", false, "named|data|/d"},
		{"volume", "", "/d", false, "anonymous||/d"},
		{"bind", "./x", "/x", true, "relative|./x|/x"},
		{"bind", "/srv", "/x", false, "bind|/srv|/x"},
		{"tmpfs", "ignored", "/t", false, "tmpfs||/t"},
		{"bind", "data", "/x", false, ""},
		{"bind", "", "/x", false, ""},
		{"volume", "a b", "/x", false, ""},
		{"volume", "data", "", false, ""},
	}
	for _, tc := range cases {
		got, err := volumeFromLong(tc.typ, tc.source, tc.target, tc.ro)
		if tc.want == "" {
			if err == nil {
				t.Errorf("volumeFromLong(%q, %q, %q) = %+v, want an error", tc.typ, tc.source, tc.target, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("volumeFromLong(%q, %q, %q) failed: %v", tc.typ, tc.source, tc.target, err)
			continue
		}
		if line := got.Kind + "|" + got.Source + "|" + got.Target; line != tc.want || got.ReadOnly != tc.ro {
			t.Errorf("volumeFromLong(%q, %q, %q) = %q ro=%v, want %q ro=%v", tc.typ, tc.source, tc.target, line, got.ReadOnly, tc.want, tc.ro)
		}
	}
}
