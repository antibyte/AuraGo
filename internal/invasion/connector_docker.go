package invasion

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/remote"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

// dockerAPIVersion is the Docker Engine API version used for all requests.
// Increment when requiring features from a newer Docker Engine.
const dockerAPIVersion = dockerutil.APIVersion
const dockerEggConfigArchivePath = "/app/data"
const dockerEggConfigFileName = "config.yaml"
const dockerEggConfigUID = 1001
const dockerEggConfigGID = 1001
const dockerEggConfigUser = "aurago"
const dockerEggConfigGroup = "aurago"
const dockerEggDefaultHTTPPort = 8099

// dockerInspectBodyLimit bounds container-inspect decoding, matching
// dockerutil.Client.DoJSON.
const dockerInspectBodyLimit = 8 << 20

// dockerEggContainerName derives the egg container name from the nest ID.
// UUID nest IDs keep the historic "aurago-egg-<first 8 characters>" name, so
// existing containers, their "-prev" backups and "-log" volumes stay attached.
func dockerEggContainerName(nestID string) (string, error) {
	prefix, err := eggIDPrefix(nestID)
	if err != nil {
		return "", err
	}
	return "aurago-egg-" + prefix, nil
}

const dockerConfigAwareHealthcheckPython = `import pathlib,re,urllib.request; data=pathlib.Path('/app/data/config.yaml').read_text(encoding='utf-8', errors='ignore'); m=re.search(r'(?m)^server:\s*(?:\n[ \t]+[^\n]*)*?\n[ \t]+port:\s*[\"\']?(\d+)', data); port=int(m.group(1)) if m else 8088; urllib.request.urlopen('http://127.0.0.1:%d/api/ready' % port, timeout=5)`

// DockerConnector deploys eggs as Docker containers, either on a remote host
// or on the local Docker daemon.
type DockerConnector struct{}

