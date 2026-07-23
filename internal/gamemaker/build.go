package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

func (s *Service) BuildJob(ctx context.Context, jobID string) BuildResult {
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	result := buildDirectory(ctx, stage, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes)
	job, _ := s.GetJob(context.Background(), jobID)
	for _, diagnostic := range result.Diagnostics {
		_, _ = s.emit(context.Background(), job.ProjectID, jobID, "diagnostic", map[string]any{
			"level": diagnostic.Level, "message": diagnostic.Message,
			"file": diagnostic.File, "line": diagnostic.Line, "column": diagnostic.Column,
		})
	}
	if result.OK {
		s.mu.Lock()
		s.previewJobs[job.ProjectID] = jobID
		s.mu.Unlock()
		_, _ = s.emit(context.Background(), job.ProjectID, jobID, "preview_reload", map[string]any{"staging": true})
	}
	return result
}

func buildDirectory(ctx context.Context, projectDir string, maxFiles int, maxBytes int64) BuildResult {
	if err := ctx.Err(); err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	for _, required := range []string{"game.json", "index.html", filepath.Join("src", "main.ts")} {
		if _, err := os.Stat(filepath.Join(projectDir, required)); err != nil {
			return BuildResult{Diagnostics: []Diagnostic{{Level: "error", File: filepath.ToSlash(required), Message: "Required game file is missing"}}}
		}
	}
	manifestData, err := os.ReadFile(filepath.Join(projectDir, "game.json"))
	if err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", File: "game.json", Message: err.Error()}}}
	}
	var manifest gameManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", File: "game.json", Message: "Invalid game manifest: " + err.Error()}}}
	}
	if manifest.Dimension != "2d" && manifest.Dimension != "3d" {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", File: "game.json", Message: "Manifest dimension must be 2d or 3d"}}}
	}
	if err := installRuntime(projectDir, manifest.Dimension); err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "dist"), 0o750); err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	build := api.Build(api.BuildOptions{
		AbsWorkingDir: projectDir,
		EntryPoints:   []string{filepath.Join("src", "main.ts")},
		Bundle:        true,
		Outfile:       filepath.Join("dist", "game.js"),
		Format:        api.FormatESModule,
		Platform:      api.PlatformBrowser,
		Target:        api.ES2020,
		External:      []string{"../vendor/*"},
		Write:         true,
		LogLevel:      api.LogLevelSilent,
		LegalComments: api.LegalCommentsLinked,
	})
	if len(build.Errors) > 0 {
		diagnostics := make([]Diagnostic, 0, len(build.Errors))
		for _, message := range build.Errors {
			diagnostic := Diagnostic{Level: "error", Message: message.Text}
			if message.Location != nil {
				diagnostic.File = filepath.ToSlash(message.Location.File)
				diagnostic.Line = message.Location.Line
				diagnostic.Column = message.Location.Column
			}
			diagnostics = append(diagnostics, diagnostic)
		}
		return BuildResult{Diagnostics: diagnostics}
	}
	source, err := os.ReadFile(filepath.Join(projectDir, "src", "main.ts"))
	if err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	if !strings.Contains(string(source), "__AURAGO_GAME_DIAGNOSTICS__") && !strings.Contains(string(source), "diagnostic(") {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", File: "src/main.ts", Message: "Required AuraGo diagnostic interface is missing"}}}
	}
	if err := validateTreeLimits(projectDir, maxFiles, maxBytes); err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	return BuildResult{OK: true}
}

func validateTreeLimits(root string, maxFiles int, maxBytes int64) error {
	var files int
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: project symlink", ErrInvalidPath)
		}
		if entry.IsDir() {
			return nil
		}
		files++
		total += info.Size()
		if files > maxFiles {
			return fmt.Errorf("game maker project exceeds %d files", maxFiles)
		}
		if total > maxBytes {
			return fmt.Errorf("game maker project exceeds %d bytes", maxBytes)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("validate game maker project: %w", err)
	}
	return nil
}
