package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// Native provider mutations use the same journal, including remotely duplicated pages.
// A remote-only identity deliberately has no invented local directory.
func ExecuteHomepageProviderOperation(cfg HomepageConfig, db *sql.DB, provider, target, account, operation string, op HomepageOperation, logger *slog.Logger, execute func() string) string {
	switch operation {
	case "duplicate", "update_metadata", "update_access", "set_password", "remove_password", "restore_version", "delete_site", "delete_version":
	default:
		return strings.TrimPrefix(execute(), "Tool Output: ")
	}
	if db == nil {
		return ErrorJSON("homepage registry is required to account for a website provider mutation")
	}
	if strings.TrimSpace(op.Reason) == "" {
		return ErrorJSON("reason is required before a website provider mutation")
	}
	if target == "" {
		return ErrorJSON("provider site identity is required")
	}
	var projectID int64
	selectedAccount := ""
	rows, err := db.Query("SELECT project_id,metadata_json FROM homepage_deploy_targets WHERE provider=? AND provider_target_id=?", provider, target)
	if err != nil {
		return ErrorJSON(err.Error())
	}
	for rows.Next() {
		var id int64
		var metadata string
		if err := rows.Scan(&id, &metadata); err != nil {
			rows.Close()
			return ErrorJSON(err.Error())
		}
		parsed := map[string]interface{}{}
		json.Unmarshal([]byte(metadata), &parsed)
		knownAccount := firstLedgerString(parsed, "account_id", "account")
		if account != "" && knownAccount != "" && account != knownAccount {
			continue
		}
		if projectID > 0 && projectID != id {
			rows.Close()
			return ErrorJSON("provider target maps to several projects; select an explicit account")
		}
		if account == "" && selectedAccount != "" && knownAccount != "" && selectedAccount != knownAccount {
			rows.Close()
			return ErrorJSON("provider target maps to several accounts; select an explicit account")
		}
		if knownAccount != "" {
			selectedAccount = knownAccount
		}
		projectID = id
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ErrorJSON(err.Error())
	}
	if account == "" {
		account = selectedAccount
	}
	if projectID == 0 {
		name := provider + ":" + account + ":" + target
		if existing, err := GetProjectByName(db, name); err == nil {
			projectID = existing.ID
		} else {
			result, err := db.Exec("INSERT INTO homepage_projects(name,project_dir,origin) VALUES(?,'','unknown')", name)
			if err != nil {
				return ErrorJSON(err.Error())
			}
			projectID, err = result.LastInsertId()
			if err != nil {
				return ErrorJSON(err.Error())
			}
		}
	}
	metadata, _ := json.Marshal(map[string]string{"account": account})
	key := homepageDeploymentTargetKey(HomepageDeploymentRecord{Provider: provider, ProviderTargetID: target, Metadata: map[string]interface{}{"account": account}})
	if _, err := db.Exec("INSERT INTO homepage_deploy_targets(project_id,provider,provider_target_id,remote_path,metadata_json,target_key,status) VALUES(?,?,?,?,?,?,'unknown') ON CONFLICT(project_id,provider,target_key) DO NOTHING", projectID, provider, target, provider+":"+account+":"+target, string(metadata), key); err != nil {
		return ErrorJSON(err.Error())
	}
	op.Name = "remote_" + operation
	op.ProjectID = projectID
	op.FilesMayChange = false
	return ExecuteHomepageOperation(cfg, db, op, logger, func(c HomepageConfig) string {
		raw := strings.TrimPrefix(execute(), "Tool Output: ")
		parsed := map[string]interface{}{}
		if json.Unmarshal([]byte(raw), &parsed) != nil || parsed == nil {
			return raw
		}
		if _, ok := parsed["status"]; !ok {
			parsed["status"] = "ok"
		}
		encoded, _ := json.Marshal(parsed)
		_, outcome := homepageOperationOutcome(string(encoded))
		if outcome != "succeeded" {
			return string(encoded)
		}
		switch operation {
		case "duplicate", "restore_version":
			detail := parsed
			if nested, ok := parsed["site"].(map[string]interface{}); ok {
				detail = nested
			}
			newTarget := firstLedgerString(detail, "slug", "site_id", "id")
			if newTarget == "" && operation != "duplicate" {
				newTarget = target
			}
			if newTarget == "" {
				return homepageOperationPersistenceError(parsed, op, fmt.Errorf("provider did not return the created site identity"))
			}
			deploymentProjectID := projectID
			if operation == "duplicate" {
				created, err := db.Exec("INSERT INTO homepage_projects(name,project_dir,origin,creation_run_id) VALUES(?,'','agent_created',?)", provider+":"+account+":"+newTarget, op.RunID)
				if err != nil {
					return homepageOperationPersistenceError(parsed, op, err)
				}
				deploymentProjectID, err = created.LastInsertId()
				if err != nil {
					return homepageOperationPersistenceError(parsed, op, err)
				}
			}
			url := firstLedgerString(detail, "site_url", "url", "deployment_url")
			locator := ""
			if url == "" {
				locator = provider + ":" + account + ":" + newTarget
			}
			if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{ProjectID: deploymentProjectID, Provider: provider, ProviderTargetID: newTarget, ProviderDeployID: firstLedgerString(detail, "current_version_id", "version_id"), URL: url, RemotePath: locator, Status: "published_unverified", Metadata: map[string]interface{}{"account": account, "created_by": "agent", "reason": op.Reason}}); err != nil {
				return homepageOperationPersistenceError(parsed, op, err)
			}
			if operation == "duplicate" {
				if _, err := recordHomepageOperationEvent(db, deploymentProjectID, homepageCurrentOperation(c), "project_created", "succeeded", map[string]interface{}{"source_managed_site_id": projectID}); err != nil {
					return homepageOperationPersistenceError(parsed, op, err)
				}
				parsed["created_managed_site_id"] = deploymentProjectID
				encoded, _ = json.Marshal(parsed)
			}
		case "delete_site":
			if _, err := db.Exec("UPDATE homepage_deploy_targets SET status='inactive',updated_at=CURRENT_TIMESTAMP WHERE project_id=? AND provider=? AND provider_target_id=?", projectID, provider, target); err != nil {
				return homepageOperationPersistenceError(parsed, op, err)
			}
		}
		return string(encoded)
	})
}
