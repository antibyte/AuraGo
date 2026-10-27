package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

// The two internal networks prevent the public-looking fixture IP from reaching
// the Internet. No existing container or host service is stopped or reconfigured.
func TestGotenbergIsolatedDockerAcceptance(t *testing.T) {
	if os.Getenv("AURAGO_RUN_GOTENBERG_DOCKER") != "1" {
		t.Skip("set AURAGO_RUN_GOTENBERG_DOCKER=1 to run the isolated real-image acceptance")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	probe, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Os}}/{{.Server.Arch}}").CombinedOutput()
	if err != nil {
		t.Skipf("Docker daemon unavailable: %v", err)
	}
	platform := strings.Split(strings.TrimSpace(string(probe)), "/")
	if len(platform) != 2 || platform[0] != "linux" || (platform[1] != "amd64" && platform[1] != "arm64") {
		t.Fatalf("expected a Linux amd64/arm64 daemon, got %q", probe)
	}
	docker := func(args ...string) string {
		t.Helper()
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %s: %v\n%s", args[0], err, out)
		}
		return strings.TrimSpace(string(out))
	}
	cleanup := func(kind, name string) {
		t.Cleanup(func() {
			cleanupCtx, done := context.WithTimeout(context.Background(), 15*time.Second)
			defer done()
			args := []string{kind, "rm", name}
			if kind == "container" {
				args = []string{kind, "rm", "--force", name}
			}
			if out, err := exec.CommandContext(cleanupCtx, "docker", args...).CombinedOutput(); err != nil {
				t.Errorf("cleanup %s %s: %v: %s", kind, name, err, out)
			}
		})
	}
	docker("pull", gotenbergImage)
	name := fmt.Sprintf("aurago-document-test-%d", time.Now().UnixNano())
	publicNet, privateNet := name+"-public", name+"-private"
	docker("network", "create", "--internal", "--subnet", "93.184.216.0/24", publicNet)
	cleanup("network", publicNet)
	docker("network", "create", "--internal", "--subnet", "192.168.245.0/24", privateNet)
	cleanup("network", privateNet)

	fixtureDir := t.TempDir()
	if err := os.Chmod(fixtureDir, 0755); err != nil {
		t.Fatal(err)
	}
	build := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(fixtureDir, "fixture"), "./testdata/gotenberg_fixture")
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+platform[1])
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build isolated HTTP fixture: %v: %s", err, out)
	}
	if err := os.Chmod(filepath.Join(fixtureDir, "fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	fixture := docker("run", "--detach", "--network", publicNet, "--ip", "93.184.216.2", "--cap-drop", "ALL",
		"--publish", "127.0.0.1::8080", "--mount", "type=bind,source="+fixtureDir+",target=/fixture,readonly",
		"--entrypoint", "/fixture/fixture", gotenbergImage)
	cleanup("container", fixture)
	docker("network", "connect", "--ip", "192.168.245.2", privateNet, fixture)
	args := []string{"run", "--detach", "--network", publicNet, "--ip", "93.184.216.3", "--publish", "127.0.0.1::3000",
		"--cap-drop", "ALL", "--cap-add", "CHOWN", "--cap-add", "FOWNER", "--security-opt", "no-new-privileges",
		"--cpus", "1", "--memory", "1g", "--shm-size", "256m"}
	for _, env := range gotenbergEnvironment() {
		args = append(args, "--env", env)
	}
	container := docker(append(args, gotenbergImage)...)
	cleanup("container", container)
	docker("network", "connect", "--ip", "192.168.245.3", privateNet, container)
	endpoint := "http://" + docker("port", container, "3000/tcp")
	statsURL := "http://" + docker("port", fixture, "8080/tcp") + "/stats"
	cfg := &config.DocumentCreatorConfig{Backend: "gotenberg", OutputDir: t.TempDir(), Gotenberg: config.GotenbergConfig{URL: endpoint, Timeout: 30}}
	if err := waitForGotenberg(ctx, &cfg.Gotenberg); err != nil {
		t.Fatal(err)
	}
	stats := func() map[string]int64 {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, statsURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var result map[string]int64
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1024)).Decode(&result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	readOutput := func(result string) []byte {
		t.Helper()
		out := c102DecodeResult(t, result)
		if out["status"] != "success" {
			t.Fatal(result)
		}
		data, err := os.ReadFile(out["file_path"].(string))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	render := func(operation, html string) []byte {
		return readOutput(ExecuteDocumentCreator(ctx, cfg, operation, "", html, "", "acceptance", "A4", false, "", ""))
	}
	const page = `<!doctype html><html><head><link rel="stylesheet" href="http://93.184.216.2:8080/public"></head><body><h1>Document acceptance</h1></body></html>`
	if data := render("html_to_pdf", page); !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatal("renderer did not produce a PDF")
	}
	if _, err := png.DecodeConfig(bytes.NewReader(render("screenshot_html", page))); err != nil {
		t.Fatalf("renderer did not produce a PNG: %v", err)
	}
	if got := stats()["public"]; got != 0 {
		t.Fatalf("offline default fetched %d public resources", got)
	}
	cfg.AllowRemoteResources = true
	render("html_to_pdf", page)
	if stats()["public"] == 0 {
		t.Fatal("admin resource grant did not load the public fixture")
	}
	// Exercise the renderer's own filter, independently of AuraGo's URL preflight.
	for _, target := range []string{"http://192.168.245.2:8080/private", "http://93.184.216.2:8080/redirect"} {
		if _, err := gotenbergRequest(ctx, &cfg.Gotenberg, "/forms/chromium/convert/url", map[string]string{"url": target}, nil); err == nil {
			t.Fatalf("private target or redirect was accepted: %s", target)
		}
	}
	_, err = gotenbergRequest(ctx, &cfg.Gotenberg, "/forms/chromium/convert/html", map[string]string{"failOnResourceLoadingFailed": "true"}, map[string][]byte{
		"index.html": []byte(`<html><body><img src="http://192.168.245.2:8080/private"></body></html>`),
	})
	if err == nil {
		t.Fatal("private subresource did not fail resource loading")
	}
	if got := stats()["private"]; got != 0 {
		t.Fatalf("renderer reached the private trap %d times", got)
	}
}
