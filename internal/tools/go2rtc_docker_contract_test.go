package tools

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

// TestGo2RTCRealImageContract is opt-in because it pulls and runs the pinned
// upstream image. Enable with AURAGO_GO2RTC_DOCKER_TEST=1 on a Docker host.
func TestGo2RTCRealImageContract(t *testing.T) {
	if os.Getenv("AURAGO_GO2RTC_DOCKER_TEST") != "1" {
		t.Skip("set AURAGO_GO2RTC_DOCKER_TEST=1 to run the real-image contract")
	}
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Fatalf("Docker is required for guarded go2rtc contract: %v", err)
	}

	jpegData := syntheticJPEG(t)
	cameraListener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("listen synthetic camera: %v", err)
	}
	cameraPort := cameraListener.Addr().(*net.TCPAddr).Port
	cameraServer := &http.Server{Handler: syntheticMJPEGHandler(jpegData)}
	go func() { _ = cameraServer.Serve(cameraListener) }()
	t.Cleanup(func() {
		_ = cameraServer.Shutdown(context.Background())
	})

	configDir := t.TempDir()
	if err := os.Chmod(configDir, 0o755); err != nil {
		t.Fatalf("chmod config dir: %v", err)
	}
	const password = "aurago-contract-password"
	rendered := renderGo2RTCConfig(config.Go2RTCConfig{})
	if err := os.WriteFile(configDir+"/go2rtc.yaml", rendered, 0o644); err != nil {
		t.Fatalf("write go2rtc config: %v", err)
	}
	containerName := "aurago-go2rtc-contract-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	run := exec.CommandContext(ctx, "docker", "run", "-d", "--rm",
		"--name", containerName,
		"--user", "65532:65532",
		"--read-only",
		"--security-opt", "no-new-privileges:true",
		"--cap-drop", "ALL",
		"--tmpfs", "/tmp:rw,noexec,nosuid,size=64m,mode=1777",
		"--pids-limit", "100",
		"--memory", "256m",
		"--cpus", "0.5",
		"--add-host", "host.docker.internal:host-gateway",
		"-e", "AURAGO_GO2RTC_API_PASSWORD="+password,
		"-v", configDir+":/config:ro",
		"-p", "127.0.0.1::1984",
		config.Go2RTCDefaultImage,
	)
	if output, err := run.CombinedOutput(); err != nil {
		t.Fatalf("start pinned go2rtc image: %v\n%s", err, output)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	portOutput, err := exec.CommandContext(ctx, "docker", "port", containerName, "1984/tcp").Output()
	if err != nil {
		t.Fatalf("resolve go2rtc port: %v", err)
	}
	_, portText, err := net.SplitHostPort(strings.TrimSpace(string(portOutput)))
	if err != nil {
		t.Fatalf("parse go2rtc port mapping %q: %v", portOutput, err)
	}
	baseURL := "http://127.0.0.1:" + portText + "/api/go2rtc/proxy"
	client := &http.Client{Timeout: 5 * time.Second}
	if err := waitForGo2RTCContractAPI(ctx, client, baseURL, password); err != nil {
		t.Fatal(err)
	}

	source := fmt.Sprintf("http://host.docker.internal:%d/camera.mjpeg", cameraPort)
	patchURL := baseURL + "/api/streams?" + url.Values{
		"name": {"aurago_synthetic"},
		"src":  {source},
	}.Encode()
	if _, err := go2RTCContractRequest(ctx, client, http.MethodPatch, patchURL, password); err != nil {
		t.Fatalf("runtime stream reconcile: %v", err)
	}
	snapshotURL := baseURL + "/api/frame.jpeg?src=aurago_synthetic"
	var snapshot []byte
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, err = go2RTCContractRequest(ctx, client, http.MethodGet, snapshotURL, password)
		if err == nil && len(snapshot) >= 4 && snapshot[0] == 0xff && snapshot[1] == 0xd8 {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("snapshot contract failed: %v", err)
}

func syntheticJPEG(t *testing.T) []byte {
	t.Helper()
	frame := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			frame.Set(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 10), B: 80, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, frame, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode synthetic JPEG: %v", err)
	}
	return encoded.Bytes()
}

func syntheticMJPEGHandler(frame []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/camera.mjpeg" {
			http.NotFound(w, r)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			if _, err := fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame)); err != nil {
				return
			}
			if _, err := w.Write(frame); err != nil {
				return
			}
			if _, err := io.WriteString(w, "\r\n"); err != nil {
				return
			}
			flusher.Flush()
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
			}
		}
	})
}

func waitForGo2RTCContractAPI(ctx context.Context, client *http.Client, baseURL, password string) error {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		if _, err := go2RTCContractRequest(ctx, client, http.MethodGet, baseURL+"/api", password); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("pinned go2rtc API did not become ready: %w", lastErr)
}

func go2RTCContractRequest(ctx context.Context, client *http.Client, method, endpoint, password string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(go2RTCAPIUser, password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, go2RTCMaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}
