package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/security"
	"gopkg.in/yaml.v3"
)

func cloudflareFixtureContainer(running bool, auth string) cloudflareContainer {
	c := cloudflareContainer{ID: strings.Repeat("a", 64), Name: "/" + cfdContainerName}
	c.Config.Image = "cloudflare/cloudflared:latest"
	c.Config.Cmd = []string{"tunnel", "run"}
	if auth == "named" {
		c.Config.Cmd = append(c.Config.Cmd, "fixture")
	}
	if auth == "quick" {
		c.Config.Cmd = []string{"tunnel", "--url", "http://127.0.0.1:18080"}
	}
	c.State.Running = running
	c.State.StartedAt = time.Now().Add(-time.Minute)
	return c
}

func TestCloudflareOrphanReconcileAndStop(t *testing.T) {
	for _, auth := range []string{"token", "named", "quick"} {
		t.Run(auth, func(t *testing.T) {
			configureDockerSecurityTestPermissions(t, false)
			resetCloudflareTunnelRuntimeForTest()
			t.Cleanup(resetCloudflareTunnelRuntimeForTest)
			var mu sync.Mutex
			container := cloudflareFixtureContainer(true, auth)
			exists := true
			stops, removes := 0, 0
			host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				switch {
				case strings.HasSuffix(r.URL.Path, "/json"):
					if !exists {
						w.WriteHeader(404)
						return
					}
					_ = json.NewEncoder(w).Encode(container)
				case strings.HasSuffix(r.URL.Path, "/stop"):
					if !strings.Contains(r.URL.Path, container.ID) {
						t.Error("stop did not use original container ID")
					}
					stops++
					container.State.Running = false
					w.WriteHeader(204)
				case r.Method == http.MethodDelete:
					removes++
					exists = false
					w.WriteHeader(204)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(500)
				}
			})
			cfg := CloudflareTunnelConfig{Enabled: true, Mode: "docker", DockerHost: host, AuthMethod: auth, DataDir: t.TempDir()}
			if result := CloudflareTunnelReconcile(cfg, slogDiscard()); !cloudflareTunnelToolResultOK(result) {
				t.Fatal(result)
			}
			if auth == "quick" {
				if IsTunnelRunning() || stops != 1 || removes != 1 {
					t.Fatal("orphan quick publication was adopted")
				}
				return
			}
			if !IsTunnelRunning() {
				t.Fatal("running orphan was not adopted with auto_start=false")
			}
			var status map[string]interface{}
			_ = json.Unmarshal([]byte(CloudflareTunnelStatus(cfg, nil, slogDiscard())), &status)
			if status["running"] != true || status["auth_method"] != auth {
				t.Fatal(status)
			}
			cfg.Enabled = false
			if result := CloudflareTunnelShutdown(cfg, nil, slogDiscard(), false); !cloudflareTunnelToolResultOK(result) {
				t.Fatal(result)
			}
			if stops != 1 || removes != 1 || IsTunnelRunning() {
				t.Fatal("disabled tunnel remained active")
			}
		})
	}
}

func TestCloudflareStateLossStopAndUnknownStatus(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	for _, scenario := range []string{"state_loss", "stop_failure", "remove_failure", "foreign", "unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			resetCloudflareTunnelRuntimeForTest()
			t.Cleanup(resetCloudflareTunnelRuntimeForTest)
			container := cloudflareFixtureContainer(true, "token")
			if scenario == "foreign" {
				container.Config.Image = "unrelated/image:latest"
			}
			mutations := 0
			host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					if scenario == "unavailable" {
						w.WriteHeader(503)
						return
					}
					_ = json.NewEncoder(w).Encode(container)
					return
				}
				mutations++
				if scenario == "stop_failure" || scenario == "remove_failure" && r.Method == http.MethodDelete {
					w.WriteHeader(500)
					return
				}
				w.WriteHeader(204)
			})
			cfg := CloudflareTunnelConfig{Mode: "docker", DockerHost: host, DataDir: t.TempDir()}
			var status map[string]interface{}
			_ = json.Unmarshal([]byte(CloudflareTunnelStatus(cfg, nil, slogDiscard())), &status)
			if scenario == "foreign" || scenario == "unavailable" {
				if status["state_known"] != false || status["running"] != nil {
					t.Fatal(status)
				}
			}
			result := CloudflareTunnelStop(cfg, nil, slogDiscard())
			if scenario == "state_loss" {
				if !cloudflareTunnelToolResultOK(result) || mutations != 2 {
					t.Fatal(result)
				}
			} else {
				if cloudflareTunnelToolResultOK(result) {
					t.Fatal("unconfirmed stop was acknowledged")
				}
				if (scenario == "foreign" || scenario == "unavailable") && mutations != 0 {
					t.Fatal("mutated unverified container")
				}
				if scenario == "stop_failure" || scenario == "remove_failure" {
					if !IsTunnelRunning() {
						t.Fatal("lost retryable state")
					}
				}
			}
		})
	}
}

