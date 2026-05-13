package tools

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

const packageManagerOutputLimit = 8 * 1024

var (
	packageManagerLookPath = exec.LookPath
	packageManagerANSI     = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	packageManagerCRLine   = regexp.MustCompile(`(?m)^.*\r`)
)

// DetectPackageManager returns the first supported package manager available on PATH.
func DetectPackageManager() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return detectPackageManagerFrom([]string{"apt", "dnf", "yum", "pacman", "zypper", "apk"}, "no supported package manager found on this Linux system")
	case "darwin":
		return detectPackageManagerFrom([]string{"brew"}, "homebrew is not installed")
	case "windows":
		return detectPackageManagerFrom([]string{"winget", "choco", "scoop"}, "no supported package manager found (install winget, choco, or scoop)")
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func detectPackageManagerFrom(candidates []string, errMsg string) (string, error) {
	for _, pm := range candidates {
		if _, err := packageManagerLookPath(pm); err == nil {
			return pm, nil
		}
	}
	return "", fmt.Errorf("%s", errMsg)
}

func PackageManagerInstall(pm, pkg string, sudo bool, password string) (string, string, error) {
	return runPackageManagerMutation(pm, "install", pkg, sudo, password)
}

func PackageManagerRemove(pm, pkg string, sudo bool, password string) (string, string, error) {
	return runPackageManagerMutation(pm, "remove", pkg, sudo, password)
}

func PackageManagerUpdate(pm string, sudo bool, password string) (string, string, error) {
	return runPackageManagerMutation(pm, "update", "", sudo, password)
}

func PackageManagerUpgrade(pm, pkg string, sudo bool, password string) (string, string, error) {
	return runPackageManagerMutation(pm, "upgrade", pkg, sudo, password)
}

func PackageManagerSearch(pm, query string) (string, error) {
	return runPackageManagerReadOnly(pm, "search", query)
}

func PackageManagerListInstalled(pm string) (string, error) {
	return runPackageManagerReadOnly(pm, "list_installed", "")
}

func PackageManagerInfo(pm, pkg string) (string, error) {
	return runPackageManagerReadOnly(pm, "info", pkg)
}

func runPackageManagerMutation(pm, operation, pkg string, sudo bool, password string) (string, string, error) {
	if err := requirePackageManagerMutationPermission(operation); err != nil {
		return "", "", err
	}
	cmd, err := buildPMCommand(pm, operation, pkg)
	if err != nil {
		return "", "", err
	}
	if sudo {
		if strings.TrimSpace(password) == "" {
			return "", "", fmt.Errorf("sudo password is required for %s with %s; store sudo_password in the vault", operation, pm)
		}
		stdout, stderr, err := ExecuteSudo(shellJoin(cmd.Args), ".", password)
		return NormalizePMOutput(stdout), NormalizePMOutput(stderr), err
	}
	stdout, stderr, err := runPackageManagerCommand(cmd)
	return stdout, stderr, err
}

func runPackageManagerReadOnly(pm, operation, pkg string) (string, error) {
	if err := requirePackageManagerPermission(); err != nil {
		return "", err
	}
	cmd, err := buildPMCommand(pm, operation, pkg)
	if err != nil {
		return "", err
	}
	stdout, stderr, err := runPackageManagerCommand(cmd)
	if err != nil {
		if stderr != "" {
			return stdout, fmt.Errorf("command failed: %w: %s", err, stderr)
		}
		return stdout, err
	}
	if stdout != "" {
		return stdout, nil
	}
	return stderr, nil
}

