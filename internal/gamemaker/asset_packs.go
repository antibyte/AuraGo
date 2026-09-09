package gamemaker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/webassets"

	"github.com/evanw/esbuild/pkg/api"
)

// Only runtime sheets and catalog are included in the external resource set.
var assetPackFS = webassets.Namespace("gamemaker")

type AssetPackSummary struct {
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type AssetPack struct {
	AssetPackSummary
	SchemaVersion int             `json:"schema_version"`
	Image         string          `json:"image"`
	Columns       int             `json:"columns"`
	Rows          int             `json:"rows"`
	FrameWidth    int             `json:"frame_width"`
	FrameHeight   int             `json:"frame_height"`
	Frames        json.RawMessage `json:"frames"`
	Assets        json.RawMessage `json:"assets"`
	Animations    json.RawMessage `json:"animations"`
	Assemblies    json.RawMessage `json:"assemblies,omitempty"`
	Provenance    json.RawMessage `json:"provenance"`
}

type ImportedAssetPack struct {
	ID            string `json:"id"`
	Version       string `json:"version"`
	Image         string `json:"image"`
	Metadata      string `json:"metadata"`
	PhaserExample string `json:"phaser_example"`
}

// Parse imports without loading dependencies or writing output. A hallucinated
// pack must not replace the current source or consume a gameplay repair pass.
func (s *Service) validateScriptAssetImports(ctx context.Context, jobID, rel, content string) error {
	loader, script := map[string]api.Loader{
		".ts": api.LoaderTS, ".mts": api.LoaderTS, ".cts": api.LoaderTS, ".tsx": api.LoaderTSX,
		".js": api.LoaderJS, ".mjs": api.LoaderJS, ".cjs": api.LoaderJS, ".jsx": api.LoaderJSX,
	}[strings.ToLower(filepath.Ext(rel))]
	if !script {
		return nil
	}
	packs, err := s.importedJobPacks(ctx, jobID)
	if err != nil {
		return err
	}
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return err
	}
	present := map[string]string{}
	var examples []string
	for _, pack := range packs {
		present[pack.Metadata] = pack.Image
		path, _ := filepath.Rel(filepath.Dir(rel), filepath.FromSlash(pack.Metadata))
		if !strings.HasPrefix(path, ".") {
			path = "./" + path
		}
		examples = append(examples, filepath.ToSlash(path))
	}
	hint := "No complete packs are imported. Use search_assets/describe_asset, then import_pack before writing the import."
	if len(examples) > 0 {
		hint = "Available metadata imports from this file: " + strings.Join(examples[:min(8, len(examples))], ", ") + ". Use list_files for all project copies."
	}
	result := api.Build(api.BuildOptions{
		Stdin:  &api.StdinOptions{Contents: content, Sourcefile: rel, Loader: loader},
		Bundle: true, Write: false, LogLevel: api.LogLevelSilent,
		TsconfigRaw: `{"compilerOptions":{"verbatimModuleSyntax":true}}`,
		Plugins: []api.Plugin{{Name: "game-asset-imports", Setup: func(build api.PluginBuild) {
			build.OnResolve(api.OnResolveOptions{Filter: ".*"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
				out := api.OnResolveResult{External: true}
				path := filepath.ToSlash(filepath.Join(filepath.Dir(rel), filepath.FromSlash(args.Path)))
				if !strings.Contains(strings.ReplaceAll(args.Path, "\\", "/"), "assets/builtin/") && !strings.HasPrefix(path, "assets/builtin/") {
					return out, nil
				}
				image, exists := present[path]
				_, _, metaErr := secureJoin(stage, path, false)
				_, _, imageErr := secureJoin(stage, image, false)
				if !(strings.HasPrefix(args.Path, "./") || strings.HasPrefix(args.Path, "../")) || !exists || metaErr != nil || imageErr != nil {
					out.Errors = []api.Message{{Text: fmt.Sprintf("asset_import_invalid: %q does not reference an imported PNG/JSON pair from %s. File unchanged. %s Preserve the accepted plan and existing pack IDs; do not guess another pack or add ../ segments.", args.Path, rel, hint)}}
				}
				return out, nil
			})
		}}},
	})
	for _, diagnostic := range result.Errors {
		if strings.HasPrefix(diagnostic.Text, "asset_import_invalid:") {
			return fmt.Errorf("%s", diagnostic.Text)
		}
	}
	// Other syntax/build errors retain the existing validation workflow.
	return nil
}

