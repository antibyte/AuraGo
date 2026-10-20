package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"
	"gopkg.in/yaml.v3"
)

type go2RTCTuyaInputs struct {
	Accounts map[string]map[string]string
	Devices  map[string]string
}

func readTuyaCredentials(vault *security.Vault, id string) (config.Go2RTCTuyaCredentials, error) {
	var c config.Go2RTCTuyaCredentials
	if vault == nil {
		return c, fmt.Errorf("Tuya requires the secrets vault")
	}
	value, err := vault.ReadSecret(config.Go2RTCTuyaAccountKey(id))
	if err != nil {
		return c, err
	}
	if json.Unmarshal([]byte(value), &c) != nil {
		return c, fmt.Errorf("invalid saved Tuya credentials")
	}
	return c, nil
}

func tuyaNewRevision() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

// prepareTuyaChanges runs under CfgSaveMu in both save paths. It does no I/O to
// Tuya or Docker and writes nothing until every account and binding is valid.
func prepareTuyaChanges(old config.Go2RTCConfig, next *config.Go2RTCConfig, vault *security.Vault, inputs go2RTCTuyaInputs, sources []go2RTCSourceChange) ([]go2RTCSourceChange, error) {
	oldAccounts := map[string]config.Go2RTCTuyaAccount{}
	for _, a := range old.TuyaAccounts {
		oldAccounts[a.ID] = a
	}
	accounts := map[string]config.Go2RTCTuyaAccount{}
	credentials := map[string]config.Go2RTCTuyaCredentials{}
	changes := append([]go2RTCSourceChange(nil), sources...)
	wantedCameras := map[string]bool{}
	for _, camera := range next.Streams {
		wantedCameras[camera.ID] = true
	}
	for _, previous := range old.Streams {
		if !wantedCameras[previous.ID] {
			for _, role := range []string{"main", "sub", "snapshot"} {
				changes = append(changes, go2RTCSourceChange{ID: previous.ID, Role: role, Delete: true})
			}
			continue
		}
		if previous.Tuya == nil {
			continue
		}
		for _, camera := range next.Streams {
			if camera.ID != previous.ID || camera.Tuya != nil {
				continue
			}
			replaced := false
			explicitRoles := map[string]bool{}
			for _, change := range sources {
				if change.ID == camera.ID {
					role := change.Role
					if role == "" {
						role = "main"
					}
					explicitRoles[role] = true
					if role == "main" && !change.Delete && change.Value != "" {
						replaced = true
					}
				}
			}
			if !replaced {
				return nil, fmt.Errorf("removing a Tuya binding requires replacing the camera source or deleting the camera")
			}
			for _, role := range []string{"sub", "snapshot"} {
				if !explicitRoles[role] {
					changes = append(changes, go2RTCSourceChange{ID: camera.ID, Role: role, Delete: true})
				}
			}
		}
	}
	for i, a := range next.TuyaAccounts {
		if config.Go2RTCTuyaAccountKey(a.ID) == "" || accounts[a.ID].ID != "" {
			return nil, fmt.Errorf("invalid or duplicate Tuya account ID")
		}
		previous, exists := oldAccounts[a.ID]
		var c, before config.Go2RTCTuyaCredentials
		if exists {
			var err error
			before, err = readTuyaCredentials(vault, a.ID)
			if err != nil {
				return nil, fmt.Errorf("saved Tuya credentials cannot be read")
			}
			if previous.Method == a.Method {
				c = before
			}
		}
		fields := map[string]*string{"email": &c.Email, "password": &c.Password, "client_id": &c.ClientID, "client_secret": &c.ClientSecret, "uid": &c.UID}
		for field, value := range inputs.Accounts[a.ID] {
			dst := fields[field]
			if dst == nil {
				return nil, fmt.Errorf("unsupported Tuya credential field")
			}
			if value != "" && value != "••••••••" {
				*dst = value
				security.RegisterSensitive(value)
				security.RegisterSensitive(url.QueryEscape(value))
			}
		}
		if err := config.ValidateGo2RTCTuyaCredentials(a.Method, c); err != nil {
			return nil, err
		}
		a.Revision = previous.Revision
		if !exists || a.Method != previous.Method || a.Region != previous.Region || !reflect.DeepEqual(c, before) || a.Revision == "" {
			var err error
			a.Revision, err = tuyaNewRevision()
			if err != nil {
				return nil, err
			}
			encoded, _ := json.Marshal(c)
			changes = append(changes, go2RTCSourceChange{AccountID: a.ID, Value: string(encoded)})
		}
		next.TuyaAccounts[i] = a
		accounts[a.ID], credentials[a.ID] = a, c
	}
	for _, camera := range next.Streams {
		if camera.Tuya != nil && accounts[camera.Tuya.AccountID].ID == "" {
			return nil, &go2RTCMutationError{Status: 409, Code: "tuya_account_in_use", Message: fmt.Sprintf("Tuya account is missing or still used by camera %q", camera.ID)}
		}
	}
	for id := range oldAccounts {
		if accounts[id].ID == "" {
			changes = append(changes, go2RTCSourceChange{AccountID: id, Delete: true})
		}
	}
	for i := range next.Streams {
		camera := &next.Streams[i]
		if camera.Tuya == nil {
			continue
		}
		b := *camera.Tuya
		if b.MainResolution == "" {
			b.MainResolution = "hd"
		}
		camera.Tuya = &b
		device := inputs.Devices[camera.ID]
		if device == "" || device == "••••••••" {
			var source string
			if vault != nil {
				source, _ = vault.ReadSecret(config.Go2RTCSourceKey(camera.ID, "main"))
			}
			if config.ValidateGo2RTCTuyaSource(source) == nil {
				u, _ := url.Parse(source)
				device = u.Query().Get("device_id")
			}
		}
		for _, change := range sources {
			if change.ID == camera.ID && !change.Delete && change.Value != "" {
				return nil, fmt.Errorf("Tuya camera sources are generated from the selected account")
			}
		}
		for _, role := range []string{"main", "sub", "snapshot"} {
			resolution := b.MainResolution
			if role == "sub" {
				resolution = b.SubResolution
			}
			if role == "snapshot" || resolution == "" {
				changes = append(changes, go2RTCSourceChange{ID: camera.ID, Role: role, Delete: true})
				continue
			}
			source, err := config.Go2RTCTuyaSource(accounts[b.AccountID], credentials[b.AccountID], device, resolution)
			if err != nil {
				return nil, err
			}
			security.RegisterSensitive(source)
			changes = append(changes, go2RTCSourceChange{ID: camera.ID, Role: role, Value: source})
		}
	}
	if err := config.ValidateGo2RTCTuya(*next); err != nil {
		return nil, err
	}
	return changes, nil
}

