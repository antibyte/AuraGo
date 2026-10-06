package desktopstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The install journal records what one install attempt created, so a failed,
// blocked or interrupted install removes only those resources. Retained data
// (volumes, Vault secrets and workspace files an uninstall kept) and foreign
// containers are never touched.
const (
	installResourceAttempt   = "attempt" // marker: the app's install attempts use the journal
	installResourceContainer = "container"
	installResourceVolume    = "volume"
	installResourceNetwork   = "network"
	installResourceSecret    = "secret"
	installResourceWorkspace = "workspace"
)

// OperationErrorContainerNameInUse is the operation error code of
// ContainerNameConflictError; the Desktop shows desktop.store.error_<code>.
const OperationErrorContainerNameInUse = "container_name_in_use"

// ContainerNameConflictError stops an install before anything is created: a
// container with a name the install needs exists and was not created by the
// Software Store for this app.
type ContainerNameConflictError struct {
	AppID     string
	Container string
}

func (e *ContainerNameConflictError) Error() string {
	return fmt.Sprintf("container %s already exists and was not created by the Software Store for %s; rename it (docker rename %s %s-old) or remove it, then install again",
		e.Container, e.AppID, e.Container, e.Container)
}

// operationErrorDetails returns the UI error code and parameters of a failed
// operation, or "" for errors the UI shows as text.
func operationErrorDetails(err error) (string, map[string]string) {
	var conflict *ContainerNameConflictError
	if errors.As(err, &conflict) {
		return OperationErrorContainerNameInUse, map[string]string{"name": conflict.Container, "app": conflict.AppID}
	}
	return "", nil
}

type installResource struct {
	Kind string
	Name string
}

