package virtualcomputers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// MachineMeta is the AuraGo-owned metadata of a machine (boringd ignores it).
type MachineMeta struct {
	Label    string
	VolumeID string
}

// SetMachineLabel stores the user label of a machine without touching other columns.
func (l *Ledger) SetMachineLabel(ctx context.Context, id, label string) error {
	id = strings.TrimSpace(id)
	if l == nil || l.db == nil || id == "" {
		return nil
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO machines (id, label, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET label = excluded.label`, id, label, nowText())
	if err != nil {
		return fmt.Errorf("set virtual computer machine label: %w", err)
	}
	return nil
}

// SetMachineVolume records which volume a machine was launched with.
func (l *Ledger) SetMachineVolume(ctx context.Context, id, volumeID string) error {
	id = strings.TrimSpace(id)
	if l == nil || l.db == nil || id == "" {
		return nil
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO machines (id, volume_id, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET volume_id = excluded.volume_id`, id, strings.TrimSpace(volumeID), nowText())
	if err != nil {
		return fmt.Errorf("set virtual computer machine volume: %w", err)
	}
	return nil
}

// MachineMetadata returns label and volume of every machine row that carries any.
func (l *Ledger) MachineMetadata(ctx context.Context) (map[string]MachineMeta, error) {
	out := map[string]MachineMeta{}
	if l == nil || l.db == nil {
		return out, nil
	}
	rows, err := l.db.QueryContext(ctx, `SELECT id, label, volume_id FROM machines WHERE label <> '' OR volume_id <> ''`)
	if err != nil {
		return nil, fmt.Errorf("list virtual computer machine metadata: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var meta MachineMeta
		if err := rows.Scan(&id, &meta.Label, &meta.VolumeID); err != nil {
			return nil, fmt.Errorf("scan virtual computer machine metadata: %w", err)
		}
		out[id] = meta
	}
	return out, rows.Err()
}

// PruneMachines deletes ledger rows of machines that are not in keep.
func (l *Ledger) PruneMachines(ctx context.Context, keep []string) error {
	if l == nil || l.db == nil {
		return nil
	}
	keepSet := make(map[string]struct{}, len(keep))
	for _, id := range keep {
		keepSet[strings.TrimSpace(id)] = struct{}{}
	}
	rows, err := l.db.QueryContext(ctx, `SELECT id FROM machines`)
	if err != nil {
		return fmt.Errorf("list virtual computer machine rows: %w", err)
	}
	var stale []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan virtual computer machine row: %w", err)
		}
		if _, ok := keepSet[id]; !ok {
			stale = append(stale, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range stale {
		if _, err := l.db.ExecContext(ctx, `DELETE FROM machines WHERE id = ?`, id); err != nil {
			return fmt.Errorf("prune virtual computer machine row: %w", err)
		}
	}
	return nil
}

// WorkspaceByMachine returns the workspace row bound to a machine in any state.
func (l *Ledger) WorkspaceByMachine(ctx context.Context, machineID string) (Workspace, bool, error) {
	if l == nil || l.db == nil {
		return Workspace{}, false, fmt.Errorf("virtual computers ledger is not open")
	}
	row := l.db.QueryRowContext(ctx, `SELECT `+workspaceSelectColumns+` FROM workspaces WHERE machine_id = ?`, strings.TrimSpace(machineID))
	workspace, err := scanWorkspace(row)
	if err == sql.ErrNoRows {
		return Workspace{}, false, nil
	}
	if err != nil {
		return Workspace{}, false, fmt.Errorf("get virtual workspace by machine: %w", err)
	}
	return workspace, true, nil
}

// ActiveWorkspaceForMachine returns the opening/ready/closing workspace of a machine.
func (l *Ledger) ActiveWorkspaceForMachine(ctx context.Context, machineID string) (Workspace, bool, error) {
	workspace, ok, err := l.WorkspaceByMachine(ctx, machineID)
	if err != nil || !ok {
		return Workspace{}, false, err
	}
	if !workspaceStateActive(workspace.State) {
		return Workspace{}, false, nil
	}
	return workspace, true, nil
}

func workspaceStateActive(state string) bool {
	switch state {
	case WorkspaceStateOpening, WorkspaceStateReady, WorkspaceStateClosing:
		return true
	}
	return false
}

// UpdateWorkspaceMaxExpiry moves the hard expiry of an active workspace.
func (l *Ledger) UpdateWorkspaceMaxExpiry(ctx context.Context, id string, expiry time.Time) error {
	if l == nil || l.db == nil {
		return fmt.Errorf("virtual computers ledger is not open")
	}
	_, err := l.db.ExecContext(ctx, `UPDATE workspaces SET max_expires_at = ?, updated_at = ?
		WHERE id = ? AND state IN (?, ?, ?)`, timeText(expiry), nowText(), strings.TrimSpace(id),
		WorkspaceStateOpening, WorkspaceStateReady, WorkspaceStateClosing)
	if err != nil {
		return fmt.Errorf("update workspace max expiry: %w", err)
	}
	return nil
}
