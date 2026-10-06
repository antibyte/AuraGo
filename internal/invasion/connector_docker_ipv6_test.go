package invasion

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDockerConnectorAPIURLHosts(t *testing.T) {
	cases := []struct {
		host string
		port int
		tls  string
		want string
	}{
		{"fd00::5", 2375, "", "http://[fd00::5]:2375"},
		{"[fd00::5]", 0, "", "http://[fd00::5]:2375"},
		{"fd00::5", 0, "tls", "https://[fd00::5]:2376"},
		{"10.0.0.5", 2375, "", "http://10.0.0.5:2375"},
		{"docker.lan", 0, "tls", "https://docker.lan:2376"},
		{"", 0, "", "http://:2375"},
	}
	for _, tc := range cases {
		nest := NestRecord{Host: tc.host, Port: tc.port, DeployMethod: "docker_remote", DockerTLS: tc.tls}
		want := fmt.Sprintf("%s/%s/version", tc.want, dockerAPIVersion)
		if got := (&DockerConnector{}).apiURL(nest, "/version"); got != want {
			t.Fatalf("apiURL(%q) = %q, want %q", tc.host, got, want)
		}
	}
}

func TestDockerConnectorReachesAnIPv6LoopbackEngine(t *testing.T) {
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"ApiVersion": "1.45", "MinAPIVersion": "1.25"})
	}))
	_ = ts.Listener.Close() // the default IPv4 listener is replaced
	ts.Listener = listener
	ts.Start()
	defer ts.Close()
	nest := NestRecord{ID: "12345678-abcd-ef12-3456-7890abcdef12", Host: "::1", Port: listener.Addr().(*net.TCPAddr).Port, DeployMethod: "docker_remote"}
	if err := (&DockerConnector{}).Validate(context.Background(), nest, nil); err != nil {
		t.Fatalf("Validate against [::1]: %v", err)
	}
}
