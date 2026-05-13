package tools

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestBuildPMCommand(t *testing.T) {
	tests := []struct {
		name      string
		pm        string
		operation string
		pkg       string
		want      []string
	}{
		{name: "apt install", pm: "apt", operation: "install", pkg: "jq", want: []string{"apt-get", "install", "-y", "jq"}},
		{name: "dnf update", pm: "dnf", operation: "update", want: []string{"dnf", "check-update"}},
		{name: "yum info", pm: "yum", operation: "info", pkg: "git", want: []string{"yum", "info", "git"}},
		{name: "pacman remove", pm: "pacman", operation: "remove", pkg: "nginx", want: []string{"pacman", "-Rns", "--noconfirm", "nginx"}},
		{name: "zypper list", pm: "zypper", operation: "list_installed", want: []string{"zypper", "search", "--installed-only"}},
		{name: "apk upgrade package", pm: "apk", operation: "upgrade", pkg: "curl", want: []string{"apk", "upgrade", "curl"}},
		{name: "brew search", pm: "brew", operation: "search", pkg: "postgres", want: []string{"brew", "search", "postgres"}},
		{name: "winget install", pm: "winget", operation: "install", pkg: "Git.Git", want: []string{"winget", "install", "--id", "Git.Git"}},
		{name: "choco upgrade all", pm: "choco", operation: "upgrade", want: []string{"choco", "upgrade", "all", "-y"}},
		{name: "scoop info", pm: "scoop", operation: "info", pkg: "7zip", want: []string{"scoop", "info", "7zip"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := buildPMCommand(tt.pm, tt.operation, tt.pkg)
			if err != nil {
				t.Fatalf("buildPMCommand() error = %v", err)
			}
			if got := cmd.Args; strings.Join(got, "\x00") != strings.Join(tt.want, "\x00") {
				t.Fatalf("buildPMCommand() args = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestBuildPMCommandRequiresPackage(t *testing.T) {
	for _, operation := range []string{"install", "remove", "search", "info"} {
		t.Run(operation, func(t *testing.T) {
			if _, err := buildPMCommand("apt", operation, ""); err == nil {
				t.Fatalf("buildPMCommand() error = nil, want required package error")
			}
		})
	}
}

func TestPMRequiresSudo(t *testing.T) {
	tests := []struct {
		pm        string
		operation string
		want      bool
	}{
		{pm: "apt", operation: "install", want: true},
		{pm: "dnf", operation: "remove", want: true},
		{pm: "pacman", operation: "update", want: true},
		{pm: "apk", operation: "upgrade", want: true},
		{pm: "apt", operation: "search", want: false},
		{pm: "brew", operation: "install", want: false},
		{pm: "winget", operation: "remove", want: false},
		{pm: "choco", operation: "upgrade", want: false},
	}

	for _, tt := range tests {
		if got := pmRequiresSudo(tt.pm, tt.operation); got != tt.want {
			t.Fatalf("pmRequiresSudo(%q, %q) = %v, want %v", tt.pm, tt.operation, got, tt.want)
		}
	}
}

func TestNormalizePMOutput(t *testing.T) {
	input := "\x1b[32mInstalling\x1b[0m\rProgress 10%\rDone\n"
	got := NormalizePMOutput(input)
	if strings.Contains(got, "\x1b[") || strings.Contains(got, "Progress 10%") {
		t.Fatalf("NormalizePMOutput() = %q", got)
	}
	if !strings.Contains(got, "Done") {
		t.Fatalf("NormalizePMOutput() = %q, want final output", got)
	}
}

func TestDetectPackageManagerUsesLookPath(t *testing.T) {
	original := packageManagerLookPath
	t.Cleanup(func() { packageManagerLookPath = original })
	packageManagerLookPath = func(file string) (string, error) {
		switch runtime.GOOS {
		case "linux":
			if file == "dnf" {
				return "/usr/bin/dnf", nil
			}
		case "darwin":
			if file == "brew" {
				return "/opt/homebrew/bin/brew", nil
			}
		case "windows":
			if file == "winget" {
				return `C:\Windows\winget.exe`, nil
			}
		}
		return "", errors.New("not found")
	}

	got, err := DetectPackageManager()
	if runtime.GOOS == "linux" && (err != nil || got != "dnf") {
		t.Fatalf("DetectPackageManager() = %q, %v; want dnf", got, err)
	}
	if runtime.GOOS == "darwin" && (err != nil || got != "brew") {
		t.Fatalf("DetectPackageManager() = %q, %v; want brew", got, err)
	}
	if runtime.GOOS == "windows" && (err != nil || got != "winget") {
		t.Fatalf("DetectPackageManager() = %q, %v; want winget", got, err)
	}
}

func TestPackageManagerPermissionDefaultsDeny(t *testing.T) {
	ClearRuntimePermissionsForTest()
	t.Cleanup(ClearRuntimePermissionsForTest)
	_, _, err := PackageManagerInstall("apt", "jq", false, "")
	if err == nil || !strings.Contains(err.Error(), "runtime permissions") {
		t.Fatalf("PackageManagerInstall() error = %v, want runtime permission denial", err)
	}
}

func TestValidatePackageManagerRejectsUnknown(t *testing.T) {
	if _, err := buildPMCommand("unknown", "search", "pkg"); err == nil {
		t.Fatal("buildPMCommand() error = nil, want unsupported manager")
	}
}
