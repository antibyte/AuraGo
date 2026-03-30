package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// MergeConfig runs the config-merger binary to merge the updated template
// into the user's existing config.yaml.
func MergeConfig(cfg *Config) error {
	mergerPath := filepath.Join(cfg.InstallDir, "bin", "config-merger")
	if runtime.GOOS == "linux" {
		if p := mergerPath + "_linux"; fileExists(p) {
			mergerPath = p
		}
	}

	if !fileExists(mergerPath) {
		cfg.log("Config-merger binary not found — skipping")
		return nil
	}

	templatePath := filepath.Join(cfg.InstallDir, "config_template.yaml")
	if !fileExists(templatePath) {
		// Binary mode: template might be config.yaml.new_template
		altPath := filepath.Join(cfg.InstallDir, "config.yaml.new_template")
		if fileExists(altPath) {
			templatePath = altPath
		} else {
			cfg.log("Config template not found — skipping merge")
			return nil
		}
	}

	configPath := filepath.Join(cfg.InstallDir, "config.yaml")

	cmd := exec.Command(mergerPath, "-template", templatePath, "-output", configPath)
	cmd.Dir = cfg.InstallDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		cfg.log("Config-merger output: " + string(out))
		return err
	}
	cfg.log("Configuration merged successfully")
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