func TestCloudflareStoppedContainersRespectAutoStart(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	for _, auth := range []string{"token", "named"} {
		for _, automatic := range []bool{false, true} {
			resetCloudflareTunnelRuntimeForTest()
			container := cloudflareFixtureContainer(false, auth)
			starts := 0
			host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(container)
					return
				}
				if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/start") || !strings.Contains(r.URL.Path, container.ID) {
					t.Errorf("unexpected mutation %s %s", r.Method, r.URL.Path)
				}
				starts++
				w.WriteHeader(204)
			})
			cfg := CloudflareTunnelConfig{Enabled: true, Mode: "docker", DockerHost: host, AuthMethod: auth, AutoStart: automatic}
			if result := CloudflareTunnelReconcile(cfg, slogDiscard()); !cloudflareTunnelToolResultOK(result) {
				t.Fatal(result)
			}
			if automatic && starts != 1 || !automatic && starts != 0 {
				t.Fatalf("auth=%s auto=%v starts=%d", auth, automatic, starts)
			}
		}
	}
	resetCloudflareTunnelRuntimeForTest()
}

func TestCloudflareSlowStatusDoesNotBlockStateOrStop(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	resetCloudflareTunnelRuntimeForTest()
	t.Cleanup(resetCloudflareTunnelRuntimeForTest)
	container := cloudflareFixtureContainer(true, "token")
	entered, release := make(chan struct{}), make(chan struct{})
	var slow atomic.Bool
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if slow.Load() {
				close(entered)
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
			}
			_ = json.NewEncoder(w).Encode(container)
			return
		}
		w.WriteHeader(204)
	})
	t.Cleanup(func() { close(release) })
	cfg := CloudflareTunnelConfig{Enabled: true, Mode: "docker", DockerHost: host, DataDir: t.TempDir()}
	if result := CloudflareTunnelReconcile(cfg, slogDiscard()); !cloudflareTunnelToolResultOK(result) {
		t.Fatal(result)
	}
	slow.Store(true)
	done := make(chan string, 1)
	go func() { done <- CloudflareTunnelStatus(cfg, nil, slogDiscard()) }()
	<-entered
	stopped := make(chan string, 1)
	go func() { stopped <- CloudflareTunnelStop(cfg, nil, slogDiscard()) }()
	select {
	case result := <-stopped:
		if !cloudflareTunnelToolResultOK(result) {
			t.Fatal(result)
		}
	case <-time.After(time.Second):
		t.Fatal("slow status held the state/lifecycle lock")
	}
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("inspect exceeded its three-second budget")
	}
	if IsTunnelRunning() {
		t.Fatal("late inspect resurrected stopped state")
	}
}

