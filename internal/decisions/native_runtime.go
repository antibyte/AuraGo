package decisions

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/onnxrt"
)

type managedRuntimeFactory struct {
	options managerOptions
}

func newManagedRuntimeFactory(options managerOptions) runtimeFactory {
	return &managedRuntimeFactory{options: options}
}

func (factory *managedRuntimeFactory) install(ctx context.Context, deployment string, access RuntimeAccess, dataDir string, cpuAllowed bool, progress func(string, int64, int64)) error {
	if deployment == config.DecisionsDeploymentDocker {
		if !dockerMayMutate(access) {
			return NewError(CodeDeploymentDenied, nil)
		}
		if err := preflightDockerMounts(ctx, access, dataDir, factory.options.dockerSelfContainer); err != nil {
			return err
		}
		return ensureDockerRuntimeImages(ctx, access, cpuAllowed, progress)
	}
	return ensureNativeRuntime(ctx, runtimeBuildDirectory(factory.options), filepath.Join(dataDir, "decisions", "runtime"), cpuAllowed, progress)
}

func (factory *managedRuntimeFactory) discover(ctx context.Context, deployment string, access RuntimeAccess, paths modelPaths, dataDir string) ([]runtimeDevice, error) {
	if deployment == config.DecisionsDeploymentDocker {
		return discoverDockerDevices(ctx, access, paths, factory.options.dockerSelfContainer)
	}
	managedRoot := filepath.Join(dataDir, "decisions", "runtime")
	return discoverNativeDevices(ctx, managedRoot, runtimeBuildDirectory(factory.options), paths.Model)
}

func (factory *managedRuntimeFactory) start(ctx context.Context, deployment string, device runtimeDevice, paths modelPaths, cfg config.DecisionsConfig, access RuntimeAccess, dataDir string, progress func(string)) (runtimeSession, error) {
	if deployment == config.DecisionsDeploymentDocker {
		return startDockerRuntime(ctx, device, paths, cfg, access, dataDir, factory.options.dockerSelfContainer, progress)
	}
	return startNativeRuntime(ctx, device, paths, cfg, dataDir, progress)
}

func discoverNativeDevices(ctx context.Context, managedRoot, sourceRoot, modelPath string) ([]runtimeDevice, error) {
	if err := ctx.Err(); err != nil {
		return nil, NewError(CodeCancelled, err)
	}
	devices := make([]runtimeDevice, 0)
	checked := false
	executed := false
	var integrityErr error
	for _, backend := range runtimeBackends(runtime.GOOS) {
		candidates, err := nativeRuntimeCandidates(ctx, sourceRoot, managedRoot, backend)
		if ErrorCode(err) == CodeCancelled {
			return nil, err
		}
		if ErrorCode(err) == CodeRuntimeAttestationFailed {
			integrityErr = err
		}
		for _, bundle := range candidates {
			directory, binary, digest := bundle.directory, bundle.binary, bundle.digest
			checked = true
			args := []string{"--discover"}
			if modelPath != "" {
				args = append(args, "--model", modelPath)
			}
			output, err := runNativeDiscovery(ctx, binary, directory, args)
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, NewError(CodeCancelled, ctxErr)
			}
			if ErrorCode(err) == CodeRuntimeDiscoveryOutputLimit {
				return nil, err
			}
			if err != nil {
				continue
			}
			var result struct {
				Protocol string `json:"protocol"`
				Build    struct {
					LlamaCPPCommit      string `json:"llama_cpp_commit"`
					LlamaCPPPatchSHA256 string `json:"llama_cpp_patch_sha256"`
					Backend             string `json:"backend"`
					BinarySHA256        string `json:"binary_sha256"`
				} `json:"build"`
				Devices     []runtimeDevice `json:"devices"`
				ModelLayers int             `json:"model_layers"`
			}
			if json.Unmarshal(output, &result) != nil || result.Protocol != runtimeProtocol || result.Build.LlamaCPPCommit != llamaCPPCommit ||
				result.Build.LlamaCPPPatchSHA256 != llamaCPPPatchSHA256 ||
				!strings.EqualFold(result.Build.Backend, backend) || !strings.EqualFold(result.Build.BinarySHA256, digest) {
				continue
			}
			executed = true
			for _, device := range result.Devices {
				if device.ID == "" || device.Name == "" || !strings.EqualFold(device.Backend, backend) {
					continue
				}
				device.BinaryPath, device.BinarySHA, device.ModelLayers = binary, digest, result.ModelLayers
				if strings.EqualFold(device.Type, "IGPU") {
					device.Type = "GPU"
					if device.Integrated == nil {
						integrated := true
						device.Integrated = &integrated
					}
				}
				devices = append(devices, device)
			}
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, NewError(CodeCancelled, err)
	}
	if len(devices) == 0 && !checked {
		if integrityErr != nil {
			return nil, integrityErr
		}
		return nil, NewError(CodeRuntimeArtifactMissing, fmt.Errorf("no verified native runtime bundles are installed"))
	}
	if checked && !executed {
		return nil, NewError("runtime_dependencies_missing", nil)
	}
	return devices, nil
}

