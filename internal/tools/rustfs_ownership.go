package tools

import (
	"aurago/internal/dockerutil"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DockerBindUsesRustFSVolume handles named volumes, backing paths and their known host aliases.
func DockerBindUsesRustFSVolume(bind string) bool {
	spec, ok := parseDockerBindMount(bind)
	return ok && dockerPathUsesRustFSVolume(spec.hostPath)
}

func dockerPathUsesRustFSVolume(source string) bool {
	if dockerutil.IsRustFSVolumePath(source) {
		return true
	}
	// This resolves aliases visible in AuraGo's native filesystem; it cannot attest an unseen remote symlink.
	for _, variant := range dockerComposePathVariants(source) {
		if dockerutil.IsRustFSVolumePath(variant) {
			return true
		}
	}
	return false
}

// DockerVolumeUsesRustFS inspects existing named aliases without exposing Engine metadata.
// An absent volume is safe to create; an unverified existing alias is never admitted.
func DockerVolumeUsesRustFS(cfg DockerConfig, name string) (bool, error) {
	if dockerutil.IsRustFSVolumeName(name) {
		return true, nil
	}
	if validateDockerName(name) != nil {
		return false, fmt.Errorf("Docker volume ownership could not be verified: invalid name")
	}
	owner := cfg.Context
	if owner == nil {
		owner = context.Background()
	}
	ctx, cancel := context.WithTimeout(owner, 3*time.Second)
	defer cancel()
	cfg.Context = ctx
	data, status, err := dockerRequest(cfg, http.MethodGet, "/volumes/"+url.PathEscape(name), "")
	if err != nil {
		return false, fmt.Errorf("Docker volume ownership could not be verified")
	}
	if status == http.StatusNotFound {
		return false, nil
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("Docker volume ownership could not be verified (HTTP %d)", status)
	}
	var info struct {
		Name       string
		Driver     string
		Labels     map[string]string
		Mountpoint string
		Options    map[string]string
	}
	if json.Unmarshal(data, &info) != nil || info.Name != name || !dockerBindHostLooksLikePath(info.Mountpoint) {
		return false, fmt.Errorf("Docker volume ownership could not be verified: invalid metadata")
	}
	if dockerComposeVolumeIsLocalBind(DockerComposeNamedVolume{Driver: info.Driver, DriverOpts: info.Options}) && !dockerBindHostLooksLikePath(info.Options["device"]) {
		return false, fmt.Errorf("Docker volume ownership could not be verified: relative backing path")
	}
	return dockerutil.ManagedBy(info.Labels, dockerutil.RustFSOwner) || dockerPathUsesRustFSVolume(info.Mountpoint) || dockerPathUsesRustFSVolume(info.Options["device"]), nil
}

// DockerComposeUsesRustFSVolumeAliases bounds the additional Engine work for resolved named-volume references.
func DockerComposeUsesRustFSVolumeAliases(cfg DockerConfig, models ...DockerComposeModel) (bool, error) {
	names := map[string]bool{}
	for _, model := range models {
		for key, volume := range model.Volumes {
			name := volume.Name
			if name == "" {
				name = key
			}
			if name != "" {
				names[name] = true
			}
		}
		for _, service := range model.Services {
			for _, mount := range service.Volumes {
				if !strings.EqualFold(mount.Type, "volume") || mount.Source == "" {
					continue
				}
				name := mount.Source
				if declared, ok := model.Volumes[name]; ok && declared.Name != "" {
					name = declared.Name
				}
				names[name] = true
			}
		}
	}
	if len(names) > 128 {
		return false, fmt.Errorf("Docker volume ownership could not be verified: too many named references")
	}
	owner := cfg.Context
	if owner == nil {
		owner = context.Background()
	}
	ctx, cancel := context.WithTimeout(owner, 5*time.Second)
	defer cancel()
	cfg.Context = ctx
	for _, name := range sortedDockerComposeKeys(names) {
		owned, err := DockerVolumeUsesRustFS(cfg, name)
		if owned || err != nil {
			return owned, err
		}
	}
	return false, nil
}
