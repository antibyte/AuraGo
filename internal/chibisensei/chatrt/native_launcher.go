package chatrt

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"aurago/internal/llamacpp"
	"aurago/internal/onnxrt"
	"aurago/internal/security"
)

const assetUserAgent = "AuraGo/chibi-sensei"

// nativeLauncher gives every child it starts the same API key: security.RegisterSensitive keeps each value for the
// life of the process, so a key per start would grow the registry with every idle stop and restart.
type nativeLauncher struct {
	register func(string) // security.RegisterSensitive in production; a field so a test can count the calls

	keyMu sync.Mutex // guards key
	key   string
}

// NativeLauncher is the real launcher: pinned llama.cpp archives in an onnxrt asset cache and loopback children.
func NativeLauncher() Launcher { return &nativeLauncher{register: security.RegisterSensitive} }

// apiKey creates the launcher's key on first use and registers it before it is used anywhere (decision D15).
// The lock covers only the random read, which does not block.
func (n *nativeLauncher) apiKey() (string, error) {
	n.keyMu.Lock()
	defer n.keyMu.Unlock()
	if n.key != "" {
		return n.key, nil
	}
	key, err := llamacpp.RandomAPIKey()
	if err != nil {
		return "", err
	}
	if n.register != nil {
		n.register(key)
	}
	n.key = key
	return key, nil
}

func (*nativeLauncher) Platform() (string, string) { return runtime.GOOS, runtime.GOARCH }

func (*nativeLauncher) Hardware(ctx context.Context) llamacpp.HardwareInfo {
	return llamacpp.DetectHardware(ctx)
}

func (*nativeLauncher) Assets(backend string) ([]llamacpp.AssetSpec, bool) {
	return installSet(runtime.GOOS, runtime.GOARCH, backend)
}

// installSet is the CPU archive, then the backend's archive and its supplements, without duplicate IDs
// (darwin/arm64 cpu and metal are one archive). ok is false without a CPU archive.
func installSet(goos, goarch, backend string) ([]llamacpp.AssetSpec, bool) {
	cpu, ok := llamacpp.RuntimeAsset(goos, goarch, "cpu")
	if !ok {
		return nil, false
	}
	set := []llamacpp.AssetSpec{cpu}
	seen := map[string]bool{cpu.ID: true}
	add := func(a llamacpp.AssetSpec) {
		if a.ID != "" && !seen[a.ID] {
			seen[a.ID] = true
			set = append(set, a)
		}
	}
	if backend != "" && backend != "cpu" {
		if gpu, ok := llamacpp.RuntimeAsset(goos, goarch, backend); ok {
			add(gpu)
			for _, extra := range llamacpp.SupplementalAssets(goos, goarch, backend) {
				add(extra)
			}
		}
	}
	return set, true
}

func (*nativeLauncher) Verify(ctx context.Context, cacheDir string, assets []llamacpp.AssetSpec) bool {
	cache := onnxrt.NewAssetCache(cacheDir, assetUserAgent, nil)
	for _, asset := range assets {
		if _, err := cache.VerifyRuntime(ctx, asset); err != nil {
			return false
		}
	}
	return len(assets) > 0
}

func (*nativeLauncher) Install(ctx context.Context, cacheDir string, assets []llamacpp.AssetSpec, progress func(string, int64)) error {
	cache := onnxrt.NewAssetCache(cacheDir, assetUserAgent, func(s onnxrt.DownloadStatus) {
		if progress != nil {
			progress(s.Asset, s.Downloaded)
		}
	})
	for _, asset := range assets {
		if _, err := cache.EnsureRuntime(ctx, asset); err != nil {
			return fmt.Errorf("install llama.cpp runtime %s: %w", asset.ID, err)
		}
	}
	return nil
}

func (n *nativeLauncher) Launch(ctx context.Context, spec LaunchSpec) (Process, error) {
	cache := onnxrt.NewAssetCache(spec.CacheDir, assetUserAgent, nil)
	roots := make([]string, 0, len(spec.Assets))
	for _, asset := range spec.Assets {
		root, err := cache.VerifyRuntime(ctx, asset)
		if err != nil {
			return nil, fmt.Errorf("llama.cpp runtime %s: %w", asset.ID, ErrNotInstalled)
		}
		roots = append(roots, root)
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("launch llama-server: no runtime assets")
	}
	server, err := llamacpp.FindServer(roots[0])
	if err != nil {
		return nil, err
	}
	key, err := n.apiKey()
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		proc, bindFailure, err := startServer(ctx, server, roots, key, spec)
		if err == nil {
			return proc, nil
		}
		lastErr = err
		if !bindFailure || ctx.Err() != nil {
			break
		}
	}
	return nil, lastErr
}

// serverArgs lists the argv of one server. The API key is never part of it: it travels in the environment.
func serverArgs(spec LaunchSpec, port int) []string {
	ngl := "0"
	if spec.Backend != "cpu" {
		ngl = "999"
	}
	t := strconv.Itoa(max(1, spec.Threads))
	return []string{
		"--model", spec.ModelPath, "--alias", chatModelAlias, "--host", "127.0.0.1", "--port", strconv.Itoa(port),
		"--ctx-size", strconv.Itoa(spec.ContextSize), "--parallel", "1", "--n-gpu-layers", ngl,
		"--threads", t, "--threads-batch", t, "--jinja", "--reasoning", "off", "--no-webui", "--no-slots",
	}
}

// serverEnv drops every inherited LLAMA_* variable (LLAMA_ARG_* would change flags we do not pass) and adds the key.
func serverEnv(env []string, key string) []string {
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if len(entry) >= 6 && strings.EqualFold(entry[:6], "LLAMA_") {
			continue
		}
		out = append(out, entry)
	}
	return append(out, "LLAMA_API_KEY="+key)
}

// newServerCommand builds the child command with the filtered environment of onnxrt.ConfigureWorkerCommand.
func newServerCommand(server string, roots []string, key string, args []string) *exec.Cmd {
	cmd := exec.Command(server, args...)
	onnxrt.ConfigureWorkerCommand(cmd, roots...)
	cmd.Env = serverEnv(cmd.Env, key)
	return cmd
}
