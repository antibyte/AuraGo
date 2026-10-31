package kaggle

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"aurago/internal/config"
)

type grantSnapshotKey struct{}

func WithConfigSnapshot(ctx context.Context, c config.KaggleConfig) context.Context {
	c.AllowedOwners = append([]string(nil), c.AllowedOwners...)
	return context.WithValue(ctx, grantSnapshotKey{}, c)
}
func intersectConfig(a, b config.KaggleConfig) config.KaggleConfig {
	a.Enabled = a.Enabled && b.Enabled
	a.ReadOnly = a.ReadOnly || b.ReadOnly
	a.AllowDownload = a.AllowDownload && b.AllowDownload
	a.AllowCreate = a.AllowCreate && b.AllowCreate
	a.AllowUpdate = a.AllowUpdate && b.AllowUpdate
	a.AllowPublish = a.AllowPublish && b.AllowPublish
	a.AllowDelete = a.AllowDelete && b.AllowDelete
	a.AllowSubmit = a.AllowSubmit && b.AllowSubmit
	a.AllowRemoteExecute = a.AllowRemoteExecute && b.AllowRemoteExecute
	a.AllowGPU = a.AllowGPU && b.AllowGPU
	a.AllowTPU = a.AllowTPU && b.AllowTPU
	a.AllowInternet = a.AllowInternet && b.AllowInternet
	owners := []string{}
	for _, v := range a.AllowedOwners {
		for _, w := range b.AllowedOwners {
			if strings.EqualFold(v, w) {
				owners = append(owners, v)
				break
			}
		}
	}
	a.AllowedOwners = owners
	return a
}

// Access is the caller's existing global gate snapshot; grants never widen it.
type Access struct {
	Network bool
	Write   bool
	Python  bool
	Docker  bool
	Admin   bool
}

func CheckPolicy(c config.KaggleConfig, a Access, op Operation, params map[string]any, account string) error {
	if err := CheckGrants(c, a, op, params); err != nil {
		return err
	}
	// Ownership limits remote writes, not access to public third-party resources.
	if op.Grant == "read" || op.Grant == "download" {
		return nil
	}
	owner := stringValue(params, "ownerSlug")
	if owner == "" {
		owner = stringValue(params, "userName")
	}
	if owner == "" {
		if slug := stringValue(params, "slug"); strings.Contains(slug, "/") {
			owner = strings.Split(slug, "/")[0]
		}
		if slug, ok := params["slug"].(map[string]any); ok {
			owner = stringValue(slug, "ownerSlug")
		}
	}
	if owner != "" && !ownerAllowed(c, account, owner) {
		return errors.New("Kaggle resource owner is not allowed")
	}
	if slugs, ok := params["taskSlugs"].([]any); ok {
		if len(slugs) != 1 {
			return errors.New("schedule one benchmark task per durable job")
		}
		for _, v := range slugs {
			slug, _ := v.(map[string]any)
			if !ownerAllowed(c, account, stringValue(slug, "ownerSlug")) {
				return errors.New("Kaggle benchmark owner is not allowed")
			}
		}
	}
	if op.Name == "batch_schedule_benchmark_task_runs" {
		models, _ := params["modelVersionSlugs"].([]any)
		if len(models) != 1 {
			return errors.New("schedule one benchmark model per durable job")
		}
		if agents, ok := params["agentSlugs"].([]any); ok && len(agents) > 0 {
			return errors.New("benchmark agent execution is outside this integration")
		}
	}
	return nil
}

// CheckGrants checks capabilities before contacting Kaggle or resolving account ownership.
func CheckGrants(c config.KaggleConfig, a Access, op Operation, params map[string]any) error {
	if !c.Enabled {
		return errors.New("Kaggle integration is disabled")
	}
	if !a.Network {
		return errors.New("network requests are disabled")
	}
	if op.Grant == "read" {
		return nil
	}
	if op.Grant == "download" {
		if !c.AllowDownload || !a.Write {
			return errors.New("Kaggle downloads require download and filesystem write grants")
		}
		return nil
	}
	if op.Grant == "cancel" && a.Admin {
		return nil
	}
	if c.ReadOnly {
		return errors.New("Kaggle is read-only")
	}
	allowed := map[string]bool{"create": c.AllowCreate, "update": c.AllowUpdate, "delete": c.AllowDelete, "publish": c.AllowPublish, "submit": c.AllowSubmit, "execute": c.AllowRemoteExecute, "cancel": c.AllowRemoteExecute}
	if op.Name == "save_kernel" {
		allowed["update"] = c.AllowCreate || c.AllowUpdate
	}
	if !allowed[op.Grant] {
		return fmt.Errorf("Kaggle %s permission is disabled", op.Grant)
	}
	if !a.Write {
		return errors.New("filesystem writes are disabled")
	}
	if op.Name == "create_dataset_version" {
		if body, ok := params["body"].(map[string]any); ok && boolValue(body, "deleteOldVersions") && !c.AllowDelete {
			return errors.New("deleting old dataset versions requires deletion permission")
		}
	}
	if op.Name == "save_kernel" {
		kind := stringValue(params, "kernelExecutionType")
		if kind != "QUICK_SAVE" && kind != "SAVE_AND_RUN_ALL" {
			return errors.New("notebooks support QUICK_SAVE or SAVE_AND_RUN_ALL only")
		}
		if kind == "SAVE_AND_RUN_ALL" && !c.AllowRemoteExecute {
			return errors.New("Kaggle remote execution is disabled")
		}
		if boolValue(params, "enableGpu") && !c.AllowGPU {
			return errors.New("Kaggle GPU permission is disabled")
		}
		if boolValue(params, "enableTpu") && !c.AllowTPU {
			return errors.New("Kaggle TPU permission is disabled")
		}
		if boolValue(params, "enableInternet") && !c.AllowInternet {
			return errors.New("Kaggle internet permission is disabled")
		}
		if stringValue(params, "machineShape") != "" || stringValue(params, "dockerImage") != "" {
			return errors.New("custom Kaggle compute images and machine shapes are not supported")
		}
		if params["id"] != nil {
			return errors.New("use an owner/slug reference instead of a notebook ID")
		}
	}
	if explicitlyPublic(params) && !c.AllowPublish {
		return errors.New("Kaggle publication permission is disabled")
	}
	return nil
}

func ownerAllowed(c config.KaggleConfig, account, owner string) bool {
	if account != "" && strings.EqualFold(account, owner) {
		return true
	}
	for _, v := range c.AllowedOwners {
		if strings.EqualFold(v, owner) {
			return true
		}
	}
	return false
}
func stringValue(m map[string]any, key string) string { v, _ := m[key].(string); return v }
func boolValue(m map[string]any, key string) bool     { v, _ := m[key].(bool); return v }
func explicitlyPublic(v any) bool {
	switch m := v.(type) {
	case map[string]any:
		for k, x := range m {
			if k == "isPrivate" && x == false || k == "isPublic" && x == true || k == "publishTime" && x != nil && x != "" {
				return true
			}
			if explicitlyPublic(x) {
				return true
			}
		}
	case []any:
		for _, x := range m {
			if explicitlyPublic(x) {
				return true
			}
		}
	}
	return false
}

func resourcePrivate(v any) (bool, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return false, false
	}
	if b, ok := m["isPrivate"].(bool); ok {
		return b, true
	}
	if b, ok := m["isPublic"].(bool); ok {
		return !b, true
	}
	for _, key := range []string{"dataset", "model", "kernel", "task", "metadata"} {
		if b, ok := resourcePrivate(m[key]); ok {
			return b, true
		}
	}
	return false, false
}
