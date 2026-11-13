package invasion

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

// The image-contract job supplies its freshly built image. Ordinary Go tests
// never launch containers, pull images or touch a production Egg.
func TestDockerEggRuntimeBootstrapAndReconfigure(t *testing.T) {
	if os.Getenv("AURAGO_RUN_DOCKER_EGG_SMOKE") != "1" {
		t.Skip("isolated Docker image-contract opt-in required")
	}
	if runtime.GOOS != "linux" {
		t.Fatal("Docker image-contract smoke requires Linux")
	}
	image := os.Getenv("AURAGO_EGG_SMOKE_IMAGE")
	if image == "" {
		t.Fatal("the current CI image is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	key := strings.Repeat("a", 64)
	shared := strings.Repeat("c", 64)
	security.RegisterSensitive(key)
	security.RegisterSensitive(shared)
	suffix, err := GenerateSharedKey()
	if err != nil {
		t.Fatal(err)
	}
	nest := NestRecord{ID: suffix[:8] + "-isolated-ci", DeployMethod: "docker_local"}
	if err := requirePrivateDockerBootstrap(nest); err != nil {
		t.Fatal(err)
	}
	name, _ := dockerEggContainerName(nest.ID)
	docker := dockerutil.NewClient(dockerLocalHost(), 30*time.Second)
	logVolume := "aurago-egg-" + suffix[:8] + "-log"
	for _, path := range []string{"/containers/" + name + "/json", "/volumes/" + logVolume} {
		status, err := docker.DoJSON(ctx, http.MethodGet, path, nil, nil)
		if err != nil || status != http.StatusNotFound {
			t.Fatalf("refusing occupied smoke resource %s: HTTP %d (%v)", path, status, err)
		}
	}
	c := &DockerConnector{}
	master := &config.Config{Providers: []config.ProviderEntry{{ID: "fixture", Type: "ollama", BaseURL: "http://127.0.0.1:1", Model: "fixture"}}}
	egg := EggRecord{Provider: "fixture", EggPort: 8099, AllowedTools: "[]"}
	cfg, err := GenerateEggConfig(master, egg, nest, shared, "wss://127.0.0.1:1", key)
	if err != nil {
		t.Fatal(err)
	}
	body := dockerEggCreateBody(image, nest.ID, EggDeployPayload{ConfigYAML: cfg, EggPort: 8099})
	status, err := docker.DoJSON(ctx, http.MethodPost, "/containers/create?name="+name, body, nil)
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create isolated Egg: HTTP %d (%v)", status, err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		if err := c.removeContainer(cleanup, nest, nil, name); err != nil {
			t.Error(err)
		}
		_, _ = docker.DoJSON(cleanup, http.MethodDelete, "/volumes/"+logVolume, nil, nil)
	})
	fixtureDir := t.TempDir()
	vaultPath := filepath.Join(fixtureDir, "vault.bin")
	vault, err := security.NewVault(key, vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("smoke_retained", "retained-fixture"); err != nil {
		t.Fatal(err)
	}
	encrypted, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	var seeded bytes.Buffer
	tarWriter := tar.NewWriter(&seeded)
	for _, entry := range []struct {
		name      string
		data      []byte
		directory bool
	}{
		{".env", []byte("AURAGO_MASTER_KEY=" + key + "\n"), false}, {"data/", nil, true}, {"data/vault.bin", encrypted, false},
		{"data/retained-state.db", []byte("isolated-state-fixture"), false},
	} {
		header := &tar.Header{Name: entry.name, Mode: 0600, Size: int64(len(entry.data)), Uid: dockerEggConfigUID, Gid: dockerEggConfigGID}
		if entry.directory {
			header.Typeflag = tar.TypeDir
			header.Mode = 0700
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.copyConfigArchiveToContainer(ctx, nest, nil, name, seeded.Bytes()); err != nil {
		t.Fatal(err)
	}
	bootstrap := func(value string) []byte {
		security.RegisterSensitive(value)
		data, err := json.Marshal(EggBootstrap{Version: 1, Secrets: map[string]string{"egg_shared_key": shared, "smoke_changed": value}})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if err := c.copyConfigToContainer(ctx, nest, nil, name, cfg, bootstrap("first-fixture")); err != nil {
		t.Fatal(err)
	}
	status, err = docker.DoJSON(ctx, http.MethodPost, "/containers/"+name+"/start", nil, nil)
	if err != nil || status != http.StatusNoContent {
		t.Fatalf("start isolated Egg: HTTP %d (%v)", status, err)
	}
	verify := func(want string) {
		t.Helper()
		var lastErr error
		for ctx.Err() == nil {
			input, inputStatus, err := dockerEggSmokeReadFile(ctx, c, nest, name, "/app/data/data/"+EggBootstrapFile)
			clear(input)
			if err == nil && inputStatus == http.StatusNotFound {
				data, code, err := dockerEggSmokeReadFile(ctx, c, nest, name, "/app/data/data/vault.bin")
				if err == nil && code == http.StatusOK {
					if err := os.WriteFile(vaultPath, data, 0600); err != nil {
						t.Fatal(err)
					}
					read, err := security.NewVault(key, vaultPath)
					if err == nil {
						changed, _ := read.ReadSecret("smoke_changed")
						retained, _ := read.ReadSecret("smoke_retained")
						if changed == want && retained == "retained-fixture" && c.HealthCheck(ctx, nest, nil) == nil {
							return
						}
					}
					lastErr = err
				} else {
					lastErr = err
				}
			} else {
				lastErr = err
			}
			select {
			case <-ctx.Done():
			case <-time.After(500 * time.Millisecond):
			}
		}
		t.Fatalf("isolated Egg never imported its private bootstrap while retaining the test Vault: %v", lastErr)
	}
	verify("first-fixture")
	if err := c.Reconfigure(ctx, nest, nil, cfg, bootstrap("reconfigured-fixture")); err != nil {
		t.Fatal(err)
	}
	verify("reconfigured-fixture")
	env, code, err := dockerEggSmokeReadFile(ctx, c, nest, name, "/app/data/.env")
	if err != nil || code != http.StatusOK || !bytes.Contains(env, []byte(key)) {
		t.Fatal("existing test master key changed")
	}
	data, code, err := dockerEggSmokeReadFile(ctx, c, nest, name, "/app/data/data/retained-state.db")
	if err != nil || code != http.StatusOK || string(data) != "isolated-state-fixture" {
		t.Fatal("existing test state moved or changed")
	}
}

func dockerEggSmokeReadFile(ctx context.Context, c *DockerConnector, nest NestRecord, name, path string) ([]byte, int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL(nest, "/containers/"+name+"/archive?path="+url.QueryEscape(path)), nil)
	if err != nil {
		return nil, 0, err
	}
	response, err := c.httpClient(nest, nil).Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, nil
	}
	reader := tar.NewReader(io.LimitReader(response.Body, 16<<20))
	header, err := reader.Next()
	if err != nil {
		return nil, response.StatusCode, err
	}
	if header.Mode&0077 != 0 {
		return nil, response.StatusCode, fmt.Errorf("private smoke artifact has non-private permissions")
	}
	data, err := io.ReadAll(reader)
	return data, response.StatusCode, err
}
