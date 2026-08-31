package server

import (
	"aurago/internal/virtualcomputers"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var virtualComputersSetupStatePath = "data/virtual_computers_setup_state.json"

var (
	virtualComputersSetupStateMu   sync.Mutex
	virtualComputersLoadSetupState = defaultVirtualComputersLoadSetupState
	virtualComputersSaveSetupState = defaultVirtualComputersSaveSetupState
)

type virtualComputersSetupState struct {
	ConsecutiveFailures       int       `json:"consecutive_failures"`
	LastFailureAt             time.Time `json:"last_failure_at"`
	WorkspaceAssetFingerprint string    `json:"workspace_asset_fingerprint,omitempty"`
	WorkspaceVerifiedAt       time.Time `json:"workspace_verified_at,omitempty"`
}

func virtualComputersWorkspaceSetupVerified() bool {
	state, err := virtualComputersLoadSetupState()
	return err == nil && !state.WorkspaceVerifiedAt.IsZero() && state.WorkspaceAssetFingerprint == virtualcomputers.WorkspaceAssetFingerprint()
}

func virtualComputersRecordWorkspaceSetupVerified() error {
	state, err := virtualComputersLoadSetupState()
	if err != nil {
		return err
	}
	state.WorkspaceAssetFingerprint = virtualcomputers.WorkspaceAssetFingerprint()
	state.WorkspaceVerifiedAt = time.Now().UTC()
	return virtualComputersSaveSetupState(state)
}

func virtualComputersClearWorkspaceSetupVerified() error {
	state, err := virtualComputersLoadSetupState()
	if err != nil {
		return err
	}
	state.WorkspaceAssetFingerprint = ""
	state.WorkspaceVerifiedAt = time.Time{}
	return virtualComputersSaveSetupState(state)
}

func defaultVirtualComputersLoadSetupState() (virtualComputersSetupState, error) {
	virtualComputersSetupStateMu.Lock()
	defer virtualComputersSetupStateMu.Unlock()

	data, err := os.ReadFile(virtualComputersSetupStatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return virtualComputersSetupState{}, nil
		}
		return virtualComputersSetupState{}, fmt.Errorf("load virtual computers setup state: %w", err)
	}
	var state virtualComputersSetupState
	if err := json.Unmarshal(data, &state); err != nil {
		return virtualComputersSetupState{}, fmt.Errorf("parse virtual computers setup state: %w", err)
	}
	return state, nil
}

func defaultVirtualComputersSaveSetupState(state virtualComputersSetupState) error {
	virtualComputersSetupStateMu.Lock()
	defer virtualComputersSetupStateMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(virtualComputersSetupStatePath), 0750); err != nil {
		return fmt.Errorf("create virtual computers setup state directory: %w", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal virtual computers setup state: %w", err)
	}
	if err := os.WriteFile(virtualComputersSetupStatePath, data, 0600); err != nil {
		return fmt.Errorf("write virtual computers setup state: %w", err)
	}
	return nil
}
