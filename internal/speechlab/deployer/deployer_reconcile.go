package deployer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

// resolveInstalledContainerIDs repairs stale references without installing a
// different bundle or recreating services. Missing initializers may have been
// pruned after the deployment was successfully published.
func (m *Manager) resolveInstalledContainerIDs(ctx context.Context, op operationSnapshot, state State) ([]string, error) {
	failure := func(message string) ([]string, error) {
		return nil, &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("%s; use the managed update action to repair the deployment", message)}
	}
	missing := false
	for _, id := range state.ContainerIDs {
		container, found, err := m.inspectContainer(ctx, op, id)
		if err != nil {
			return nil, err
		}
		if !found {
			missing = true
			continue
		}
		if !dockerutil.ManagedBy(container.Config.Labels, OwnerLabel) || container.Config.Labels["aurago.component"] != "speech-lab" {
			return failure("Installed container ownership does not match Speech Lab")
		}
	}
	if !missing {
		return append([]string(nil), state.ContainerIDs...), nil
	}
	if state.Digest == "" || state.Bundle == "" || state.NetworkID == "" {
		return failure("The installed Speech Lab bundle identity is incomplete")
	}
	manifestURL := m.manifestURL
	if manifestURL == ManifestURL {
		manifestURL = strings.Replace(ManifestURL, "/latest/download/", "/download/"+url.PathEscape(state.Bundle)+"/", 1)
	}
	manifest, raw, err := m.fetchManifestFrom(ctx, manifestURL)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != state.Digest || manifest.BundleVersion != state.Bundle {
		return failure("The available Speech Lab bundle differs from the installed bundle")
	}
	if err := validateManifest(manifest); err != nil {
		return nil, &Error{Code: "speech_lab_bundle_incompatible", Err: err}
	}
	if err := selectSpeechLabASRImage(&manifest, state.GPUBackend); err != nil {
		return nil, &Error{Code: "speech_lab_bundle_incompatible", Err: err}
	}
	// Auto may have published a CPU deployment on a host without GPU devices.
	fingerprints := map[string]bool{speechLabDeploymentFingerprint(state.Digest, state.GPUBackend): true}
	if state.GPUBackend == config.SpeechLabGPUBackendCPU {
		fingerprints[speechLabDeploymentFingerprint(state.Digest, config.SpeechLabGPUBackendAuto)] = true
	}
	ids := make([]string, 0, len(manifest.Services))
	fingerprint := ""
	for _, role := range manifest.StartOrder {
		for _, service := range manifest.Services {
			if service.Role != role {
				continue
			}
			name := "aurago-speech-lab-" + strings.ReplaceAll(role, "_", "-")
			container, found, err := m.inspectContainer(ctx, op, name)
			if err != nil {
				return nil, err
			}
			initializer := role == "model_init" || role == "control_init"
			if !found && initializer {
				continue
			}
			image := imageByKey(manifest.Images, service.Image)
			if image == "" {
				image = imageByKey(manifest.Images, role)
			}
			labels := container.Config.Labels
			if !found || container.ID == "" || !dockerutil.ManagedBy(labels, OwnerLabel) ||
				labels["aurago.component"] != "speech-lab" || labels["aurago.role"] != role ||
				labels["aurago.bundle"] != state.Bundle || container.Config.Image != image ||
				!fingerprints[labels["aurago.fingerprint"]] {
				return failure("A Speech Lab service does not match the installed bundle")
			}
			if fingerprint != "" && labels["aurago.fingerprint"] != fingerprint {
				return failure("Speech Lab services have inconsistent deployment identities")
			}
			fingerprint = labels["aurago.fingerprint"]
			if !initializer {
				attached, err := m.networkContainsContainer(ctx, op, state.NetworkID, container.ID)
				if err != nil {
					return nil, err
				}
				if !attached {
					return failure("A Speech Lab service is detached from the installed network")
				}
			}
			ids = append(ids, container.ID)
		}
	}
	if len(ids) == 0 {
		return failure("The installed Speech Lab services are missing")
	}
	return ids, nil
}
