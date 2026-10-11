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

// ConfigureWorkerCommand prepares a native worker process. It replaces
// cmd.Env (callers add their own variables afterwards): the environment is
// the current one filtered through sandbox.FilterEnv, with every directory
// below runtimeRoots prepended to PATH (plus LD_LIBRARY_PATH on Linux,
// DYLD_LIBRARY_PATH on macOS). Roots are made absolute; roots that are empty,
// missing, not directories or cannot be stat'ed contribute nothing, and when
// no directory is found the filtered environment is used unchanged, so the
// working directory never lands on a library search path. On Windows it also sets HideWindow and
// CREATE_NO_WINDOW on cmd.SysProcAttr, keeping any other attributes the
// caller already set, so no console window opens.
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
	return prependLibraryDirectories(goos, filterEnvironment(os.Environ()), runtimeDirectories(runtimeRoots))
}

// runtimeDirectories lists every directory below the roots as absolute
// paths. An empty root is skipped before it can resolve to the working
// directory.
func runtimeDirectories(runtimeRoots []string) []string {
	var directories []string
	for _, runtimeRoot := range runtimeRoots {
		if runtimeRoot == "" {
			continue
		}
		absoluteRoot, err := filepath.Abs(runtimeRoot)
		if err != nil {
			continue
		}
		_ = filepath.WalkDir(absoluteRoot, func(path string, entry os.DirEntry, err error) error {
			if err == nil && entry.IsDir() {
				directories = append(directories, path)
			}
			return nil
		})
	}
	return directories
}

// prependLibraryDirectories puts directories in front of the library search
// variables of goos. Without directories the environment is returned as it
// is: joining nothing would add an empty element, which stands for the
// working directory.
func prependLibraryDirectories(goos string, environment, directories []string) []string {
	if len(directories) == 0 {
		return environment
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
		current := environmentValue(goos, environment, key)
		value := prefix
		if current != "" {
			value += string(os.PathListSeparator) + current
		}
		environment = setEnvironmentValue(goos, environment, key, value)
	}
	return environment
}

// environmentEntryHasName reports whether entry ("NAME=value") sets key.
// Environment names are case-insensitive on Windows only.
func environmentEntryHasName(goos, entry, key string) bool {
	prefix := key + "="
	if goos == "windows" {
		return strings.HasPrefix(strings.ToUpper(entry), strings.ToUpper(prefix))
	}
	return strings.HasPrefix(entry, prefix)
}

func environmentValue(goos string, environment []string, key string) string {
	for _, entry := range environment {
		if environmentEntryHasName(goos, entry, key) {
			return entry[len(key)+1:]
		}
	}
	return ""
}

func setEnvironmentValue(goos string, environment []string, key, value string) []string {
	for index, entry := range environment {
		if environmentEntryHasName(goos, entry, key) {
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