func TestCloudflareRestartIsSerializedAndDoesNotRestartAfterUnconfirmedStop(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	for _, failedStop := range []bool{false, true} {
		t.Run(map[bool]string{false: "serialized", true: "failed_stop"}[failedStop], func(t *testing.T) {
			resetCloudflareTunnelRuntimeForTest()
			t.Cleanup(resetCloudflareTunnelRuntimeForTest)
			container := cloudflareFixtureContainer(true, "token")
			var mu sync.Mutex
			exists := true
			stopEntered, release := make(chan struct{}), make(chan struct{})
			creates := 0
			host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/stop"):
					close(stopEntered)
					<-release
					if failedStop {
						w.WriteHeader(500)
					} else {
						w.WriteHeader(204)
					}
				case r.Method == http.MethodDelete:
					mu.Lock()
					exists = false
					mu.Unlock()
					w.WriteHeader(204)
				case strings.HasSuffix(r.URL.Path, "/images/json"):
					_, _ = io.WriteString(w, `[{}]`)
				case strings.HasSuffix(r.URL.Path, "/containers/create"):
					mu.Lock()
					creates++
					exists = true
					mu.Unlock()
					w.WriteHeader(201)
					_ = json.NewEncoder(w).Encode(map[string]string{"Id": container.ID})
				case strings.HasSuffix(r.URL.Path, "/start"):
					w.WriteHeader(204)
				default:
					mu.Lock()
					present := exists
					mu.Unlock()
					if !present {
						w.WriteHeader(404)
					} else {
						_ = json.NewEncoder(w).Encode(container)
					}
				}
			})
			cfg := CloudflareTunnelConfig{Enabled: true, Mode: "docker", AuthMethod: "token", DockerHost: host, DataDir: t.TempDir()}
			if result := CloudflareTunnelReconcile(cfg, slogDiscard()); !cloudflareTunnelToolResultOK(result) {
				t.Fatal(result)
			}
			vault, err := security.NewVault(strings.Repeat("5a", 32), filepath.Join(t.TempDir(), "vault.bin"))
			if err != nil {
				t.Fatal(err)
			}
			if err := vault.WriteSecret("cloudflared_token", "fixture-connector"); err != nil {
				t.Fatal(err)
			}
			restarted := make(chan string, 1)
			go func() { restarted <- CloudflareTunnelRestart(cfg, vault, nil, slogDiscard()) }()
			<-stopEntered
			started := make(chan string, 1)
			go func() { started <- CloudflareTunnelStart(cfg, vault, nil, slogDiscard()) }()
			select {
			case result := <-started:
				t.Fatalf("competing start entered restart transaction: %s", result)
			case <-time.After(30 * time.Millisecond):
			}
			close(release)
			result := <-restarted
			if failedStop == cloudflareTunnelToolResultOK(result) {
				t.Fatal(result)
			}
			if result := <-started; cloudflareTunnelToolResultOK(result) {
				t.Fatal("competing start replaced the restarted tunnel")
			}
			if failedStop && creates != 0 || !failedStop && creates != 1 {
				t.Fatalf("unexpected creates: %d", creates)
			}
		})
	}
}

func TestCloudflareConfigRevocationRejectsQueuedStart(t *testing.T) {
	resetCloudflareTunnelRuntimeForTest()
	t.Cleanup(resetCloudflareTunnelRuntimeForTest)
	old := CloudflareTunnelConfig{Enabled: true, Mode: "native", AuthMethod: "token"}
	next := old
	next.Enabled = false
	finish, err := CloudflareTunnelPrepareConfigChange(old, next, nil, slogDiscard(), false)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan string, 1)
	go func() { started <- CloudflareTunnelStart(old, nil, nil, slogDiscard()) }()
	finish(true)
	if result := <-started; cloudflareTunnelToolResultOK(result) || !strings.Contains(result, "revoked") {
		t.Fatal(result)
	}
}