func (c *DockerConnector) Validate(ctx context.Context, nest NestRecord, secret []byte) error {
	client := c.httpClient(nest, secret)
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL(nest, "/version"), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("docker API unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body := dockerutil.ReadErrorBody(resp.Body)
		return fmt.Errorf("docker API returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *DockerConnector) Deploy(ctx context.Context, nest NestRecord, secret []byte, payload EggDeployPayload) error {
	containerName, err := dockerEggContainerName(nest.ID)
	if err != nil {
		return configNotDelivered(err)
	}
	backupName := containerName + "-prev"
	// TODO: derive image tag from master version when build version is available at runtime
	image := "ghcr.io/antibyte/aurago:latest"

	// Every return before step 4 (copyConfigToContainer) is marked
	// configNotDelivered: the new egg configuration, which carries the hatch's
	// shared key, has not left the master, so the hatch puts the previous key
	// back. From step 4 on, a lost response does not prove that the Engine
	// did not store or start the new configuration; those failures stay unmarked.

	// 1. Pull image. A pull that fails with an HTTP error status returns here,
	// before step 2 stops and renames the running egg.
	//
	// A failure inside the HTTP 200 progress stream (error event, truncated
	// stream, read error) counted as success before the K15 hardening, and
	// Deploy went on with the image the Engine already held under this tag,
	// the normal case on a redeploy. That behaviour is kept when the Engine has
	// the image, so redeploys keep working when only the progress stream broke.
	// Without the image (audit S7a), or when the image check itself
	// fails, the deploy stops here instead of renaming the running egg and then
	// failing to create its replacement.
	if err := c.pullImage(ctx, nest, secret, image); err != nil {
		var streamErr *dockerPullStreamError
		if !errors.As(err, &streamErr) {
			return configNotDelivered(fmt.Errorf("failed to pull image: %w", err))
		}
		present, checkErr := c.imagePresent(ctx, nest, secret, image)
		if checkErr != nil {
			return configNotDelivered(fmt.Errorf("failed to pull image: %w (checking for the image on the Engine also failed: %v)", err, checkErr))
		}
		if !present {
			return configNotDelivered(fmt.Errorf("failed to pull image: %w", err))
		}
		slog.Warn("Invasion image pull failed; deploying the image already on the Engine", "nest_id", nest.ID, "image", image, "error", err)
	}

	// 2. Remove any stale backup, then rename current container as backup
	_ = c.removeContainer(ctx, nest, secret, backupName)
	_ = c.renameContainer(ctx, nest, secret, containerName, backupName)

	// 3. Create container with minimal env vars.
	// The full configuration (including secrets like the shared key and API keys)
	// is copied into the container via the archive API in step 4, avoiding
	// exposure via "docker inspect" which displays environment variables.
	createBody := dockerEggCreateBody(image, nest.ID, payload)

	bodyJSON, _ := json.Marshal(createBody)
	createURL := c.apiURL(nest, fmt.Sprintf("/containers/create?name=%s", containerName))
	req, err := http.NewRequestWithContext(ctx, "POST", createURL, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return configNotDelivered(fmt.Errorf("failed to create request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")

	client := c.httpClient(nest, secret)
	resp, err := client.Do(req)
	if err != nil {
		return configNotDelivered(fmt.Errorf("failed to create container: %w", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body := dockerutil.ReadErrorBody(resp.Body)
		return configNotDelivered(fmt.Errorf("container creation failed (%d): %s", resp.StatusCode, string(body)))
	}

	// 4. Copy config.yaml into the container via the Docker Archive API.
	// This ensures secrets (shared key, API keys) are not visible in "docker inspect".
	if err := c.copyConfigToContainer(ctx, nest, secret, containerName, payload.ConfigYAML); err != nil {
		// Clean up the created container on failure
		_ = c.removeContainer(ctx, nest, secret, containerName)
		return fmt.Errorf("failed to copy config to container: %w", err)
	}

	// 5. Start container
	startURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/start", containerName))
	startReq, err := http.NewRequestWithContext(ctx, "POST", startURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create start request: %w", err)
	}
	startResp, err := client.Do(startReq)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusNoContent && startResp.StatusCode != http.StatusOK {
		body := dockerutil.ReadErrorBody(startResp.Body)
		return fmt.Errorf("container start failed (%d): %s", startResp.StatusCode, string(body))
	}

	return nil
}

func dockerEggCreateBody(image, nestID string, payload EggDeployPayload) map[string]interface{} {
	eggPort := dockerEggHTTPPort(payload)
	envVars := []string{
		"AURAGO_EGG_MODE=true",
		"AURAGO_SERVER_HOST=0.0.0.0",
	}
	return map[string]interface{}{
		"Image": image,
		"Env":   envVars,
		"ExposedPorts": map[string]interface{}{
			fmt.Sprintf("%d/tcp", eggPort): map[string]interface{}{},
			"8089/tcp":                     map[string]interface{}{},
		},
		"Healthcheck": map[string]interface{}{
			"Test": []string{"CMD", "python3", "-c", dockerConfigAwareHealthcheckPython},
		},
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{
				"Name": "unless-stopped",
			},
			"Binds": dockerEggBinds(nestID),
			// Allow the egg to reach the master via host.docker.internal.
			// On Linux Docker Engine this is not injected automatically;
			// host-gateway resolves to the Docker bridge gateway (typically 172.17.0.1).
			// Safe no-op on Docker Desktop (Windows/Mac) where the name already resolves.
			"ExtraHosts": []string{"host.docker.internal:host-gateway"},
			// Security hardening — mirror the main AuraGo container's security profile.
			"SecurityOpt": []string{"no-new-privileges:true"},
			"CapDrop":     []string{"ALL"},
		},
	}
}

func dockerEggBinds(nestID string) []string {
	shortID, err := eggIDPrefix(nestID)
	if err != nil {
		// Deploy validated the ID through dockerEggContainerName, so this is
		// only reached by other callers; keep the historic slice for them.
		shortID = nestID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
	}
	return []string{
		fmt.Sprintf("aurago-egg-%s-log:/app/log", shortID),
	}
}

func dockerEggHTTPPort(payload EggDeployPayload) int {
	if payload.EggPort > 0 {
		return payload.EggPort
	}
	if port := extractServerPort(payload.ConfigYAML); port > 0 {
		return port
	}
	return dockerEggDefaultHTTPPort
}

func extractServerPort(cfgYAML []byte) int {
	var raw struct {
		Server struct {
			Port int `yaml:"port"`
		} `yaml:"server"`
	}
	if err := yaml.Unmarshal(cfgYAML, &raw); err != nil {
		return 0
	}
	return raw.Server.Port
}

// copyConfigToContainer copies the egg config YAML into a container via the
// Docker Engine Archive API (PUT /containers/{id}/archive). The config is
// written to /app/data/config.yaml with mode 0600 (owner read/write only).
func (c *DockerConnector) copyConfigToContainer(ctx context.Context, nest NestRecord, secret []byte, containerName string, configYAML []byte) error {
	archive, err := buildDockerEggConfigArchive(configYAML)
	if err != nil {
		return err
	}
	buf := bytes.NewReader(archive)

	// Upload to the persisted config path read by docker-entrypoint.sh.
	archiveURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/archive?path=%s", containerName, dockerEggConfigArchivePath))
	req, err := http.NewRequestWithContext(ctx, "PUT", archiveURL, buf)
	if err != nil {
		return fmt.Errorf("failed to create archive request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-tar")

	uploadClient := c.httpClient(nest, secret)
	resp, err := uploadClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload config: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := dockerutil.ReadErrorBody(resp.Body)
		return fmt.Errorf("config upload failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func buildDockerEggConfigArchive(configYAML []byte) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name:  dockerEggConfigFileName,
		Mode:  0600,
		Uid:   dockerEggConfigUID,
		Gid:   dockerEggConfigGID,
		Uname: dockerEggConfigUser,
		Gname: dockerEggConfigGroup,
		Size:  int64(len(configYAML)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, fmt.Errorf("failed to write tar header: %w", err)
	}
	if _, err := tw.Write(configYAML); err != nil {
		return nil, fmt.Errorf("failed to write config to tar: %w", err)
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar archive: %w", err)
	}
	return buf.Bytes(), nil
}

func (c *DockerConnector) Stop(ctx context.Context, nest NestRecord, secret []byte) error {
	containerName, err := dockerEggContainerName(nest.ID)
	if err != nil {
		return err
	}
	client := c.httpClient(nest, secret)

	// Stop container
	stopURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/stop?t=10", containerName))
	req, err := http.NewRequestWithContext(ctx, "POST", stopURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create stop request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	defer resp.Body.Close()
	// 204 = stopped, 304 = already stopped are both acceptable
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		body := dockerutil.ReadErrorBody(resp.Body)
		return fmt.Errorf("stop container failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *DockerConnector) Status(ctx context.Context, nest NestRecord, secret []byte) (string, error) {
	containerName, err := dockerEggContainerName(nest.ID)
	if err != nil {
		return "unknown", err
	}
	client := c.httpClient(nest, secret)

	inspectURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/json", containerName))
	req, err := http.NewRequestWithContext(ctx, "GET", inspectURL, nil)
	if err != nil {
		return "unknown", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "unknown", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "stopped", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "unknown", fmt.Errorf("inspect failed with status %d", resp.StatusCode)
	}

	var info struct {
		State struct {
			Status  string `json:"Status"`
			Running bool   `json:"Running"`
		} `json:"State"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, dockerInspectBodyLimit)).Decode(&info); err != nil {
		return "unknown", err
	}

	if info.State.Running {
		return "running", nil
	}
	return "stopped", nil
}

// httpClient returns the Engine client for one request. secret is the
// operation's transport credential: the Docker TLS material (JSON) for an
// encrypted docker_remote nest, the SSH key or password for docker_ssh;
// plain docker_remote and docker_local ignore it.
func (c *DockerConnector) httpClient(nest NestRecord, secret []byte) *http.Client {
	isLocal := nest.DeployMethod == "docker_local"
	if isLocal {
		dockerHost := dockerLocalHost()
		return &http.Client{
			Timeout:   30 * time.Second,
			Transport: dockerutil.NewVersionTransport(dockerLocalTransport(dockerHost)),
		}
	}
	if nest.DeployMethod == "docker_ssh" {
		// One SSH connection per Engine connection. Keep-alives are off so every
		// connection, and with it its SSH client, closes after its response.
		// The version probe's first dial includes the SSH login, so it gets
		// dockerSSHProbeTimeout instead of the default probe budget.
		return &http.Client{
			Timeout: 30 * time.Second,
			Transport: dockerutil.NewVersionTransportWithProbeTimeout(&http.Transport{
				DisableKeepAlives: true,
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialDockerEngineOverSSH(ctx, nest, secret)
				},
			}, dockerSSHProbeTimeout),
		}
	}
	if DockerRemoteUsesTLS(nest) {
		return &http.Client{Timeout: 30 * time.Second, Transport: dockerutil.NewVersionTransport(dockerRemoteTLSTransport(nest, secret))}
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: dockerutil.NewVersionTransport(http.DefaultTransport)}
}

// dockerRemoteTLSIdleConnTimeout bounds how long a TLS nest transport keeps
// idle connections. httpClient builds a new transport for every operation, so
// nothing reuses them later; a short timeout closes them and their goroutines
// soon after the operation instead of after the default 90 seconds.
const dockerRemoteTLSIdleConnTimeout = 5 * time.Second

// dockerRemoteTLSTransport clones the default transport (keeping proxy-from-
// environment exactly like plain docker_remote; CONNECT keeps TLS end to end)
// and adds the nest's TLS settings. Unusable material yields a transport that
// fails every request, so a TLS nest never falls back to plain HTTP.
func dockerRemoteTLSTransport(nest NestRecord, secret []byte) http.RoundTripper {
	var material DockerTLSMaterial
	if len(bytes.TrimSpace(secret)) > 0 {
		if err := json.Unmarshal(secret, &material); err != nil {
			return failingDockerTransport{err: fmt.Errorf("docker TLS material for nest %s is unreadable", nest.ID)}
		}
	}
	cfg, err := dockerTLSClientConfig(nest.DockerTLS, material)
	if err != nil {
		return failingDockerTransport{err: err}
	}
	var base *http.Transport
	if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
		base = defaultTransport.Clone()
	} else {
		base = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	base.TLSClientConfig = cfg
	base.IdleConnTimeout = dockerRemoteTLSIdleConnTimeout
	return base
}

// failingDockerTransport refuses every request with a fixed error.
type failingDockerTransport struct{ err error }

func (t failingDockerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
	return nil, t.err
}

// dockerSSHDialBudget is remote.DialSSH's dial and handshake budget
// (GetSSHConfig's ClientConfig.Timeout); a test keeps both equal.
const dockerSSHDialBudget = 10 * time.Second

// dockerSSHProbeTimeout bounds the Engine version probe for docker_ssh. Its
// first dial includes a full SSH login, so it covers the SSH dial budget and
// leaves 10 s for the socket open and /version (other transports: 5 s total).
const dockerSSHProbeTimeout = 20 * time.Second

// dockerSSHEngineSocket is the Engine socket a docker_ssh nest reaches through SSH.
const dockerSSHEngineSocket = "/var/run/docker.sock"

// dockerSSHSocketOpenBudget bounds the direct-streamlocal open of the Engine
// socket when no version probe precedes it, and after the probe gave up:
// net/http detaches the dial from the request, so without it an authenticated
// sshd that never answers the open would keep the SSH client and its
// goroutines alive forever. The 20 s version probe (dockerSSHProbeTimeout)
// usually ends first.
const dockerSSHSocketOpenBudget = 10 * time.Second

// dockerSSHSocketOpenTimeout is the budget in use; tests shorten it.
var dockerSSHSocketOpenTimeout = dockerSSHSocketOpenBudget

// dialDockerEngineOverSSH opens one SSH connection (known_hosts verification
// and finite dial/handshake budget via remote.DialSSH) and forwards a stream
// to the remote Engine socket. Closing the returned conn closes the SSH client.
func dialDockerEngineOverSSH(ctx context.Context, nest NestRecord, secret []byte) (net.Conn, error) {
	port := nest.Port
	if port <= 0 {
		port = 22
	}
	client, err := remote.DialSSH(ctx, nest.Host, port, nest.Username, secret)
	if err != nil {
		return nil, fmt.Errorf("docker over SSH: %w", err)
	}
	openCtx, cancel := context.WithTimeout(ctx, dockerSSHSocketOpenTimeout)
	defer cancel()
	conn, err := client.DialContext(openCtx, "unix", dockerSSHEngineSocket)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("docker over SSH: open %s: %w", dockerSSHEngineSocket, err)
	}
	return &sshTunnelConn{Conn: conn, client: client}, nil
}

// sshTunnelConn is a forwarded Engine stream that owns its SSH client.
type sshTunnelConn struct {
	net.Conn
	client *ssh.Client
	once   sync.Once
}

func (c *sshTunnelConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { _ = c.client.Close() })
	return err
}

func dockerLocalHost() string {
	if dh := strings.TrimSpace(os.Getenv("DOCKER_HOST")); dh != "" {
		return dh
	}
	return dockerutil.DefaultHost()
}

// dockerLocalIdleConnTimeout bounds idle connections of the docker_local
// transport. httpClient builds a new transport for every operation, so
// nothing reuses them; without a timeout they were never reaped.
const dockerLocalIdleConnTimeout = 5 * time.Second

func dockerLocalTransport(dockerHost string) *http.Transport {
	return &http.Transport{
		IdleConnTimeout: dockerLocalIdleConnTimeout,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dockerutil.DialContext(ctx, dockerHost)
		},
	}
}

func (c *DockerConnector) apiURL(nest NestRecord, path string) string {
	switch nest.DeployMethod {
	case "docker_local", "docker_ssh":
		// The Engine is reached through a dialled socket; "localhost" is only
		// the request's Host header.
		return fmt.Sprintf("http://localhost/%s%s", dockerAPIVersion, path)
	}
	scheme, port := "http", nest.Port
	if DockerRemoteUsesTLS(nest) {
		scheme = "https"
		if port == 0 {
			port = 2376
		}
	}
	if port == 0 {
		port = 2375
	}
	host := nest.Host
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1] // a bracketed IPv6 literal keeps working
	}
	// JoinHostPort brackets only hosts with a colon, so IPv4 addresses and
	// host names give exactly the URL they gave before.
	return fmt.Sprintf("%s://%s/%s%s", scheme, net.JoinHostPort(host, strconv.Itoa(port)), dockerAPIVersion, path)
}

// pullClient returns an HTTP client with an extended timeout suitable for
// image pull operations, which can take minutes on slow connections.
func (c *DockerConnector) pullClient(nest NestRecord, secret []byte) *http.Client {
	base := c.httpClient(nest, secret)
	return &http.Client{
		Timeout:   10 * time.Minute,
		Transport: base.Transport,
	}
}

func (c *DockerConnector) pullImage(ctx context.Context, nest NestRecord, secret []byte, image string) error {
	client := c.pullClient(nest, secret)
	pullURL := c.apiURL(nest, "/images/create?fromImage="+url.QueryEscape(image))
	req, err := http.NewRequestWithContext(ctx, "POST", pullURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pull request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body := dockerutil.ReadErrorBody(resp.Body)
		return fmt.Errorf("pull failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	// Registry resolution errors (unknown tag, auth, rate limit, DNS) arrive as
	// an HTTP error status above. Failures after the Engine started streaming
	// (layer download, verification, extraction, disk full, platform mismatch
	// on the graphdriver store) arrive as error events inside the HTTP 200 stream.
	if err := dockerutil.DrainJSONMessages(resp.Body); err != nil {
		return &dockerPullStreamError{image: image, err: err}
	}
	return nil
}

// dockerPullStreamError is a pull failure reported inside the HTTP 200
// progress stream: an error event, a truncated stream or a read error. Deploy
// tells it apart from an HTTP error status to keep using an image the Engine
// already holds.
type dockerPullStreamError struct {
	image string
	err   error
}

func (e *dockerPullStreamError) Error() string {
	return fmt.Sprintf("pull %s: %v", e.image, e.err)
}

func (e *dockerPullStreamError) Unwrap() error {
	return e.err
}

// imagePresent reports whether the Engine holds image. The reference stays
// unescaped: the Engine route is /images/{name:.*}/json, as the Docker CLI
// sends it.
func (c *DockerConnector) imagePresent(ctx context.Context, nest NestRecord, secret []byte, image string) (bool, error) {
	client := c.httpClient(nest, secret)
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL(nest, "/images/"+image+"/json"), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create image inspect request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("image inspect request failed: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		body := dockerutil.ReadErrorBody(resp.Body)
		return false, fmt.Errorf("image inspect failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
}

func (c *DockerConnector) removeContainer(ctx context.Context, nest NestRecord, secret []byte, name string) error {
	client := c.httpClient(nest, secret)
	removeURL := c.apiURL(nest, dockerRemoveContainerPath(name))
	req, err := http.NewRequestWithContext(ctx, "DELETE", removeURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("remove container request failed: %w", err)
	}
	defer resp.Body.Close()
	// 204 = deleted, 404 = not found (already gone) are both acceptable
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body := dockerutil.ReadErrorBody(resp.Body)
		return fmt.Errorf("remove container failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func dockerRemoveContainerPath(name string) string {
	return fmt.Sprintf("/containers/%s?force=true&v=true", name)
}

func (c *DockerConnector) renameContainer(ctx context.Context, nest NestRecord, secret []byte, oldName, newName string) error {
	client := c.httpClient(nest, secret)
	// Stop the container first so it can be renamed cleanly. The stop is best
	// effort: a request that cannot be built or sent skips it, and the rename
	// below still runs.
	stopURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/stop?t=5", oldName))
	if stopReq, err := http.NewRequestWithContext(ctx, "POST", stopURL, nil); err == nil {
		if resp, err := client.Do(stopReq); err == nil {
			resp.Body.Close()
		}
	}

	renameURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/rename?name=%s", oldName, newName))
	req, err := http.NewRequestWithContext(ctx, "POST", renameURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("rename container failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body := dockerutil.ReadErrorBody(resp.Body)
		return fmt.Errorf("rename container failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *DockerConnector) HealthCheck(ctx context.Context, nest NestRecord, secret []byte) error {
	containerName, err := dockerEggContainerName(nest.ID)
	if err != nil {
		return err
	}
	client := c.httpClient(nest, secret)

	inspectURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/json", containerName))
	req, err := http.NewRequestWithContext(ctx, "GET", inspectURL, nil)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("container not found")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("inspect failed with status %d", resp.StatusCode)
	}

	var info struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, dockerInspectBodyLimit)).Decode(&info); err != nil {
		return fmt.Errorf("failed to decode container state: %w", err)
	}
	if !info.State.Running {
		return fmt.Errorf("container is not running")
	}
	return nil
}

// Reconfigure writes a patched config.yaml into the running egg container and restarts it.
// The container is stopped, the config is replaced via the archive API, then restarted.
func (c *DockerConnector) Reconfigure(ctx context.Context, nest NestRecord, secret []byte, configYAML []byte) error {
	containerName, err := dockerEggContainerName(nest.ID)
	if err != nil {
		return err
	}

	// 1. Stop the container
	if err := c.Stop(ctx, nest, secret); err != nil {
		return fmt.Errorf("failed to stop container for reconfigure: %w", err)
	}

	// 2. Copy the patched config into the container
	if err := c.copyConfigToContainer(ctx, nest, secret, containerName, configYAML); err != nil {
		return fmt.Errorf("failed to copy patched config to container: %w", err)
	}

	// 3. Start the container
	client := c.httpClient(nest, secret)
	startURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/start", containerName))
	startReq, err := http.NewRequestWithContext(ctx, "POST", startURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create start request: %w", err)
	}
	startResp, err := client.Do(startReq)
	if err != nil {
		return fmt.Errorf("failed to start container after reconfigure: %w", err)
	}
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusNoContent && startResp.StatusCode != http.StatusOK && startResp.StatusCode != http.StatusNotModified {
		body := dockerutil.ReadErrorBody(startResp.Body)
		return fmt.Errorf("container start failed after reconfigure (%d): %s", startResp.StatusCode, string(body))
	}

	return nil
}

func (c *DockerConnector) Rollback(ctx context.Context, nest NestRecord, secret []byte) error {
	containerName, err := dockerEggContainerName(nest.ID)
	if err != nil {
		return err
	}
	backupName := containerName + "-prev"

	// Check if backup container exists
	client := c.httpClient(nest, secret)
	checkURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/json", backupName))
	checkReq, err := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create backup check request: %w", err)
	}
	resp, err := client.Do(checkReq)
	if err != nil {
		return fmt.Errorf("failed to check backup container: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("no backup container found for rollback")
	}

	// Remove the failed new container
	_ = c.removeContainer(ctx, nest, secret, containerName)

	// Rename backup back to primary name
	if err := c.renameContainer(ctx, nest, secret, backupName, containerName); err != nil {
		return fmt.Errorf("failed to restore backup container: %w", err)
	}

	// Start the restored container
	startURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/start", containerName))
	startReq, err := http.NewRequestWithContext(ctx, "POST", startURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create start request for restored container: %w", err)
	}
	startResp, err := client.Do(startReq)
	if err != nil {
		return fmt.Errorf("failed to start restored container: %w", err)
	}
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusNoContent && startResp.StatusCode != http.StatusOK && startResp.StatusCode != http.StatusNotModified {
		body := dockerutil.ReadErrorBody(startResp.Body)
		return fmt.Errorf("failed to start restored container (%d): %s", startResp.StatusCode, string(body))
	}

	return nil
}

// extractMasterURL extracts egg_mode.master_url from YAML config bytes.
func extractMasterURL(cfgYAML []byte) string {
	return extractYAMLField(cfgYAML, "master_url")
}

// extractField extracts a field value from YAML config bytes.
func extractField(cfgYAML []byte, field string) string {
	return extractYAMLField(cfgYAML, field)
}

func extractYAMLField(data []byte, field string) string {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ""
	}
	// Check top-level keys first
	if v, ok := raw[field]; ok {
		return fmt.Sprint(v)
	}
	// Check one level deep (e.g. egg_mode.master_url)
	for _, section := range raw {
		if m, ok := section.(map[string]interface{}); ok {
			if v, ok := m[field]; ok {
				return fmt.Sprint(v)
			}
		}
	}
	return ""
}

// GetConnector returns the appropriate NestConnector for the given nest.
// docker_ssh is listed explicitly: the default branch maps every unknown
// method to the SSH binary deploy, which is what an older AuraGo does with a
// docker_ssh nest after a downgrade.
func GetConnector(nest NestRecord) NestConnector {
	switch nest.DeployMethod {
	case "docker_remote", "docker_local", "docker_ssh":
		return &DockerConnector{}
	default: // "ssh" and everything else
		return &SSHConnector{}
	}
}

// DockerRemotePlaintext reports whether a nest deploys through the remote
// Docker Engine API over unencrypted HTTP. Hatch and reconfigure then send
// the egg configuration, including its secrets, in clear text.
func DockerRemotePlaintext(nest NestRecord) bool {
	return nest.DeployMethod == "docker_remote" && !DockerRemoteUsesTLS(nest)
}