func buildPMCommand(pm, operation, pkg string) (*exec.Cmd, error) {
	pm = strings.ToLower(strings.TrimSpace(pm))
	operation = strings.ToLower(strings.TrimSpace(operation))
	pkg = strings.TrimSpace(pkg)
	if err := validatePackageManager(pm); err != nil {
		return nil, err
	}
	if packageManagerNeedsPackage(operation) && pkg == "" {
		return nil, fmt.Errorf("package is required for %s", operation)
	}

	switch pm {
	case "apt":
		switch operation {
		case "install":
			return exec.Command("apt-get", "install", "-y", pkg), nil
		case "remove":
			return exec.Command("apt-get", "remove", "-y", pkg), nil
		case "update":
			return exec.Command("apt-get", "update"), nil
		case "upgrade":
			args := []string{"upgrade", "-y"}
			if pkg != "" {
				args = append(args, pkg)
			}
			return exec.Command("apt-get", args...), nil
		case "search":
			return exec.Command("apt-cache", "search", pkg), nil
		case "list_installed":
			return exec.Command("apt", "list", "--installed"), nil
		case "info":
			return exec.Command("apt-cache", "show", pkg), nil
		}
	case "dnf", "yum":
		switch operation {
		case "install", "remove", "upgrade":
			args := []string{operation, "-y"}
			if pkg != "" {
				args = append(args, pkg)
			}
			return exec.Command(pm, args...), nil
		case "update":
			return exec.Command(pm, "check-update"), nil
		case "search":
			return exec.Command(pm, "search", pkg), nil
		case "list_installed":
			return exec.Command(pm, "list", "installed"), nil
		case "info":
			return exec.Command(pm, "info", pkg), nil
		}
	case "pacman":
		switch operation {
		case "install":
			return exec.Command("pacman", "-S", "--noconfirm", pkg), nil
		case "remove":
			return exec.Command("pacman", "-Rns", "--noconfirm", pkg), nil
		case "update":
			return exec.Command("pacman", "-Sy"), nil
		case "upgrade":
			args := []string{"-Syu", "--noconfirm"}
			if pkg != "" {
				args = append(args, pkg)
			}
			return exec.Command("pacman", args...), nil
		case "search":
			return exec.Command("pacman", "-Ss", pkg), nil
		case "list_installed":
			return exec.Command("pacman", "-Q"), nil
		case "info":
			return exec.Command("pacman", "-Si", pkg), nil
		}
	case "zypper":
		switch operation {
		case "install":
			return exec.Command("zypper", "--non-interactive", "install", pkg), nil
		case "remove":
			return exec.Command("zypper", "--non-interactive", "remove", pkg), nil
		case "update":
			return exec.Command("zypper", "refresh"), nil
		case "upgrade":
			args := []string{"--non-interactive", "update"}
			if pkg != "" {
				args = append(args, pkg)
			}
			return exec.Command("zypper", args...), nil
		case "search":
			return exec.Command("zypper", "search", pkg), nil
		case "list_installed":
			return exec.Command("zypper", "search", "--installed-only"), nil
		case "info":
			return exec.Command("zypper", "info", pkg), nil
		}
	case "apk":
		switch operation {
		case "install":
			return exec.Command("apk", "add", pkg), nil
		case "remove":
			return exec.Command("apk", "del", pkg), nil
		case "update":
			return exec.Command("apk", "update"), nil
		case "upgrade":
			args := []string{"upgrade"}
			if pkg != "" {
				args = append(args, pkg)
			}
			return exec.Command("apk", args...), nil
		case "search":
			return exec.Command("apk", "search", pkg), nil
		case "list_installed":
			return exec.Command("apk", "info"), nil
		case "info":
			return exec.Command("apk", "info", pkg), nil
		}
	case "brew":
		switch operation {
		case "install":
			return exec.Command("brew", "install", pkg), nil
		case "remove":
			return exec.Command("brew", "uninstall", pkg), nil
		case "update":
			return exec.Command("brew", "update"), nil
		case "upgrade":
			args := []string{"upgrade"}
			if pkg != "" {
				args = append(args, pkg)
			}
			return exec.Command("brew", args...), nil
		case "search":
			return exec.Command("brew", "search", pkg), nil
		case "list_installed":
			return exec.Command("brew", "list"), nil
		case "info":
			return exec.Command("brew", "info", pkg), nil
		}
	case "winget":
		switch operation {
		case "install":
			return exec.Command("winget", "install", "--id", pkg), nil
		case "remove":
			return exec.Command("winget", "uninstall", "--id", pkg), nil
		case "update":
			return exec.Command("winget", "source", "update"), nil
		case "upgrade":
			args := []string{"upgrade"}
			if pkg != "" {
				args = append(args, "--id", pkg)
			}
			return exec.Command("winget", args...), nil
		case "search":
			return exec.Command("winget", "search", pkg), nil
		case "list_installed":
			return exec.Command("winget", "list"), nil
		case "info":
			return exec.Command("winget", "show", "--id", pkg), nil
		}
	case "choco":
		switch operation {
		case "install", "remove", "upgrade":
			chocoOp := operation
			if operation == "remove" {
				chocoOp = "uninstall"
			}
			args := []string{chocoOp}
			if pkg != "" {
				args = append(args, pkg)
			} else if operation == "upgrade" {
				args = append(args, "all")
			}
			args = append(args, "-y")
			return exec.Command("choco", args...), nil
		case "update":
			return exec.Command("choco", "upgrade", "all", "--noop"), nil
		case "search":
			return exec.Command("choco", "search", pkg), nil
		case "list_installed":
			return exec.Command("choco", "list"), nil
		case "info":
			return exec.Command("choco", "info", pkg), nil
		}
	case "scoop":
		switch operation {
		case "install":
			return exec.Command("scoop", "install", pkg), nil
		case "remove":
			return exec.Command("scoop", "uninstall", pkg), nil
		case "update":
			return exec.Command("scoop", "update"), nil
		case "upgrade":
			if pkg != "" {
				return exec.Command("scoop", "update", pkg), nil
			}
			return exec.Command("scoop", "update", "*"), nil
		case "search":
			return exec.Command("scoop", "search", pkg), nil
		case "list_installed":
			return exec.Command("scoop", "list"), nil
		case "info":
			return exec.Command("scoop", "info", pkg), nil
		}
	}
	return nil, fmt.Errorf("unsupported package manager operation: %s for %s", operation, pm)
}