// prepareTuyaConfigPatch converts the generic config patch to the same typed
// transaction as camera mutations, stripping write-only values before YAML.
func prepareTuyaConfigPatch(s *Server, raw, patch map[string]interface{}, txn *go2RTCSourceVaultTransaction) error {
	section, ok := patch["go2rtc"].(map[string]interface{})
	if !ok {
		return nil
	}
	for _, field := range []string{"streams", "tuya_accounts"} {
		if numeric, ok := section[field].(map[string]interface{}); ok {
			list, dense := numericKeyedMapToSlice(numeric)
			if !dense {
				return fmt.Errorf("incomplete Tuya configuration list")
			}
			section[field] = list
		}
	}
	inputs := go2RTCTuyaInputs{Accounts: map[string]map[string]string{}, Devices: map[string]string{}}
	if value, present := section["tuya_accounts"]; present {
		items, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("go2rtc.tuya_accounts must be a complete list")
		}
		for _, item := range items {
			a, ok := item.(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid Tuya account")
			}
			id, _ := a["id"].(string)
			fields := map[string]string{}
			for _, field := range []string{"email", "password", "client_id", "client_secret", "uid"} {
				if value, exists := a[field]; exists {
					v, ok := value.(string)
					if !ok {
						return fmt.Errorf("invalid Tuya credential field")
					}
					fields[field] = v
					delete(a, field)
				}
			}
			delete(a, "credentials_configured")
			inputs.Accounts[id] = fields
		}
	}
	if items, ok := section["streams"].([]interface{}); ok {
		for _, item := range items {
			camera, _ := item.(map[string]interface{})
			id, _ := camera["id"].(string)
			if b, ok := camera["tuya"].(map[string]interface{}); ok {
				if raw, exists := b["device_id"]; exists {
					device, ok := raw.(string)
					if !ok {
						return fmt.Errorf("invalid Tuya device")
					}
					inputs.Devices[id] = device
					delete(b, "device_id")
				}
				delete(b, "device_configured")
			}
		}
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	var merged map[string]interface{}
	if err := yaml.Unmarshal(data, &merged); err != nil {
		return err
	}
	merged = normalizeConfigYAMLMap(merged)
	deepMerge(merged, map[string]interface{}{"go2rtc": section}, "")
	data, err = yaml.Marshal(merged["go2rtc"])
	if err != nil {
		return err
	}
	var next config.Go2RTCConfig
	if err := yaml.Unmarshal(data, &next); err != nil {
		return err
	}
	var sourceInputs []go2RTCSourceChange
	if items, ok := section["streams"].([]interface{}); ok {
		// This transaction owns manual and generated role changes. Strip their
		// inputs so legacy secret extraction cannot replay a clear afterwards.
		sourceInputs, err = go2RTCSourceChanges(items, true)
		if err != nil {
			return err
		}
	}
	changes, err := prepareTuyaChanges(s.ConfigSnapshot().Go2RTC, &next, s.Vault, inputs, sourceInputs)
	if err != nil {
		return err
	}
	if err := txn.apply(changes); err != nil {
		return err
	}
	// Publish server-owned revisions and normalized bindings, preserving unrelated
	// patch fields (including the existing manual-source extraction path).
	if _, present := section["tuya_accounts"]; present {
		data, _ := yaml.Marshal(next.TuyaAccounts)
		var accounts []interface{}
		_ = yaml.Unmarshal(data, &accounts)
		section["tuya_accounts"] = accounts
	}
	if items, ok := section["streams"].([]interface{}); ok {
		for i, item := range items {
			if i < len(next.Streams) && next.Streams[i].Tuya != nil {
				item.(map[string]interface{})["tuya"] = go2RTCDetectionProjection(next.Streams[i].Tuya)
			}
		}
	}
	return nil
}

