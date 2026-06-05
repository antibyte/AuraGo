package desktopstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

func scanInstalledApp(scanner interface{ Scan(dest ...any) error }) (InstalledApp, error) {
	var app InstalledApp
	var tailscaleEnabled int
	var createdAt, updatedAt string
	var portsJSON, volumesJSON, hostBindsJSON, envJSON, extraHostsJSON, secretRefsJSON, companionsJSON string
	err := scanner.Scan(&app.AppID, &app.DesktopAppID, &app.LaunchpadLinkID, &app.ContainerName, &app.ContainerID,
		&app.Image, &app.Status, &app.Error, &app.BindMode, &app.HostIP, &app.HostPort, &app.ContainerPort, &app.Protocol,
		&tailscaleEnabled, &app.TailscaleStatus, &app.TailscalePort, &app.LogoPath, &portsJSON, &volumesJSON, &hostBindsJSON,
		&envJSON, &extraHostsJSON, &secretRefsJSON, &companionsJSON, &createdAt, &updatedAt, &app.LastOperationID,
		&app.LastOperationType, &app.LastOperationState)
	if err != nil {
		return InstalledApp{}, err
	}
	app.TailscaleEnabled = tailscaleEnabled != 0
	app.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	app.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if err := json.Unmarshal([]byte(portsJSON), &app.Ports); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store ports for %s: %w", app.AppID, err)
	}
	if len(app.Ports) == 0 && app.HostPort > 0 {
		app.Ports = []PortBinding{{ID: "main", Name: "Web UI", ContainerPort: app.ContainerPort, Protocol: app.Protocol, HostIP: app.HostIP, HostPort: app.HostPort}}
	}
	if err := json.Unmarshal([]byte(volumesJSON), &app.Volumes); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store volumes for %s: %w", app.AppID, err)
	}
	if err := json.Unmarshal([]byte(hostBindsJSON), &app.HostBinds); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store host binds for %s: %w", app.AppID, err)
	}
	if err := json.Unmarshal([]byte(envJSON), &app.Env); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store env for %s: %w", app.AppID, err)
	}
	if err := json.Unmarshal([]byte(extraHostsJSON), &app.ExtraHosts); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store extra hosts for %s: %w", app.AppID, err)
	}
	secretRefs, err := secretRefsFromStorage(secretRefsJSON)
	if err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store secret refs for %s: %w", app.AppID, err)
	}
	app.SecretRefs = secretRefs
	if err := json.Unmarshal([]byte(companionsJSON), &app.Companions); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store companions for %s: %w", app.AppID, err)
	}
	return app, nil
}

type storedSecretRef struct {
	Key      string `json:"key"`
	VaultKey string `json:"vault_key"`
	Env      string `json:"env,omitempty"`
	Label    string `json:"label,omitempty"`
	Expose   bool   `json:"expose,omitempty"`
}

func secretRefsForStorage(refs []SecretRef) []storedSecretRef {
	stored := make([]storedSecretRef, 0, len(refs))
	for _, ref := range refs {
		stored = append(stored, storedSecretRef{
			Key:      ref.Key,
			VaultKey: ref.VaultKey,
			Env:      ref.Env,
			Label:    ref.Label,
			Expose:   ref.Expose,
		})
	}
	return stored
}

func secretRefsFromStorage(raw string) ([]SecretRef, error) {
	var stored []storedSecretRef
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, err
	}
	refs := make([]SecretRef, 0, len(stored))
	for _, ref := range stored {
		refs = append(refs, SecretRef{
			Key:      ref.Key,
			VaultKey: ref.VaultKey,
			Env:      ref.Env,
			Label:    ref.Label,
			Expose:   ref.Expose,
		})
	}
	return refs, nil
}

func scanOperation(scanner interface{ Scan(dest ...any) error }) (Operation, error) {
	var op Operation
	var createdAt, updatedAt string
	var completed sql.NullString
	err := scanner.Scan(&op.ID, &op.Type, &op.AppID, &op.Status, &op.Message, &op.Error, &op.RequestJSON, &createdAt, &updatedAt, &completed)
	if err != nil {
		return Operation{}, err
	}
	op.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	op.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if completed.Valid {
		if parsed, err := time.Parse(time.RFC3339Nano, completed.String); err == nil {
			op.CompletedAt = &parsed
		}
	}
	return op, nil
}