func startNativeRuntime(ctx context.Context, device runtimeDevice, paths modelPaths, cfg config.DecisionsConfig, dataDir string, progress func(string)) (runtimeSession, error) {
	if device.BinaryPath == "" || device.BinarySHA == "" {
		return nil, NewError(CodeRuntimeArtifactMissing, nil)
	}
	port, err := availableLoopbackPort()
	if err != nil {
		return nil, NewError(CodeRuntimeStartFailed, err)
	}
	token, err := writeRuntimeToken(filepath.Join(dataDir, "decisions"))
	if err != nil {
		return nil, NewError(CodeRuntimeStartFailed, err)
	}
	args := []string{
		"--model", paths.Model, "--model-revision", modelRevision, "--model-sha256", DefaultManifest().Model.SHA256,
		"--mmproj", paths.Projector, "--mmproj-revision", modelRevision, "--mmproj-sha256", DefaultManifest().Projector.SHA256,
		"--bind", "127.0.0.1", "--port", fmt.Sprint(port), "--token-file", token,
	}
	if strings.EqualFold(device.Type, "CPU") {
		if !cfg.CPUFallbackAllowed {
			_ = os.Remove(token)
			return nil, NewError(CodeCPUConsentRequired, nil)
		}
		args = append(args, "--allow-cpu")
	} else {
		args = append(args, "--device", device.ID)
		if device.GPULayers > 0 {
			if !cfg.CPUFallbackAllowed {
				_ = os.Remove(token)
				return nil, NewError(CodeCPUConsentRequired, nil)
			}
			args = append(args, "--allow-cpu-offload", "--gpu-layers", fmt.Sprint(device.GPULayers))
		}
	}
	commandCtx, cancel := context.WithCancel(ctx)
	command := exec.CommandContext(commandCtx, device.BinaryPath, args...)
	onnxrt.ConfigureWorkerCommand(command, filepath.Dir(device.BinaryPath))
	configureHiddenProcess(command)
	command.Stdout = io.Discard
	startupStderr := &nativeStartupStderr{}
	command.Stderr = startupStderr
	cleanup, err := startRuntimeCommand(command)
	if err != nil {
		cancel()
		_ = os.Remove(token)
		return nil, NewError(CodeRuntimeStartFailed, err)
	}
	session := &nativeRuntimeSession{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port), token: token,
		client:  &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		command: command, cancel: cancel, wait: make(chan error, 1), device: device,
		cpuAllowed: cfg.CPUFallbackAllowed, deployment: config.DecisionsDeploymentNative,
	}
	go func() { err := command.Wait(); cleanup(); session.wait <- err; close(session.wait) }()
	err = waitRuntimeReady(ctx, runtimeStartupTimeout, func(probeCtx context.Context) (bool, error) {
		if phase := startupStderr.progress(); progress != nil && phase != "" {
			progress(phase)
		}
		select {
		case <-session.wait:
			return false, runtimeStartupError(startupStderr.code())
		default:
		}
		metadata, err := session.metadata(probeCtx)
		if err == nil && metadata.Ready {
			session.cached = metadata
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		if closeErr := session.Close(context.Background()); closeErr != nil {
			return session, err
		}
		return nil, err
	}
	return session, nil
}

type nativeRuntimeSession struct {
	baseURL    string
	token      string
	client     *http.Client
	command    *exec.Cmd
	cancel     context.CancelFunc
	wait       chan error
	device     runtimeDevice
	cached     runtimeMetadata
	deployment string
	cpuAllowed bool
}

func (session *nativeRuntimeSession) metadata(ctx context.Context) (runtimeMetadata, error) {
	metadata, err := session.getMetadata(ctx, "/capabilities")
	if err != nil {
		return runtimeMetadata{}, err
	}
	health, err := session.getMetadata(ctx, "/healthz")
	if err != nil {
		return runtimeMetadata{}, err
	}
	metadata.Deployment = session.deployment
	if metadata.Deployment == "" {
		metadata.Deployment = config.DecisionsDeploymentNative
	}
	if metadata.Protocol != runtimeProtocol || metadata.Limits.ContextTokens != maxEvaluationTokens || metadata.Limits.MaxRequestBytes != maxRuntimeRequestBytes ||
		metadata.Limits.MaxImages != maxImages || metadata.Limits.MaxImageBytes != maxImageBytes || metadata.Limits.MaxTotalImageBytes != maxTotalImageBytes ||
		!validImageMarker(metadata.ImageMarker) || !sameRuntimeIdentity(metadata, health) {
		return runtimeMetadata{}, fmt.Errorf("runtime capabilities differ from the supported contract")
	}
	return metadata, nil
}

func (session *nativeRuntimeSession) getMetadata(ctx context.Context, route string) (runtimeMetadata, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, session.baseURL+route, nil)
	if err != nil {
		return runtimeMetadata{}, err
	}
	request.Header.Set("Authorization", "Bearer "+session.tokenValue())
	response, err := session.client.Do(request)
	if err != nil {
		return runtimeMetadata{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return runtimeMetadata{}, fmt.Errorf("runtime metadata endpoint returned HTTP %d", response.StatusCode)
	}
	var metadata runtimeMetadata
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&metadata); err != nil {
		return runtimeMetadata{}, err
	}
	return metadata, nil
}

