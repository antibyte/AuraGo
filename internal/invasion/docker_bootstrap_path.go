package invasion

import (
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// Docker runs Linux regardless of the master's OS. Mirror config.Load's
// config-relative resolution using POSIX paths, without moving existing data.
func dockerEggBootstrapPath(configYAML []byte) (string, error) {
	var cfg struct {
		Directories struct {
			DataDir string `yaml:"data_dir"`
		} `yaml:"directories"`
	}
	cfg.Directories.DataDir = "./data"
	if err := yaml.Unmarshal(configYAML, &cfg); err != nil {
		return "", fmt.Errorf("parse Egg configuration: %w", err)
	}
	dir := cfg.Directories.DataDir
	if dir == "" || strings.ContainsAny(dir, "\\\x00\r\n") {
		return "", fmt.Errorf("invalid Docker Egg data directory")
	}
	if !path.IsAbs(dir) {
		dir = path.Join(dockerEggConfigArchivePath, dir)
	}
	dir = path.Clean(dir)
	if dir != dockerEggConfigArchivePath && !strings.HasPrefix(dir, dockerEggConfigArchivePath+"/") {
		return "", fmt.Errorf("Docker Egg data directory must stay inside %s", dockerEggConfigArchivePath)
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(dir, dockerEggConfigArchivePath), "/")
	if rel == dockerEggConfigFileName || strings.HasPrefix(rel, dockerEggConfigFileName+"/") {
		return "", fmt.Errorf("Docker Egg data directory conflicts with config.yaml")
	}
	return path.Join(rel, EggBootstrapFile), nil
}
