package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// UseTemplate replaces entry files only on an explicit agent request.
func (s *Service) UseTemplate(ctx context.Context, jobID, name string) error {
	if !slices.Contains(templateNames(), name) {
		return fmt.Errorf("unknown template %q", name)
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return err
	}
	project, _, err := s.ProjectForJob(ctx, jobID)
	if err != nil {
		return err
	}
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return err
	}
	candidate, err := os.MkdirTemp(s.stagingDir, ".template-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(candidate)
	if err := copyTree(stage, candidate, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes); err != nil {
		return err
	}
	dimension, variant, engine, version := "2d", "", "phaser", PhaserVersion
	if name == "three" || name == "voxel" || guided3D(name) {
		dimension, engine, version = "3d", "three", ThreeVersion
	}
	if name == "voxel" {
		variant = "voxel"
	}
	project.Dimension, project.Variant = dimension, variant
	plan := ExampleGamePlan(project)
	plan.Template, plan.Assets = name, nil
	if err := installRuntime(candidate, dimension, variant); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(candidate, "src"), 0o750); err != nil {
		return err
	}
	if name == "three" {
		if err := os.WriteFile(filepath.Join(candidate, "src", "main.ts"), []byte(threeScaffold), 0o640); err != nil {
			return err
		}
	} else if err := installGameTemplate(candidate, plan); err != nil {
		return err
	}
	manifest, _ := json.MarshalIndent(gameManifest{Name: project.Name, Dimension: dimension, Variant: variant, Engine: engine, Version: version, Entry: "index.html", Description: project.Description}, "", "  ")
	for path, data := range map[string][]byte{"game.json": manifest, "index.html": []byte(scaffoldHTML(dimension, project.Name))} {
		if err := os.WriteFile(filepath.Join(candidate, path), data, 0o640); err != nil {
			return err
		}
	}
	if err := validateTreeLimits(candidate, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	backup := candidate + "-previous"
	if err := os.Rename(stage, backup); err != nil {
		return err
	}
	if err := os.Rename(candidate, stage); err != nil {
		if rollback := os.Rename(backup, stage); rollback != nil {
			return fmt.Errorf("template commit failed: %w; restore failed: %v", err, rollback)
		}
		return err
	}
	_ = os.RemoveAll(backup)
	_, _ = s.emit(ctx, project.ID, jobID, "file_changed", map[string]any{"path": "index.html", "template": name})
	return nil
}