func sameRuntimeIdentity(left, right runtimeMetadata) bool {
	return left.Protocol == right.Protocol && left.Ready == right.Ready && left.Model == right.Model && left.Projector == right.Projector &&
		left.Build == right.Build && sameRuntimeHardware(left.Hardware, right.Hardware) && left.Placement == right.Placement && left.ModelLayers == right.ModelLayers &&
		left.ImageMarker == right.ImageMarker
}

func validImageMarker(marker string) bool {
	return strings.TrimSpace(marker) != "" && len(marker) <= 256 && !strings.ContainsAny(marker, "\r\n")
}

func sameRuntimeHardware(left, right runtimeHardware) bool {
	leftDevice, rightDevice := left.SelectedDevice, right.SelectedDevice
	return leftDevice.ID == rightDevice.ID && leftDevice.Name == rightDevice.Name && leftDevice.Backend == rightDevice.Backend &&
		leftDevice.Type == rightDevice.Type && deviceBoolEqual(leftDevice.Integrated, rightDevice.Integrated) && left.Offload == right.Offload
}

func (session *nativeRuntimeSession) score(ctx context.Context, score runtimeScoreRequest) (runtimeScoreResult, error) {
	requestID, err := newRuntimeRequestID()
	if err != nil {
		return runtimeScoreResult{}, NewError(CodeInferenceFailed, err)
	}
	wire := runtimeScoreWireRequest{RequestID: requestID, Prompt: score.Prompt, Candidates: score.Candidates, MaxInputTokens: score.MaxInputTokens, Images: make([]runtimeImageWire, len(score.Images))}
	for index, image := range score.Images {
		wire.Images[index] = runtimeImageWire{MIMEType: image.MIME, Data: encodeBase64(image.Data)}
	}
	body, err := json.Marshal(wire)
	if err != nil {
		return runtimeScoreResult{}, NewError(CodeInvalidRequest, err)
	}
	if session.cached.Limits.MaxRequestBytes > 0 && int64(len(body)) > session.cached.Limits.MaxRequestBytes {
		return runtimeScoreResult{}, NewError(CodeInvalidRequest, fmt.Errorf("request exceeds runtime byte limit"))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, session.baseURL+"/score", bytes.NewReader(body))
	if err != nil {
		return runtimeScoreResult{}, err
	}
	request.Header.Set("Authorization", "Bearer "+session.tokenValue())
	request.Header.Set("Content-Type", "application/json")
	stopCancel := context.AfterFunc(ctx, func() { session.cancelRequest(requestID) })
	response, err := session.client.Do(request)
	stopCancel()
	if err != nil {
		if ctx.Err() != nil {
			return runtimeScoreResult{}, NewError(CodeCancelled, ctx.Err())
		}
		return runtimeScoreResult{}, NewError(CodeRuntimeUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return runtimeScoreResult{}, decodeRuntimeError(data, response.StatusCode)
	}
	var decoded runtimeScoreWireResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 32<<20)).Decode(&decoded); err != nil {
		return runtimeScoreResult{}, NewError(CodeInvalidRuntimeResponse, err)
	}
	if decoded.RequestID != requestID || decoded.Protocol != runtimeProtocol || decoded.Model != session.cached.Model || decoded.Projector != session.cached.Projector ||
		!strings.EqualFold(decoded.Build.BinarySHA256, session.device.BinarySHA) || decoded.Build.LlamaCPPCommit != llamaCPPCommit ||
		decoded.Build.LlamaCPPPatchSHA256 != llamaCPPPatchSHA256 ||
		!strings.EqualFold(decoded.Build.Backend, session.device.Backend) || decoded.Limits != session.cached.Limits || decoded.ImageMarker != session.cached.ImageMarker {
		return runtimeScoreResult{}, NewError(CodeRuntimeAttestationFailed, fmt.Errorf("runtime score metadata changed"))
	}
	metadata := runtimeMetadata{
		Protocol: decoded.Protocol, Ready: decoded.Ready, Model: decoded.Model, Projector: decoded.Projector,
		Build: decoded.Build, Hardware: decoded.Hardware, Placement: decoded.Placement, ModelLayers: decoded.ModelLayers,
		Limits: decoded.Limits, ImageMarker: decoded.ImageMarker, Deployment: session.deployment,
	}
	if err := validateRuntimeMetadata(metadata, session.device, session.deployment, session.cpuAllowed); err != nil {
		return runtimeScoreResult{}, NewError(CodeRuntimeAttestationFailed, err)
	}
	return runtimeScoreResult{Scores: decoded.Scores, usage: decoded.Usage, Metadata: metadata}, nil
}