func (s *Service) recordInstallResource(ctx context.Context, appID, kind, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	return execWithSQLiteRetry(ctx, "record desktop store install resource", func() error {
		_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO desktop_store_install_resources(app_id, kind, name, created_at)
			VALUES(?, ?, ?, ?)`, appID, kind, name, formatTime(time.Now().UTC()))
		return err
	})
}

func (s *Service) installResources(ctx context.Context, appID string) ([]installResource, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT kind, name FROM desktop_store_install_resources
		WHERE app_id = ? ORDER BY created_at, kind, name`, appID)
	if err != nil {
		return nil, fmt.Errorf("load desktop store install resources: %w", err)
	}
	defer rows.Close()
	var out []installResource
	for rows.Next() {
		var item installResource
		if err := rows.Scan(&item.Kind, &item.Name); err != nil {
			return nil, fmt.Errorf("scan desktop store install resource: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read desktop store install resources: %w", err)
	}
	return out, nil
}

func (s *Service) forgetInstallResources(ctx context.Context, appID string, resources []installResource) error {
	for _, item := range resources {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM desktop_store_install_resources WHERE app_id = ? AND kind = ? AND name = ?`,
			appID, item.Kind, item.Name); err != nil {
			return fmt.Errorf("forget desktop store install resource: %w", err)
		}
	}
	return nil
}

// forgetInstallResourcesIfOpen is forgetInstallResources for background work:
// it holds the service lock, so it never races Close, and does nothing once
// the service is closed.
func (s *Service) forgetInstallResourcesIfOpen(ctx context.Context, appID string, resources []installResource) {
	if len(resources) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.db == nil {
		return
	}
	if err := s.forgetInstallResources(ctx, appID, resources); err != nil {
		s.logger().Warn("Store install cleanup could not update its journal", "app_id", appID, "error", err)
	}
}

func (s *Service) clearInstallResources(ctx context.Context, appID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM desktop_store_install_resources WHERE app_id = ?`, appID); err != nil {
		return fmt.Errorf("clear desktop store install resources: %w", err)
	}
	return nil
}

func hasInstallAttempt(resources []installResource) bool {
	for _, item := range resources {
		if item.Kind == installResourceAttempt {
			return true
		}
	}
	return false
}

// recordAbsentWorkspaceBinds records the managed workspace directories that do
// not exist yet, before prepareManagedWorkspaceBinds creates them.
func (s *Service) recordAbsentWorkspaceBinds(ctx context.Context, app InstalledApp) error {
	for _, bind := range app.HostBinds {
		if !bind.Managed {
			continue
		}
		if _, err := os.Lstat(bind.HostPath); err == nil || !os.IsNotExist(err) {
			continue
		}
		if err := s.recordInstallResource(ctx, app.AppID, installResourceWorkspace, bind.WorkspacePath); err != nil {
			return err
		}
	}
	return nil
}

// recordAbsentGeneratedSecrets records the generated-secret Vault keys that do
// not exist yet, before installEnv writes them.
func (s *Service) recordAbsentGeneratedSecrets(ctx context.Context, entry CatalogEntry) error {
	if s.cfg.Secrets == nil || entry.ID == GodsEyeAppID {
		return nil
	}
	for _, secret := range entry.GeneratedSecrets {
		key := strings.ToLower(strings.TrimSpace(secret.Key))
		if key == "" {
			continue
		}
		vaultKey := storeSecretVaultKey(entry.ID, key)
		if value, err := s.cfg.Secrets.ReadSecret(vaultKey); err == nil && value != "" {
			continue
		}
		if err := s.recordInstallResource(ctx, entry.ID, installResourceSecret, vaultKey); err != nil {
			return err
		}
	}
	return nil
}

// preflightInstall checks the names an install will use, before the record is
// saved and before anything is created in Docker.
//   - A container with a target name stops the install unless it carries the
//     Store labels of exactly this app or companion: then it is the leftover of
//     an earlier attempt and is removed (never its volumes).
//   - Volumes and networks that do not exist yet are recorded as created by
//     this attempt; existing ones are reused as before and never removed by a
//     failed install.
func (s *Service) preflightInstall(ctx context.Context, entry CatalogEntry, app InstalledApp) error {
	docker := s.requireDocker()
	targets := []struct{ name, companionID string }{{app.ContainerName, ""}}
	for _, companion := range app.Companions {
		targets = append(targets, struct{ name, companionID string }{companion.ContainerName, companion.ID})
	}
	// Check every name before changing anything, so a conflict leaves Docker
	// untouched.
	var free, leftovers []string
	for _, target := range targets {
		name := strings.TrimSpace(target.name)
		if name == "" {
			continue
		}
		state, found, err := docker.FindContainer(ctx, name)
		if err != nil {
			// Unknown: the container is recorded only once this attempt has
			// created it, so a failed install never removes it.
			s.logger().Warn("Store install could not check the container name", "app_id", app.AppID, "container", name, "error", err)
			continue
		}
		if !found {
			free = append(free, name)
			continue
		}
		if !isStoreLeftover(state.Labels, app.AppID, target.companionID) {
			return &ContainerNameConflictError{AppID: app.AppID, Container: name}
		}
		leftovers = append(leftovers, name)
	}
	for _, name := range leftovers {
		s.logger().Warn("Store install removes a leftover container of an earlier attempt", "app_id", app.AppID, "container", name)
		_ = docker.StopContainer(ctx, name)
		if err := docker.RemoveContainer(ctx, name, true); err != nil {
			return fmt.Errorf("remove leftover container %s: %w", name, err)
		}
		free = append(free, name)
	}
	for _, name := range free {
		if err := s.recordInstallResource(ctx, app.AppID, installResourceContainer, name); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	volumes := append([]VolumeBinding(nil), app.Volumes...)
	for _, companion := range app.Companions {
		volumes = append(volumes, companion.Volumes...)
	}
	for _, volume := range volumes {
		name := strings.TrimSpace(volume.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		exists, err := docker.VolumeExists(ctx, name)
		if err != nil {
			s.logger().Warn("Store install could not check the volume; it is kept if the install fails", "app_id", app.AppID, "volume", name, "error", err)
			continue
		}
		if exists {
			continue
		}
		if err := s.recordInstallResource(ctx, app.AppID, installResourceVolume, name); err != nil {
			return err
		}
	}
	for _, network := range installNetworks(entry, app) {
		exists, err := docker.NetworkExists(ctx, network)
		if err != nil {
			s.logger().Warn("Store install could not check the network; it is kept if the install fails", "app_id", app.AppID, "network", network, "error", err)
			continue
		}
		if exists {
			continue
		}
		if err := s.recordInstallResource(ctx, app.AppID, installResourceNetwork, network); err != nil {
			return err
		}
	}
	return nil
}

// isStoreLeftover reports whether labels mark a container the Software Store
// created for this app (companionID "") or this companion.
func isStoreLeftover(labels map[string]string, appID, companionID string) bool {
	if labels["aurago.desktop_store"] != "true" || normalizeAppID(labels["aurago.desktop_store.app_id"]) != normalizeAppID(appID) {
		return false
	}
	return strings.TrimSpace(labels["aurago.desktop_store.companion"]) == strings.TrimSpace(companionID)
}

// installNetworks lists the private networks an install uses.
func installNetworks(entry CatalogEntry, app InstalledApp) []string {
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] || !isPrivateStoreNetwork(name) {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	add(privateStoreNetworkName(entry))
	for _, companion := range app.Companions {
		add(companion.NetworkMode)
	}
	return out
}

// cleanupInstallResources removes what the journal says this app's failed
// install attempts created and updates the journal: rows whose removal failed
// stay for the next cleanup; when everything is gone the journal is cleared.
func (s *Service) cleanupInstallResources(ctx context.Context, appID string, resources []installResource) {
	removed, failed := s.removeJournaledDockerResources(ctx, appID, resources)
	localRemoved, localFailed := s.removeJournaledLocalResources(ctx, appID, resources)
	removed = append(removed, localRemoved...)
	if !failed && !localFailed {
		if err := s.clearInstallResources(ctx, appID); err != nil {
			s.logger().Warn("Store install cleanup could not clear its journal", "app_id", appID, "error", err)
		}
		return
	}
	if err := s.forgetInstallResources(ctx, appID, removed); err != nil {
		s.logger().Warn("Store install cleanup could not update its journal", "app_id", appID, "error", err)
	}
}

// cleanupBlockedInstall runs when the preflight stopped an install. It removes
// the journaled Vault secrets and workspace directories but no Docker resource:
// a container name an earlier attempt journaled may now belong to the foreign
// container that blocked this one.
func (s *Service) cleanupBlockedInstall(ctx context.Context, appID string) {
	resources, err := s.installResources(ctx, appID)
	if err != nil {
		s.logger().Warn("Store install cleanup could not read its journal", "app_id", appID, "error", err)
		return
	}
	removed, failed := s.removeJournaledLocalResources(ctx, appID, resources)
	dockerRows := false
	for _, item := range resources {
		switch item.Kind {
		case installResourceContainer, installResourceVolume, installResourceNetwork:
			dockerRows = true
		}
	}
	if !failed && !dockerRows {
		if err := s.clearInstallResources(ctx, appID); err != nil {
			s.logger().Warn("Store install cleanup could not clear its journal", "app_id", appID, "error", err)
		}
		return
	}
	if err := s.forgetInstallResources(ctx, appID, removed); err != nil {
		s.logger().Warn("Store install cleanup could not update its journal", "app_id", appID, "error", err)
	}
}

// removeJournaledDockerResources removes the journaled containers first, then
// volumes and networks. It does not touch the database, so the interrupted
// install recovery can run it in the background. removed lists the rows that
// are settled: removed, already gone, or a container name that now belongs to
// another container, which is never removed.
func (s *Service) removeJournaledDockerResources(ctx context.Context, appID string, resources []installResource) (removed []installResource, failed bool) {
	docker := s.requireDocker()
	for _, kind := range []string{installResourceContainer, installResourceVolume, installResourceNetwork} {
		for _, item := range resources {
			if item.Kind != kind {
				continue
			}
			var err error
			switch kind {
			case installResourceContainer:
				// The preflight journals a free name before the create; another
				// container can take it in between (image pulls take a while).
				state, found, findErr := docker.FindContainer(ctx, item.Name)
				switch {
				case findErr != nil:
					err = fmt.Errorf("check container %s: %w", item.Name, findErr)
				case !found:
				case !isAppStoreContainer(state.Labels, appID):
					s.logger().Warn("Store install cleanup leaves a container it did not create", "app_id", appID, "container", item.Name)
				default:
					_ = docker.StopContainer(ctx, item.Name)
					err = docker.RemoveContainer(ctx, item.Name, true)
				}
			case installResourceVolume:
				err = docker.RemoveVolume(ctx, item.Name, true)
			case installResourceNetwork:
				err = docker.RemoveNetwork(ctx, item.Name)
			}
			if err != nil {
				failed = true
				s.logger().Warn("Store install cleanup could not remove a resource", "app_id", appID, "kind", item.Kind, "name", item.Name, "error", err)
				continue
			}
			removed = append(removed, item)
		}
	}
	return removed, failed
}

// removeJournaledLocalResources deletes the journaled Vault secrets and
// workspace directories.
func (s *Service) removeJournaledLocalResources(ctx context.Context, appID string, resources []installResource) (removed []installResource, failed bool) {
	for _, item := range resources {
		var err error
		switch item.Kind {
		case installResourceSecret:
			if s.cfg.Secrets == nil {
				continue
			}
			err = s.cfg.Secrets.DeleteSecret(item.Name)
		case installResourceWorkspace:
			err = s.removeWorkspaceBindDir(item.Name)
		default:
			continue
		}
		if err != nil {
			failed = true
			s.logger().Warn("Store install cleanup could not remove a resource", "app_id", appID, "kind", item.Kind, "name", item.Name, "error", err)
			continue
		}
		removed = append(removed, item)
	}
	_ = ctx
	return removed, failed
}

// removeWorkspaceBindDir removes one managed workspace directory an install
// attempt created, with the same path checks as removeManagedWorkspaceBinds.
func (s *Service) removeWorkspaceBindDir(workspacePath string) error {
	hostPath, _, err := resolveWorkspaceBindPath(s.cfg.WorkspaceDir, workspacePath)
	if err != nil {
		return fmt.Errorf("resolve managed workspace bind %s: %w", workspacePath, err)
	}
	if err := os.RemoveAll(filepath.Clean(hostPath)); err != nil {
		return fmt.Errorf("remove workspace bind %s: %w", workspacePath, err)
	}
	return nil
}
