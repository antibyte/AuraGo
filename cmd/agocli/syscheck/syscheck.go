// Package syscheck provides system dependency detection and installation for agocli.
package syscheck

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"aurago/cmd/agocli/shared"
)

// Dependency represents a system dependency with detection and installation info.
type Dependency struct {
	Name        string   // Human-readable name
	Command     string   // Binary to check in PATH
	Required    bool     // true = setup fails without it; false = optional
	Description string   // Shown in TUI selection
	Packages    PkgNames // Package names per package manager
}

// PkgNames maps package manager to package name(s).
type PkgNames struct {
	Apt    []string
	Dnf    []string
	Yum    []string
	Pacman []string
	Apk    []string
	Zypper []string
}

// ForManager returns the package names for the given package manager.
func (p PkgNames) ForManager(pm shared.PackageManager) []string {
	switch pm {
	case shared.PkgApt:
		return p.Apt
	case shared.PkgDnf:
		return p.Dnf
	case shared.PkgYum:
		return p.Yum
	case shared.PkgPacman:
		return p.Pacman
	case shared.PkgApk:
		return p.Apk
	case shared.PkgZypper:
		return p.Zypper
	default:
		return nil
	}
}

// AllDependencies returns all known dependencies.
func AllDependencies() []Dependency {
	return []Dependency{
		{
			Name:        "Python 3",
			Command:     "python3",
			Required:    false,
			Description: "Required for Python tool execution and skills",
			Packages: PkgNames{
				Apt:    []string{"python3", "python3-pip", "python3-venv"},
				Dnf:    []string{"python3", "python3-pip"},
				Yum:    []string{"python3", "python3-pip"},
				Pacman: []string{"python", "python-pip"},
				Apk:    []string{"python3", "py3-pip"},
				Zypper: []string{"python3", "python3-pip"},
			},
		},
		{
			Name:        "FFmpeg",
			Command:     "ffmpeg",
			Required:    false,
			Description: "Required for audio/video processing (TTS, voice messages)",
			Packages: PkgNames{
				Apt:    []string{"ffmpeg"},
				Dnf:    []string{"ffmpeg-free"},
				Yum:    []string{"ffmpeg"},
				Pacman: []string{"ffmpeg"},
				Apk:    []string{"ffmpeg"},
				Zypper: []string{"ffmpeg"},
			},
		},
		{
			Name:        "Docker",
			Command:     "docker",
			Required:    false,
			Description: "Required for container management and sidecar services",
			Packages: PkgNames{
				Apt:    []string{"docker.io"},
				Dnf:    []string{"docker-ce"},
				Yum:    []string{"docker-ce"},
				Pacman: []string{"docker"},
				Apk:    []string{"docker"},
				Zypper: []string{"docker"},
			},
		},
		{
			Name:        "Git",
			Command:     "git",
			Required:    false,
			Description: "Required for source-based updates",
			Packages: PkgNames{
				Apt:    []string{"git"},
				Dnf:    []string{"git"},
				Yum:    []string{"git"},
				Pacman: []string{"git"},
				Apk:    []string{"git"},
				Zypper: []string{"git"},
			},
		},
		{
			Name:        "setcap",
			Command:     "setcap",
			Required:    false,
			Description: "Required for HTTPS on ports 80/443 without root",
			Packages: PkgNames{
				Apt:    []string{"libcap2-bin"},
				Dnf:    []string{"libcap"},
				Yum:    []string{"libcap"},
				Pacman: []string{"libcap"},
				Apk:    []string{"libcap"},
				Zypper: []string{"libcap-progs"},
			},
		},
		{
			Name:        "curl",
			Command:     "curl",
			Required:    false,
			Description: "HTTP client used by various tools",
			Packages: PkgNames{
				Apt:    []string{"curl"},
				Dnf:    []string{"curl"},
				Yum:    []string{"curl"},
				Pacman: []string{"curl"},
				Apk:    []string{"curl"},
				Zypper: []string{"curl"},
			},
		},
	}
}

// CheckResult holds the result of checking a dependency.
type CheckResult struct {
	Dependency Dependency
	Installed  bool
	Version    string
}

// CheckAll checks all dependencies and returns results.
func CheckAll() []CheckResult {
	deps := AllDependencies()
	results := make([]CheckResult, len(deps))
	for i, dep := range deps {
		results[i] = CheckResult{
			Dependency: dep,
			Installed:  shared.HasCommand(dep.Command),
		}
		if results[i].Installed {
			results[i].Version = getVersion(dep.Command)
		}
	}
	return results
}

// MissingOptional returns dependencies that are not installed and are optional.
func MissingOptional(results []CheckResult) []CheckResult {
	var missing []CheckResult
	for _, r := range results {
		if !r.Installed && !r.Dependency.Required {
			missing = append(missing, r)
		}
	}
	return missing
}

// InstallPackages installs packages via the detected package manager.
func InstallPackages(pm shared.PackageManager, pkgs []string) error {
	if pm == shared.PkgUnknown {
		return fmt.Errorf("no supported package manager found")
	}
	if len(pkgs) == 0 {
		return nil
	}

	if runtime.GOOS != "linux" {
		return fmt.Errorf("package installation only supported on Linux")
	}

	args := pm.InstallArgs(pkgs...)
	if args == nil {
		return fmt.Errorf("unsupported package manager: %s", pm)
	}

	// Prepend sudo
	cmdArgs := append([]string{"sudo"}, args...)
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// CollectPackages collects all package names for the given dependencies and package manager.
func CollectPackages(pm shared.PackageManager, deps []Dependency) []string {
	var pkgs []string
	for _, dep := range deps {
		pkgs = append(pkgs, dep.Packages.ForManager(pm)...)
	}
	return pkgs
}

func getVersion(cmd string) string {
	// Try common version flags
	for _, flag := range []string{"--version", "-v", "version"} {
		out, err := exec.Command(cmd, flag).CombinedOutput()
		if err == nil {
			v := strings.TrimSpace(string(out))
			// Take first line only
			if idx := strings.IndexByte(v, '\n'); idx > 0 {
				v = v[:idx]
			}
			if len(v) > 80 {
				v = v[:80]
			}
			return v
		}
	}
	return "installed"
}
