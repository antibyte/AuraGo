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
	return s.buildJob(ctx, jobID, "")
}

func (s *Service) buildJob(ctx context.Context, jobID, scope string) BuildResult {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	job, _ := s.GetJob(context.Background(), jobID)
	buildID := randomID("build")
	// Even a failed replacement build invalidates earlier diagnostics.
	s.mu.Lock()
	s.previewCheck = nil
	s.mu.Unlock()
	_, _ = s.emit(context.Background(), job.ProjectID, jobID, "build_started", map[string]any{"build_id": buildID})
	result := buildDirectory(ctx, stage, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes)
	result.BuildID = buildID
	for i := range result.Diagnostics {
		// Compiler and filesystem messages may quote the resolved staging path.
		result.Diagnostics[i].Message = s.RedactHostPaths(result.Diagnostics[i].Message)
	}
	for _, diagnostic := range result.Diagnostics {
		_, _ = s.emit(context.Background(), job.ProjectID, jobID, "diagnostic", map[string]any{
			"level": diagnostic.Level, "message": diagnostic.Message, "build_id": buildID,
			"file": diagnostic.File, "line": diagnostic.Line, "column": diagnostic.Column,
		})
	}
	if result.OK {
		result.fingerprint, err = validationFingerprint(stage)
		if err != nil {
			return BuildResult{BuildID: buildID, Diagnostics: []Diagnostic{{Level: "error", Message: s.RedactHostPaths(err.Error())}}}
		}
		s.mu.Lock()
		s.previewJobs[job.ProjectID] = jobID
		s.previewCheck = &previewCheck{ID: buildID, JobID: jobID, SourceMap: result.sourceMap}
		result.check = s.previewCheck
		result.BuildID = s.previewCheck.ID

		s.mu.Unlock()
		result.RuntimeStatus = "unverified"
		_, _ = s.emit(context.Background(), job.ProjectID, jobID, "preview_reload", map[string]any{"staging": true, "build_id": result.BuildID})
	}
	return result
}

func buildDirectory(ctx context.Context, projectDir string, maxFiles int, maxBytes int64) BuildResult {
	if err := ctx.Err(); err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	for _, required := range []string{"game.json", "index.html"} {
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
	if manifest.Variant != "" && (manifest.Variant != "voxel" || manifest.Dimension != "3d") {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", File: "game.json", Message: "Manifest variant must be empty or voxel in 3d"}}}
	}
	mainPath := filepath.Join(projectDir, "src", "main.ts")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		if err := validateTreeLimits(projectDir, maxFiles, maxBytes); err != nil {
			return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
		}
		return BuildResult{OK: true}
	} else if err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "dist"), 0o750); err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	build := api.Build(api.BuildOptions{
		AbsWorkingDir:  projectDir,
		EntryPoints:    []string{filepath.Join("src", "main.ts")},
		Bundle:         true,
		Outfile:        filepath.Join("dist", "game.js"),
		Format:         api.FormatESModule,
		Platform:       api.PlatformBrowser,
		Target:         api.ES2020,
		TsconfigRaw:    "{}",
		External:       []string{"../vendor/*"},
		Write:          false,
		Sourcemap:      api.SourceMapExternal,
		SourcesContent: api.SourcesContentInclude,
		Metafile:       true,
		LogLevel:       api.LogLevelSilent,
		LegalComments:  api.LegalCommentsLinked,
		Plugins:        []api.Plugin{projectImportBounds(projectDir)},
	})
	if len(build.Errors) > 0 {
		diagnostics := make([]Diagnostic, 0, len(build.Errors))
		for _, message := range build.Errors {
			diagnostic := Diagnostic{Level: "error", Message: message.Text}
			if message.Location != nil {
				diagnostic.File = filepath.ToSlash(message.Location.File)
				diagnostic.Line = message.Location.Line
				diagnostic.Column = message.Location.Column
				if path, _, err := secureJoin(projectDir, diagnostic.File, false); err == nil {
					if data, err := os.ReadFile(path); err == nil {
						diagnostic.SourceSHA256 = sourceHash(string(data))
						diagnostic.Excerpt = sourceExcerpt(string(data), diagnostic.Line)
					}
				}
			}
			diagnostics = append(diagnostics, diagnostic)
		}
		return BuildResult{Diagnostics: diagnostics}
	}
	var sourceMap *buildSourceMap
	for _, output := range build.OutputFiles {
		if strings.HasSuffix(output.Path, ".map") {
			// Mapping is advisory; its absence must not weaken build validation.
			sourceMap, _ = decodeBuildSourceMap(projectDir, output.Contents)
			continue
		}
		rel, err := filepath.Rel(projectDir, output.Path)
		if err != nil {
			return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
		}
		path, _, err := secureJoin(projectDir, rel, true)
		if err == nil {
			err = os.WriteFile(path, output.Contents, 0o640)
		}
		if err != nil {
			return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
		}
	}
	if err := validateTreeLimits(projectDir, maxFiles, maxBytes); err != nil {
		return BuildResult{Diagnostics: []Diagnostic{{Level: "error", Message: err.Error()}}}
	}
	return BuildResult{OK: true, sourceMap: sourceMap}
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