func runPackageManagerCommand(cmd *exec.Cmd) (string, string, error) {
	SetupCmd(cmd)
	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout:  GetForegroundTimeout(),
		Graceful: true,
		KillWait: shellKillWait,
		ErrMsg:   "TIMEOUT: package manager command exceeded %s limit",
	})
	stdout, stderr, err := runner.Run(context.Background())
	return NormalizePMOutput(stdout), NormalizePMOutput(stderr), err
}

func pmRequiresSudo(pm, operation string) bool {
	pm = strings.ToLower(strings.TrimSpace(pm))
	operation = strings.ToLower(strings.TrimSpace(operation))
	if !packageManagerMutation(operation) {
		return false
	}
	switch pm {
	case "apt", "dnf", "yum", "pacman", "zypper", "apk":
		return true
	default:
		return false
	}
}

func PackageManagerRequiresSudo(pm, operation string) bool {
	return pmRequiresSudo(pm, operation)
}

func packageManagerMutation(operation string) bool {
	switch operation {
	case "install", "remove", "update", "upgrade":
		return true
	default:
		return false
	}
}

func packageManagerNeedsPackage(operation string) bool {
	switch operation {
	case "install", "remove", "search", "info":
		return true
	default:
		return false
	}
}

func validatePackageManager(pm string) error {
	switch pm {
	case "apt", "dnf", "yum", "pacman", "zypper", "apk", "brew", "winget", "choco", "scoop":
		return nil
	case "":
		return fmt.Errorf("package manager is required")
	default:
		return fmt.Errorf("unsupported package manager %q", pm)
	}
}

func NormalizePMOutput(output string) string {
	output = packageManagerANSI.ReplaceAllString(output, "")
	output = packageManagerCRLine.ReplaceAllString(output, "")
	output = strings.TrimSpace(output)
	if len(output) > packageManagerOutputLimit {
		return output[:packageManagerOutputLimit] + "\n... [output truncated due to size limit]"
	}
	return output
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if strings.IndexFunc(arg, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && !strings.ContainsRune("@%_+=:,./-", r)
	}) == -1 {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}