func newRuntimeRequestID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (session *nativeRuntimeSession) cancelRequest(requestID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body, _ := json.Marshal(struct {
		RequestID string `json:"request_id"`
	}{RequestID: requestID})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, session.baseURL+"/cancel", bytes.NewReader(body))
	if err != nil {
		return
	}
	request.Header.Set("Authorization", "Bearer "+session.tokenValue())
	request.Header.Set("Content-Type", "application/json")
	response, err := session.client.Do(request)
	if err == nil {
		_ = response.Body.Close()
	}
}

func (session *nativeRuntimeSession) tokenValue() string {
	value, _ := os.ReadFile(session.token)
	return strings.TrimSpace(string(value))
}

func (session *nativeRuntimeSession) Close(ctx context.Context) error {
	ctx, closeCancel := context.WithTimeout(ctx, 10*time.Second)
	defer closeCancel()
	if session.cancel != nil {
		session.cancel()
	}
	if session.command != nil && session.command.Process != nil {
		select {
		case <-session.wait:
		case <-ctx.Done():
			_ = session.command.Process.Kill()
			return ctx.Err()
		}
	}
	if session.token != "" {
		_ = os.Remove(session.token)
	}
	return nil
}

func availableLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func writeRuntimeToken(directory string) (string, error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	token := hex.EncodeToString(random)
	file, err := os.CreateTemp(directory, ".decisions-token-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if _, err := io.WriteString(file, token+"\n"); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