func TestCloudflareNamedYAMLAndValidation(t *testing.T) {
	cfg := CloudflareTunnelConfig{TunnelName: "prod: tunnel #1", CustomIngress: []CloudflareIngress{{Hostname: "*.example.com", Path: "^/a:#\\t", Service: "https://localhost:8443"}, {Path: "^/public", Service: "http://localhost:3000"}}, HTTPSEnabled: true, HTTPSPort: 8443, ExposeWebUI: true, ExposeHomepage: true}
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := writeNamedTunnelConfig(cfg, "C:\\some folder\\credentials.json", path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["tunnel"] != cfg.TunnelName {
		t.Fatal("tunnel name was reinterpreted")
	}
	rules := doc["ingress"].([]interface{})
	if len(rules) != 3 || rules[2].(map[string]interface{})["service"] != "http_status:404" {
		t.Fatal(rules)
	}
	if rules[0].(map[string]interface{})["path"] != cfg.CustomIngress[0].Path {
		t.Fatal("regex was reinterpreted")
	}
	for _, rule := range []CloudflareIngress{{Service: "http://localhost:3000"}, {Hostname: "bad: host", Service: "http://localhost:3000"}, {Hostname: "a.*.com", Service: "http://localhost:3000"}, {Hostname: "a.example", Path: "[", Service: "http://localhost:3000"}, {Hostname: "a.example", Service: "http://user:pass@localhost:3000"}, {Hostname: "a.example", Service: "http://localhost:3000/a"}} {
		if err := validateCustomIngress([]CloudflareIngress{rule}); err == nil {
			t.Fatalf("invalid route accepted: %+v", rule)
		}
	}
	cfg.CustomIngress = nil
	if err := writeNamedTunnelConfig(cfg, "credentials.json", path); err == nil {
		t.Fatal("missing routes accepted")
	}
}

func TestCloudflareQuickURLAndRuntimeArguments(t *testing.T) {
	for _, text := range []string{"https://a.trycloudflare.com.evil.example", "https://trycloudflare.com", "https://a.b.trycloudflare.com", "https://a.trycloudflare.com@evil.example", "https://a.trycloudflare.com:443", "https://a.trycloudflare.com?q=1", "https://a.trycloudflare.com/#x", "https://a.trycloudflare.com:", "https://a.trycloudflare.com?", "https://a.trycloudflare.com#"} {
		if got := extractQuickTunnelURL(text); got != "" {
			t.Fatal(got)
		}
	}
	if got := extractQuickTunnelURL("| https://one-two.trycloudflare.com |"); got != "https://one-two.trycloudflare.com" {
		t.Fatal(got)
	}
	args := cloudflareRuntimeArgs(CloudflareTunnelConfig{LogLevel: "warn", MetricsPort: 19000}, tokenTunnelArgs())
	if strings.Join(args, " ") != "tunnel --no-autoupdate --loglevel warn --metrics localhost:19000 run" {
		t.Fatal(args)
	}
}

func TestCloudflareInstallerKeepsPreviousBinaryOnFailures(t *testing.T) {
	oldClient, oldPublish := cloudflaredDownloadClient, cloudflaredPublish
	t.Cleanup(func() { cloudflaredDownloadClient, cloudflaredPublish = oldClient, oldPublish })
	payload := "verified fixture"
	hash := sha256.Sum256([]byte(payload))
	metadata := cloudflaredDownload{DownloadURL: "https://fixture.invalid/binary", SHA256: hex.EncodeToString(hash[:])}
	for _, scenario := range []string{"success", "hash", "oversized", "stream_limit", "copy", "replace"} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cloudflared")
			if err := os.WriteFile(path, []byte("previous"), 0600); err != nil {
				t.Fatal(err)
			}
			cloudflaredPublish = oldPublish
			if scenario == "replace" {
				cloudflaredPublish = func(context.Context, string, string) error { return errors.New("fixture publication failure") }
			}
			cloudflaredDownloadClient = &http.Client{Transport: cloudflareRoundTripFunc(func(*http.Request) (*http.Response, error) {
				body := payload
				if scenario == "hash" {
					body = "wrong hash"
				}
				resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body))}
				if scenario == "oversized" {
					resp.ContentLength = cloudflaredMaxDownload + 1
				}
				if scenario == "copy" {
					resp.Body = io.NopCloser(cloudflareErrorReader{})
				}
				if scenario == "stream_limit" {
					resp.ContentLength = -1
					resp.Body = io.NopCloser(io.LimitReader(cloudflareZeroReader{}, cloudflaredMaxDownload+1))
				}
				return resp, nil
			})}
			result := installCloudflaredDownload(path, metadata, slogDiscard())
			data, _ := os.ReadFile(path)
			if scenario == "success" {
				if !cloudflareTunnelToolResultOK(result) || string(data) != payload {
					t.Fatal(result)
				}
			} else {
				if cloudflareTunnelToolResultOK(result) || string(data) != "previous" {
					t.Fatal(result)
				}
			}
			files, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".cloudflared-*"))
			if len(files) != 0 {
				t.Fatal("temporary files leaked")
			}
		})
	}
}

type cloudflareErrorReader struct{}

type cloudflareZeroReader struct{}

func (cloudflareZeroReader) Read(data []byte) (int, error) { clear(data); return len(data), nil }

func (cloudflareErrorReader) Read([]byte) (int, error) {
	return 0, errors.New("fixture stream failure")
}

