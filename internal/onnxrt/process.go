package onnxrt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"aurago/internal/sandbox"
)

// ConfigureWorkerCommand prepares a native worker process: the environment is
// filtered through sandbox.FilterEnv, every directory below runtimeRoots is
// prepended to PATH (plus LD_LIBRARY_PATH on Linux, DYLD_LIBRARY_PATH on
// macOS), and on Windows no console window opens. Without runtime roots the
// filtered environment is used unchanged.
func ConfigureWorkerCommand(cmd *exec.Cmd, runtimeRoots ...string) {
	hideWindow(cmd)
	if len(runtimeRoots) == 0 {
		cmd.Env = filterEnvironment(os.Environ())
		return
	}
	cmd.Env = runtimeEnvironment(goruntime.GOOS, runtimeRoots...)
}

// filterEnvironment is sandbox.FilterEnv with a non-nil result: exec.Cmd
// treats a nil Env as "inherit the whole environment", which would hand the
// secrets back to the worker when every variable was filtered out.
func filterEnvironment(environment []string) []string {
	filtered := sandbox.FilterEnv(environment)
	if filtered == nil {
		filtered = []string{}
	}
	return filtered
}

func runtimeEnvironment(goos string, runtimeRoots ...string) []string {
	environment := filterEnvironment(os.Environ())
	var directories []string
	for _, runtimeRoot := range runtimeRoots {
		_ = filepath.WalkDir(runtimeRoot, func(path string, entry os.DirEntry, err error) error {
			if err == nil && entry.IsDir() {
				directories = append(directories, path)
			}
			return nil
		})
	}
	prefix := strings.Join(directories, string(os.PathListSeparator))
	keys := []string{"PATH"}
	switch goos {
	case "linux":
		keys = append(keys, "LD_LIBRARY_PATH")
	case "darwin":
		keys = append(keys, "DYLD_LIBRARY_PATH")
	}
	for _, key := range keys {
		current := environmentValue(environment, key)
		value := prefix
		if current != "" {
			value += string(os.PathListSeparator) + current
		}
		environment = setEnvironmentValue(environment, key, value)
	}
	return environment
}

func environmentValue(environment []string, key string) string {
	prefix := strings.ToUpper(key) + "="
	for _, entry := range environment {
		if strings.HasPrefix(strings.ToUpper(entry), prefix) {
			return entry[len(prefix):]
		}
	}
	return ""
}

func setEnvironmentValue(environment []string, key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	for index, entry := range environment {
		if strings.HasPrefix(strings.ToUpper(entry), prefix) {
			environment[index] = key + "=" + value
			return environment
		}
	}
	return append(environment, key+"="+value)
}

// AccessibleRenderNode returns the first /dev/dri/renderD* node below driRoot
// that this process can open read-write. The error texts are part of the
// embeddings hardware fingerprint; keep them unchanged.
func AccessibleRenderNode(driRoot string) (string, error) {
	renderDevices, err := filepath.Glob(filepath.Join(driRoot, "renderD*"))
	if err != nil {
		return "", fmt.Errorf("find Vulkan render devices: %w", err)
	}
	if len(renderDevices) == 0 {
		return "", fmt.Errorf("no Vulkan render device found under %s", driRoot)
	}
	var accessErrors []string
	for _, renderDevice := range renderDevices {
		handle, openErr := os.OpenFile(renderDevice, os.O_RDWR, 0)
		if openErr == nil {
			if closeErr := handle.Close(); closeErr != nil {
				return "", fmt.Errorf("close Vulkan render device %s: %w", renderDevice, closeErr)
			}
			return renderDevice, nil
		}
		accessErrors = append(accessErrors, filepath.Base(renderDevice)+": "+openErr.Error())
	}
	return "", fmt.Errorf("no accessible Vulkan render device under %s (%s)", driRoot, strings.Join(accessErrors, "; "))
}