// Report complete project copies, including older immutable pack versions.
func (s *Service) importedJobPacks(ctx context.Context, jobID string) ([]ImportedAssetPack, error) {
	files, err := s.ListJobFiles(ctx, jobID)
	if err != nil {
		return nil, err
	}
	packs, err := s.ListAssetPacks()
	if err != nil {
		return nil, err
	}
	known, present := map[string]bool{}, map[string]bool{}
	for _, pack := range packs {
		known[pack.ID] = true
	}
	for _, file := range files {
		present[file] = true
	}
	var out []ImportedAssetPack
	for _, file := range files {
		parts := strings.Split(file, "/")
		if len(parts) != 5 || parts[0] != "assets" || parts[1] != "builtin" || !known[parts[2]] || parts[4] != "sheet.json" {
			continue
		}
		image := strings.TrimSuffix(file, "sheet.json") + "sheet.png"
		if present[image] {
			out = append(out, ImportedAssetPack{ID: parts[2], Version: parts[3], Image: image, Metadata: file})
		}
	}
	return out, nil
}

func (s *Service) ListAssetPacks() ([]AssetPackSummary, error) {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	if !s.policy.Enabled {
		return nil, ErrDisabled
	}
	data, err := assetPackFS.ReadFile("asset_packs/catalog.json")
	if err != nil {
		return nil, fmt.Errorf("list bundled sprite packs: %w", err)
	}
	var packs []AssetPackSummary
	if err := json.Unmarshal(data, &packs); err != nil {
		return nil, fmt.Errorf("decode sprite catalog: %w", err)
	}
	return packs, nil
}

func (s *Service) AssetPackFile(id, filename string) ([]byte, error) {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	if !s.policy.Enabled {
		return nil, ErrDisabled
	}
	return bundledAssetPackFile(id, filename)
}

func bundledAssetPackFile(id, filename string) ([]byte, error) {
	if id == "" || strings.ContainsAny(id, "/\\.\x00") || (filename != "sheet.png" && filename != "sheet.json") {
		return nil, ErrNotFound
	}
	data, err := assetPackFS.ReadFile("asset_packs/" + id + "/" + filename)
	if err != nil {
		return nil, ErrNotFound
	}
	return data, nil
}

func (s *Service) DescribeAssetPack(id string) (AssetPack, error) {
	data, err := s.AssetPackFile(id, "sheet.json")
	if err != nil {
		return AssetPack{}, err
	}
	var pack AssetPack
	if err := json.Unmarshal(data, &pack); err != nil {
		return AssetPack{}, fmt.Errorf("decode bundled sprite metadata: %w", err)
	}
	return pack, nil
}

func validateAssetPackIDs(ids []string) ([]string, error) {
	available, err := fs.Glob(assetPackFS, "asset_packs/*/sheet.json")
	if err != nil {
		return nil, fmt.Errorf("list bundled sprite packs: %w", err)
	}
	if len(ids) > len(available) {
		return nil, fmt.Errorf("at most %d sprite packs may be selected", len(available))
	}
	var result []string
	seen := map[string]bool{}
	for _, id := range ids {
		if _, err := bundledAssetPackFile(id, "sheet.json"); err != nil {
			return nil, fmt.Errorf("unknown sprite pack %q: %w", id, err)
		}
		if !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	return result, nil
}

// ImportAssetPack publishes the PNG/JSON pair by renaming one temporary directory.
// Existing project copies are immutable: matching copies are reused, not replaced.
func (s *Service) ImportAssetPack(ctx context.Context, jobID, id string) (ImportedAssetPack, error) {
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return ImportedAssetPack{}, err
	}
	s.policyMu.RLock()
	policy := s.policy
	if !policy.Enabled || policy.ReadOnly || !policy.AllowEdit {
		s.policyMu.RUnlock()
		if !policy.Enabled {
			return ImportedAssetPack{}, ErrDisabled
		}
		return ImportedAssetPack{}, ErrReadOnly
	}
	result, err := s.importAssetPack(ctx, jobID, id)
	s.policyMu.RUnlock()
	if err == nil {
		_ = s.BuildJob(ctx, jobID)
	}
	return result, err
}

