// Package shared provides common utilities for agocli subcommands.
package shared

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// PackageManager identifies a system package manager.
type PackageManager int

const (
	PkgUnknown PackageManager = iota
	PkgApt
	PkgDnf
	PkgYum
	PkgPacman
	PkgApk
	PkgZypper
)

// String returns the command name for the package manager.
func (p PackageManager) String() string {
	switch p {
	case PkgApt:
		return "apt-get"
	case PkgDnf:
		return "dnf"
	case PkgYum:
		return "yum"
	case PkgPacman:
		return "pacman"
	case PkgApk:
		return "apk"
	case PkgZypper:
		return "zypper"
	default:
		return ""
	}
}

// InstallArgs returns the arguments for installing packages.
func (p PackageManager) InstallArgs(pkgs ...string) []string {
	switch p {
	case PkgApt:
		args := []string{"apt-get", "install", "-y"}
		return append(args, pkgs...)
	case PkgDnf:
		args := []string{"dnf", "install", "-y"}
		return append(args, pkgs...)
	case PkgYum:
		args := []string{"yum", "install", "-y"}
		return append(args, pkgs...)
	case PkgPacman:
		args := []string{"pacman", "-S", "--noconfirm"}
		return append(args, pkgs...)
	case PkgApk:
		args := []string{"apk", "add"}
		return append(args, pkgs...)
	case PkgZypper:
		args := []string{"zypper", "install", "-y"}
		return append(args, pkgs...)
	default:
		return nil
	}
}

// DetectPackageManager detects the system package manager.
func DetectPackageManager() PackageManager {
	if runtime.GOOS != "linux" {
		return PkgUnknown
	}
	for _, pm := range []struct {
		cmd string
		pkg PackageManager
	}{
		{"apt-get", PkgApt},
		{"dnf", PkgDnf},
		{"yum", PkgYum},
		{"pacman", PkgPacman},
		{"apk", PkgApk},
		{"zypper", PkgZypper},
	} {
		if _, err := exec.LookPath(pm.cmd); err == nil {
			return pm.pkg
		}
	}
	return PkgUnknown
}

// HasSystemd returns true if the system uses systemd.
func HasSystemd() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := exec.LookPath("systemctl")
	return err == nil
}

// HasCommand returns true if a command exists in PATH.
func HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// DetectArch returns the Go-compatible GOARCH string.
func DetectArch() string {
	return runtime.GOARCH
}

// DetectOS returns the Go-compatible GOOS string.
func DetectOS() string {
	return runtime.GOOS
}

// IsGitRepo returns true if the current directory (or installDir) is a git repo.
func IsGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// ServiceUser returns the best user to run the service as.
// Priority: SUDO_USER → current user → directory owner.
func ServiceUser() string {
	if u := os.Getenv("SUDO_USER"); u != "" && u != "root" {
		return u
	}
	if u := os.Getenv("USER"); u != "" && u != "root" {
		return u
	}
	return "root"
}

// ReadOSRelease reads a field from /etc/os-release.
func ReadOSRelease(field string) string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	prefix := field + "="
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			val := strings.TrimPrefix(line, prefix)
			return strings.Trim(val, "\"")
		}
	}
	return ""
}
