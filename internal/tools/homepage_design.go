package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"aurago/internal/designs"
	"aurago/internal/fileutil"
)

// homepageDesignFile is the project file that carries the selected design
// system. The agent reads it as the project's own design guidance.
const homepageDesignFile = "DESIGN.md"

// ApplyHomepageDesign writes the selected design template into a homepage
// project as DESIGN.md. It works both locally (via the filesystem) and in
// Docker (through the archive transport), so the file content never reaches a
// shell argument.
func ApplyHomepageDesign(cfg HomepageConfig, projectDir, designID string) error {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return fmt.Errorf("project_dir is required to apply a design template")
	}
	if err := sanitizeProjectDir(projectDir); err != nil {
		return err
	}
	content, ok := designs.Content(designID)
	if !ok {
		return fmt.Errorf("unknown design template %q", designID)
	}
	relPath := path.Join(filepath.ToSlash(projectDir), homepageDesignFile)

	if !checkDockerAvailable(cfg.DockerHost, cfg.Context) {
		if cfg.WorkspacePath == "" {
			return fmt.Errorf("homepage workspace path is not configured")
		}
		fullPath := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(projectDir), homepageDesignFile)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("create project directory: %w", err)
		}
		if err := fileutil.WriteFileContext(context.Background(), fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
		return nil
	}

	dockerCfg := DockerConfig{Context: cfg.Context, Host: cfg.DockerHost}
	if err := homepageWriteDockerFile(dockerCfg, relPath, content); err != nil {
		return fmt.Errorf("write %s: %w", relPath, err)
	}
	return nil
}

// SetHomepageProjectDesign stores the selected design template for a project
// directory in the homepage registry.
func SetHomepageProjectDesign(db *sql.DB, projectDir, designID string) error {
	if db == nil {
		return fmt.Errorf("homepage registry DB not initialized")
	}
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return fmt.Errorf("project_dir is required")
	}
	canonical, err := NormalizeHomepageProjectIdentity(projectDir, false)
	if err != nil {
		return err
	}
	designID = strings.TrimSpace(designID)
	if designID != "" {
		if _, ok := designs.Lookup(designID); !ok {
			return fmt.Errorf("unknown design template %q", designID)
		}
	}
	project, err := GetProjectByDir(db, canonical)
	if err != nil {
		return err
	}
	return UpdateProject(db, project.ID, map[string]interface{}{"design_id": designID})
}

// GetHomepageProjectDesign returns the design template bound to a project
// directory. An empty id means the project has no design template yet.
func GetHomepageProjectDesign(db *sql.DB, projectDir string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("homepage registry DB not initialized")
	}
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return "", nil
	}
	canonical, err := NormalizeHomepageProjectIdentity(projectDir, false)
	if err != nil {
		return "", err
	}
	project, err := GetProjectByDir(db, canonical)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(project.DesignID), nil
}

// HomepageInitProjectWithDesign scaffolds a homepage project and applies a
// design template from the homepage design catalog to it.
//
// The design is optional: an empty design id behaves exactly like
// HomepageInitProject. HomepageInitProject keeps its original signature and
// file, so its existing callers and tests are unaffected; this wrapper only
// adds the design step on top of a successful scaffold.
//
// A failing design write does not undo the scaffold. The result then carries
// design_applied=false plus a design_error message so the caller can see that
// the project exists but still misses its design system.
func HomepageInitProjectWithDesign(cfg HomepageConfig, framework, name, template, design string, logger *slog.Logger) string {
	result := HomepageInitProject(cfg, framework, name, template, logger)
	design = strings.TrimSpace(design)
	if design == "" || !homepageInitProjectSucceeded(result) {
		return result
	}
	projectDir := strings.TrimSpace(name)
	if projectDir == "" {
		projectDir = "my-site"
	}
	designErr := ApplyHomepageDesign(cfg, projectDir, design)
	if logger != nil {
		if designErr != nil {
			logger.Warn("[Homepage] Design template could not be applied", "design", design, "project", projectDir, "error", designErr)
		} else {
			logger.Info("[Homepage] Design template applied", "design", design, "project", projectDir)
		}
	}
	return homepageResultWithDesign(result, design, designErr == nil, designErr)
}

// homepageInitProjectSucceeded reports whether an init_project result means
// the project directory exists.
func homepageInitProjectSucceeded(raw string) bool {
	var parsed struct {
		Status   string `json:"status"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return false
	}
	return parsed.Status == "ok" && parsed.ExitCode == 0
}

// homepageResultWithDesign adds the design bookkeeping fields to a tool
// result. Unparsable results are returned unchanged.
func homepageResultWithDesign(raw, designID string, applied bool, designErr error) string {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || payload == nil {
		return raw
	}
	payload["design_id"] = designID
	payload["design_applied"] = applied
	if designErr != nil {
		payload["design_error"] = designErr.Error()
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return string(out)
}
