package decisions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"aurago/internal/config"
	"aurago/internal/fileutil"
)

const runtimeManifestSchema = "aurago.decisions.runtime-manifest.v1"

type runtimeManifest struct {
	Schema   string `json:"schema"`
	Protocol string `json:"protocol"`
	Build    struct {
		Target              string `json:"target"`
		Backend             string `json:"backend"`
		LlamaCPPCommit      string `json:"llama_cpp_commit"`
		LlamaCPPPatchSHA256 string `json:"llama_cpp_patch_sha256"`
	} `json:"build"`
	Artifact struct {
		File      string `json:"file"`
		SHA256    string `json:"sha256"`
		SizeBytes int64  `json:"size_bytes"`
	} `json:"artifact"`
	Models []struct {
		Role      string `json:"role"`
		Repo      string `json:"repo"`
		Revision  string `json:"revision"`
		File      string `json:"file"`
		SHA256    string `json:"sha256"`
		SizeBytes int64  `json:"size_bytes"`
	} `json:"models"`
}

func runtimeTarget(goos, goarch string) string {
	return goos + "-" + goarch
}

func runtimeBackends(goos string) []string {
	switch goos {
	case "darwin":
		return []string{"metal", "vulkan", "cpu"}
	case "windows":
		return []string{"cuda", "vulkan", "cpu"}
	default:
		return []string{"cuda", "vulkan", "cpu"}
	}
}

func validateRuntimeManifest(directory, backend string, goos, goarch string) (string, string, error) {
	manifestPath := filepath.Join(directory, "release-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", "", err
	}
	var manifest runtimeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", "", fmt.Errorf("decode runtime release manifest")
	}
	if manifest.Schema != runtimeManifestSchema || manifest.Protocol != runtimeProtocol ||
		manifest.Build.Target != runtimeTarget(goos, goarch) || manifest.Build.Backend != backend || manifest.Build.LlamaCPPCommit != llamaCPPCommit ||
		manifest.Build.LlamaCPPPatchSHA256 != llamaCPPPatchSHA256 ||
		manifest.Artifact.File != executableName(goos) || manifest.Artifact.SizeBytes <= 0 || !validSHA256(manifest.Artifact.SHA256) {
		return "", "", fmt.Errorf("runtime release manifest does not match application pins")
	}
	pins := DefaultManifest()
	modelFound, projectorFound := false, false
	for _, model := range manifest.Models {
		if model.Repo != pins.Repository || model.Revision != modelRevision {
			continue
		}
		switch model.Role {
		case "model":
			modelFound = model.File == pins.Model.Filename && model.SHA256 == pins.Model.SHA256 && model.SizeBytes == pins.Model.Size
		case "projector":
			projectorFound = model.File == pins.Projector.Filename && model.SHA256 == pins.Projector.SHA256 && model.SizeBytes == pins.Projector.Size
		}
	}
	if !modelFound || !projectorFound {
		return "", "", fmt.Errorf("runtime manifest does not pin both model files")
	}
	binaryPath := filepath.Join(directory, manifest.Artifact.File)
	info, err := os.Lstat(binaryPath)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() || info.Size() != manifest.Artifact.SizeBytes {
		return "", "", fmt.Errorf("runtime binary size or file type differs from manifest")
	}
	actualSHA, err := fileSHA256(binaryPath)
	if err != nil {
		return "", "", err
	}
	if !strings.EqualFold(actualSHA, manifest.Artifact.SHA256) {
		return "", "", fmt.Errorf("runtime binary digest differs from manifest")
	}
	return binaryPath, actualSHA, nil
}

func executableName(goos string) string {
	if goos == "windows" {
		return "aurago-decisions-runtime.exe"
	}
	return "aurago-decisions-runtime"
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validateInstalledModelPaths(ctx context.Context, directory string, manifest Manifest, progress func(string, int64, int64)) (modelPaths, error) {
	if err := manifest.Validate(); err != nil {
		return modelPaths{}, err
	}
	modelPath := filepath.Join(directory, manifest.Model.Filename)
	projectorPath := filepath.Join(directory, manifest.Projector.Filename)
	if err := validateManagedFilePath(directory, modelPath); err != nil {
		return modelPaths{}, err
	}
	if err := validateManagedFilePath(directory, projectorPath); err != nil {
		return modelPaths{}, err
	}
	if err := verifyArtifactFile(ctx, modelPath, manifest.Model, progress); err != nil {
		return modelPaths{}, err
	}
	if err := verifyArtifactFile(ctx, projectorPath, manifest.Projector, progress); err != nil {
		return modelPaths{}, err
	}
	return modelPaths{Model: modelPath, Projector: projectorPath}, nil
}

func provisionNativeRuntime(ctx context.Context, sourceRoot, dataRoot string) error {
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		return err
	}
	found := false
	for _, backend := range runtimeBackends(runtime.GOOS) {
		if err := ctx.Err(); err != nil {
			return err
		}
		sourceDir := filepath.Join(sourceRoot, "bin", "decisions-runtime", runtimeTarget(runtime.GOOS, runtime.GOARCH), backend)
		_, _, err := validateRuntimeManifest(sourceDir, backend, runtime.GOOS, runtime.GOARCH)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("verify %s runtime bundle: %w", backend, err)
		}
		found = true
		destinationDir := filepath.Join(dataRoot, runtimeTarget(runtime.GOOS, runtime.GOARCH), backend)
		if err := copyVerifiedRuntimeBundle(ctx, sourceDir, destinationDir); err != nil {
			return err
		}
	}
	if !found {
		return NewError(CodeRuntimeArtifactMissing, fmt.Errorf("no locally built pinned decisions runtime bundle is available"))
	}
	return nil
}

func copyVerifiedRuntimeBundle(ctx context.Context, sourceDir, destinationDir string) error {
	if err := os.MkdirAll(destinationDir, 0o700); err != nil {
		return err
	}
	for _, name := range []string{executableName(runtime.GOOS), "release-manifest.json"} {
		source := filepath.Join(sourceDir, name)
		destination := filepath.Join(destinationDir, name)
		if err := copyAtomic(ctx, source, destination); err != nil {
			return err
		}
	}
	if _, _, err := validateRuntimeManifest(destinationDir, filepath.Base(destinationDir), runtime.GOOS, runtime.GOARCH); err != nil {
		return fmt.Errorf("verify provisioned runtime bundle: %w", err)
	}
	return nil
}

func copyAtomic(ctx context.Context, source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("runtime bundle source is not a regular file")
	}
	temporary := destination + ".tmp"
	output, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(&artifactProgressWriter{ctx: ctx, writer: output}, input)
	if copyErr == nil {
		copyErr = output.Sync()
	}
	closeErr := output.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		_ = os.Remove(temporary)
		return copyErr
	}
	if err := fileutil.RenameContext(ctx, temporary, destination); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func deploymentForInstall(cfg config.DecisionsConfig, access RuntimeAccess) (string, error) {
	return resolveDeployment(cfg.Deployment, access)
}
