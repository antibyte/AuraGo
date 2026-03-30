package updater

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// BuildBinaries compiles all AuraGo binaries from source.
func BuildBinaries(cfg *Config) error {
	binDir := filepath.Join(cfg.InstallDir, "bin")

	targets := []struct {
		output string
		pkg    string
	}{
		{"aurago_linux", "./cmd/aurago"},
		{"lifeboat_linux", "./cmd/lifeboat"},
		{"config-merger_linux", "./cmd/config-merger"},
		{"aurago-remote_linux", "./cmd/remote"},
		{"agocli_linux", "./cmd/agocli"},
	}

	ldflags := "-s -w"

	for _, t := range targets {
		cfg.log("  Building " + t.output + "...")
		outPath := filepath.Join(binDir, t.output)
		cmd := exec.Command("go", "build",
			"-trimpath",
			"-ldflags", ldflags,
			"-o", outPath,
			t.pkg,
		)
		cmd.Dir = cfg.InstallDir
		cmd.Env = append(cmd.Environ(),
			"CGO_ENABLED=0",
			"GOOS=linux",
			"GOARCH="+runtime.GOARCH,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			cfg.log("  ERROR building " + t.output + ": " + string(out))
			return err
		}
	}

	cfg.log("All binaries built successfully")
	return nil
}