func TestCloudflareInstallerSerializesDownloads(t *testing.T) {
	oldClient := cloudflaredDownloadClient
	t.Cleanup(func() { cloudflaredDownloadClient = oldClient })
	var active, peak atomic.Int32
	payload := "fixture"
	hash := sha256.Sum256([]byte(payload))
	cloudflaredDownloadClient = &http.Client{Transport: cloudflareRoundTripFunc(func(*http.Request) (*http.Response, error) {
		n := active.Add(1)
		if n > peak.Load() {
			peak.Store(n)
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(payload))}, nil
	})}
	metadata := cloudflaredDownload{DownloadURL: "https://fixture.invalid", SHA256: hex.EncodeToString(hash[:])}
	path := filepath.Join(t.TempDir(), "cloudflared")
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if result := installCloudflaredDownload(path, metadata, slogDiscard()); !cloudflareTunnelToolResultOK(result) {
				t.Error(result)
			}
		}()
	}
	wg.Wait()
	if peak.Load() != 1 {
		t.Fatal("parallel downloads were not serialized")
	}
}

func TestCloudflareTLSWarningsPersistAcrossStartRestartAndStatus(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	resetCloudflareTunnelRuntimeForTest()
	t.Cleanup(resetCloudflareTunnelRuntimeForTest)
	old := cfHTTPClient
	t.Cleanup(func() { cfHTTPClient = old })
	cfHTTPClient = &http.Client{Transport: cloudflareAuditTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("fixture provider failure") })}
	container := cloudflareFixtureContainer(true, "token")
	exists := false
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/images/json"):
			_, _ = w.Write([]byte(`[{}]`))
		case strings.HasSuffix(r.URL.Path, "/json"):
			if !exists {
				w.WriteHeader(404)
			} else {
				_ = json.NewEncoder(w).Encode(container)
			}
		case strings.HasSuffix(r.URL.Path, "/containers/create"):
			exists = true
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(map[string]string{"Id": container.ID})
		case r.Method == http.MethodDelete:
			exists = false
			w.WriteHeader(204)
		default:
			w.WriteHeader(204)
		}
	})
	vault, err := security.NewVault(strings.Repeat("ab", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"cloudflared_token": "fixture-connector", "cloudflare_api_token": "fixture-api"} {
		if err := vault.WriteSecret(key, value); err != nil {
			t.Fatal(err)
		}
	}
	cfg := CloudflareTunnelConfig{Enabled: true, Mode: "docker", AuthMethod: "token", DockerHost: host, DataDir: t.TempDir(), HTTPSEnabled: true, HTTPSPort: 8443, AccountID: "fixture", TunnelID: "fixture"}
	for _, operation := range []func() string{func() string { return CloudflareTunnelStart(cfg, vault, nil, slogDiscard()) }, func() string { return CloudflareTunnelRestart(cfg, vault, nil, slogDiscard()) }} {
		result := operation()
		if !cloudflareTunnelToolResultOK(result) || !strings.Contains(result, "tls_get") {
			t.Fatal(result)
		}
		status := CloudflareTunnelStatus(cfg, nil, slogDiscard())
		if !strings.Contains(status, "tls_get") || !strings.Contains(status, `"running":true`) {
			t.Fatal(status)
		}
	}
	if result := CloudflareTunnelShutdown(cfg, nil, slogDiscard(), false); !cloudflareTunnelToolResultOK(result) {
		t.Fatal(result)
	}
}

func TestCloudflareLogsPreserveUTF8Tail(t *testing.T) {
	resetCloudflareTunnelRuntimeForTest()
	t.Cleanup(resetCloudflareTunnelRuntimeForTest)
	registry := NewProcessRegistry(slogDiscard())
	info := &ProcessInfo{PID: 123, Alive: true}
	_, _ = info.Write([]byte(strings.Repeat("€", 2000)))
	registry.Register(info)
	tunnelMu.Lock()
	tunnelMode, tunnelPID = "native", 123
	tunnelMu.Unlock()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(CloudflareTunnelLogs(registry, slogDiscard())), &result); err != nil {
		t.Fatal(err)
	}
	logs, _ := result["logs"].(string)
	if !utf8.ValidString(logs) || strings.ContainsRune(logs, utf8.RuneError) || len(logs) > 4096 || logs == "" {
		t.Fatal("invalid UTF-8 log tail")
	}
}
