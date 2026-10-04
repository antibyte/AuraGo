package tools

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomepageStatusDistinguishesSuccessfulQueryFromRunningServer(t *testing.T) {
	docker := httptest.NewServer(http.NotFoundHandler())
	defer docker.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cfg := HomepageConfig{DockerHost: strings.Replace(docker.URL, "http://", "tcp://", 1), WebServerPort: listener.Addr().(*net.TCPAddr).Port}
	for _, running := range []bool{true, false} {
		if !running {
			_ = listener.Close()
		}
		var result struct {
			Status string `json:"status"`
			Docker bool   `json:"docker_available"`
			Python struct {
				Running bool `json:"running"`
			} `json:"python_server"`
		}
		raw := HomepageStatus(cfg, slogDiscard())
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			t.Fatal(err)
		}
		if result.Status != "ok" || result.Docker || result.Python.Running != running {
			t.Fatalf("query status and runtime conflated: %s", raw)
		}
	}
}