func (t *go2RTCSourceVaultTransaction) apply(changes []go2RTCSourceChange) error {
	if len(changes) == 0 {
		return nil
	}
	if t.vault == nil {
		return fmt.Errorf("Tuya requires the secrets vault")
	}
	// Only the final operation for a key is published. Earlier operations must
	// neither remove a regenerated role nor leave an intermediate secret behind.
	last := make(map[string]int, len(changes))
	for i, change := range changes {
		key := go2RTCChangeKey(change)
		if key == "" {
			return fmt.Errorf("invalid camera or account ID")
		}
		last[key] = i
	}
	for i, change := range changes {
		key := go2RTCChangeKey(change)
		if last[key] != i {
			continue
		}
		current := go2RTCSourceVaultSnapshot{}
		value, err := t.vault.ReadSecret(key)
		if err == nil {
			current = go2RTCSourceVaultSnapshot{value: value, exists: true}
		} else if !strings.Contains(strings.ToLower(err.Error()), "secret not found") {
			return fmt.Errorf("saved credentials cannot be read")
		}
		if _, ok := t.previous[key]; !ok {
			t.previous[key] = current
		}
		if (change.Delete && !current.exists) || (!change.Delete && current.exists && current.value == change.Value) {
			continue
		}
		if change.Delete {
			if err := t.vault.DeleteSecret(key); err != nil {
				return fmt.Errorf("credential could not be removed")
			}
		} else {
			if change.Value == "" {
				return fmt.Errorf("empty camera credential")
			}
			security.RegisterSensitive(change.Value)
			if err := t.vault.WriteSecret(key, change.Value); err != nil {
				return fmt.Errorf("credential could not be saved")
			}
		}
	}
	return nil
}

func go2RTCChangeKey(c go2RTCSourceChange) string {
	if c.AccountID != "" {
		if c.ID != "" {
			return ""
		}
		return config.Go2RTCTuyaAccountKey(c.AccountID)
	}
	return config.Go2RTCSourceKey(c.ID, c.Role)
}