func (s *Service) importAssetPack(ctx context.Context, jobID, id string) (ImportedAssetPack, error) {
	var result ImportedAssetPack
	metadata, err := bundledAssetPackFile(id, "sheet.json")
	if err != nil {
		return result, err
	}
	png, err := bundledAssetPackFile(id, "sheet.png")
	if err != nil {
		return result, err
	}
	var pack AssetPack
	if err := json.Unmarshal(metadata, &pack); err != nil {
		return result, fmt.Errorf("decode sprite pack: %w", err)
	}
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return result, err
	}
	// Serialize against builds and other imports so limit checks see one snapshot.
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	rel := "assets/builtin/" + id + "/" + pack.Version
	target, _, err := secureJoin(stage, rel, false)
	if err != nil {
		return result, err
	}
	result = ImportedAssetPack{ID: id, Version: pack.Version, Image: rel + "/sheet.png", Metadata: rel + "/sheet.json"}
	_, assets, assemblies, _, err := readPackUsage(id)
	if err != nil || len(assets) == 0 {
		return ImportedAssetPack{}, fmt.Errorf("sprite pack has no usable asset metadata")
	}
	assetID, assemblyID := assets[0].ID, ""
	if len(assemblies) > 0 {
		assetID, assemblyID = "", assemblies[0].ID
	} else {
		for _, asset := range assets {
			if !asset.AssemblyPart {
				assetID = asset.ID
				break
			}
		}
	}
	detail, err := s.describeAsset(id, assetID, assemblyID)
	if err != nil {
		return ImportedAssetPack{}, err
	}
	result.PhaserExample = detail.Example
	if info, statErr := os.Lstat(target); statErr == nil {
		if !info.IsDir() {
			return ImportedAssetPack{}, fmt.Errorf("sprite pack destination already exists")
		}
		entries, err := os.ReadDir(target)
		if err != nil || len(entries) != 2 {
			return ImportedAssetPack{}, fmt.Errorf("sprite pack copy is incomplete or modified")
		}
		for filename, expected := range map[string][]byte{"sheet.png": png, "sheet.json": metadata} {
			path, _, err := secureJoin(stage, rel+"/"+filename, false)
			if err != nil {
				return ImportedAssetPack{}, err
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, expected) {
				return ImportedAssetPack{}, fmt.Errorf("sprite pack copy is incomplete or modified")
			}
		}
		return result, nil
	} else if !os.IsNotExist(statErr) {
		return ImportedAssetPack{}, fmt.Errorf("inspect sprite pack destination: %w", statErr)
	}
	if int64(len(png)) > s.opts.MaxAssetBytes || int64(len(metadata)) > s.opts.MaxFileBytes {
		return ImportedAssetPack{}, fmt.Errorf("sprite pack exceeds configured file or asset limit")
	}
	if err := validateTreeLimits(stage, s.opts.MaxFilesPerProject-2, s.opts.MaxProjectBytes-int64(len(png)+len(metadata))); err != nil {
		return ImportedAssetPack{}, fmt.Errorf("sprite pack exceeds configured project limit: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return ImportedAssetPack{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return ImportedAssetPack{}, fmt.Errorf("create sprite pack parent: %w", err)
	}
	tmp, err := os.MkdirTemp(filepath.Dir(target), ".gm-pack-*")
	if err != nil {
		return ImportedAssetPack{}, fmt.Errorf("stage sprite pack: %w", err)
	}
	defer os.RemoveAll(tmp)
	for filename, data := range map[string][]byte{"sheet.png": png, "sheet.json": metadata} {
		if err := os.WriteFile(filepath.Join(tmp, filename), data, 0o640); err != nil {
			return ImportedAssetPack{}, fmt.Errorf("write staged sprite pack: %w", err)
		}
	}
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return ImportedAssetPack{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportedAssetPack{}, fmt.Errorf("record sprite pack: %w", err)
	}
	defer tx.Rollback()
	for filename, data := range map[string][]byte{"sheet.png": png, "sheet.json": metadata} {
		_, err = tx.ExecContext(ctx, `INSERT INTO gm_assets(project_id,job_id,path,kind,generator,provenance,content_hash,created_at) VALUES(?,?,?,?,?,?,?,?)`,
			job.ProjectID, jobID, rel+"/"+filename, "sprite_pack", "builtin", id+"@"+pack.Version, sha256Bytes(data), time.Now().UTC())
		if err != nil {
			return ImportedAssetPack{}, fmt.Errorf("record sprite pack file: %w", err)
		}
	}
	if err := os.Rename(tmp, target); err != nil {
		return ImportedAssetPack{}, fmt.Errorf("publish sprite pack: %w", err)
	}
	if err := tx.Commit(); err != nil {
		if rollbackErr := os.Rename(target, tmp); rollbackErr != nil {
			return ImportedAssetPack{}, fmt.Errorf("record sprite pack: %w; undo import: %v", err, rollbackErr)
		}
		return ImportedAssetPack{}, fmt.Errorf("record sprite pack: %w", err)
	}
	_, _ = s.emit(ctx, job.ProjectID, jobID, "asset_changed", map[string]any{"path": rel, "kind": "sprite_pack", "generator": "builtin"})
	return result, nil
}
