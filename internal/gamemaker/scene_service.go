package gamemaker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"sort"
	"strings"
)

// MarshalSceneJSON returns the canonical, stable representation persisted in
// src/scene.json. Arrays are sorted by stable ID; map keys are sorted by the
// standard JSON encoder.
func MarshalSceneJSON(scene Scene) ([]byte, error) {
	canonical, err := cloneScene(scene)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(canonical.Levels, func(i, j int) bool { return canonical.Levels[i].ID < canonical.Levels[j].ID })
	sort.SliceStable(canonical.Nodes, func(i, j int) bool { return canonical.Nodes[i].ID < canonical.Nodes[j].ID })
	sort.SliceStable(canonical.Regions, func(i, j int) bool { return canonical.Regions[i].ID < canonical.Regions[j].ID })
	sort.SliceStable(canonical.Placements, func(i, j int) bool { return canonical.Placements[i].ID < canonical.Placements[j].ID })
	sort.SliceStable(canonical.Colliders, func(i, j int) bool { return canonical.Colliders[i].ID < canonical.Colliders[j].ID })
	sort.SliceStable(canonical.Attachments, func(i, j int) bool { return canonical.Attachments[i].ID < canonical.Attachments[j].ID })
	sort.SliceStable(canonical.Zones, func(i, j int) bool { return canonical.Zones[i].ID < canonical.Zones[j].ID })
	sort.SliceStable(canonical.Routes, func(i, j int) bool { return canonical.Routes[i].ID < canonical.Routes[j].ID })
	return json.MarshalIndent(canonical, "", "  ")
}

// DecodeSceneJSON rejects unknown fields and trailing JSON so direct source
// writes and agent operations share exactly one parser.
func DecodeSceneJSON(data []byte) (Scene, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Scene{}, fmt.Errorf("scene JSON is empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var scene Scene
	if err := decoder.Decode(&scene); err != nil {
		return Scene{}, fmt.Errorf("decode scene JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Scene{}, fmt.Errorf("decode scene JSON: multiple values")
		}
		return Scene{}, fmt.Errorf("decode scene JSON: %w", err)
	}
	return scene, nil
}

func sceneHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func dryRunValue(values []bool) bool { return len(values) > 0 && values[0] }

func (s *Service) readSceneLocked(ctx context.Context, jobID string) (Scene, []byte, bool, []SceneDiagnostic, error) {
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return Scene{}, nil, false, nil, err
	}
	path, _, err := secureJoin(stage, SceneFilePath, true)
	if err != nil {
		return Scene{}, nil, false, nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Scene{}, nil, false, nil, nil
	}
	if err != nil {
		return Scene{}, nil, false, nil, fmt.Errorf("read scene: %w", err)
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return Scene{}, nil, false, nil, nil
	}
	if int64(len(data)) > s.opts.MaxFileBytes {
		return Scene{}, data, true, []SceneDiagnostic{sceneError(SceneFilePath, "exceeds the configured project file limit")}, fmt.Errorf("scene exceeds configured file limit")
	}
	scene, err := DecodeSceneJSON(data)
	if err != nil {
		return Scene{}, data, true, []SceneDiagnostic{sceneError(SceneFilePath, err.Error())}, err
	}
	diagnostics := ValidateScene(scene, AssetCatalog{})
	if err := validateScene(scene, AssetCatalog{}); err != nil {
		return scene, data, true, diagnostics, err
	}
	return scene, data, true, diagnostics, nil
}

func (s *Service) resultForScene(scene Scene, data []byte, exists, written, dryRun bool, diagnostics []SceneDiagnostic) SceneResult {
	return s.sceneResult(scene, data, nil, exists, written, dryRun, diagnostics)
}

func (s *Service) sceneResult(scene Scene, currentData, proposedData []byte, exists, written, dryRun bool, diagnostics []SceneDiagnostic) SceneResult {
	result := SceneResult{Scene: scene, Path: SceneFilePath, Exists: exists, Written: written, DryRun: dryRun, Diagnostics: diagnostics}
	if len(currentData) > 0 {
		result.CurrentSHA256 = sceneHash(currentData)
	}
	if len(proposedData) > 0 {
		previous, _ := DecodeSceneJSON(currentData)
		result.Changes = sceneChanges(previous, scene)
		result.ProposedSHA256 = sceneHash(proposedData)
	}
	if written {
		result.SHA256 = result.ProposedSHA256
	} else {
		result.SHA256 = result.CurrentSHA256
	}
	return result
}

func checkExpectedSceneHash(expected string, current []byte, exists bool) error {
	expected = strings.TrimSpace(expected)
	if !exists {
		if expected != "" {
			return fmt.Errorf("%w: scene does not exist", ErrSceneConflict)
		}
		return nil
	}
	if expected == "" {
		return fmt.Errorf("%w: existing scene requires its current sha256", ErrSceneConflict)
	}
	actual := sceneHash(current)
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("%w: expected %s, current %s", ErrSceneConflict, expected, actual)
	}
	return nil
}

func (s *Service) writeSceneLocked(ctx context.Context, jobID string, scene Scene, expected string, dryRun bool) (SceneResult, error) {
	data, err := MarshalSceneJSON(scene)
	if err != nil {
		return SceneResult{}, err
	}
	if int64(len(data)) > s.opts.MaxFileBytes {
		return SceneResult{}, fmt.Errorf("scene exceeds configured file limit")
	}
	current, currentData, exists, diagnostics, err := s.readSceneLocked(ctx, jobID)
	// A complete valid replacement may repair malformed scene bytes, but still
	// requires the exact inspected hash. Filesystem failures remain fatal.
	if err != nil && (!exists || len(currentData) == 0) {
		return s.resultForScene(current, currentData, exists, false, dryRun, diagnostics), err
	}
	if err := checkExpectedSceneHash(expected, currentData, exists); err != nil {
		return s.resultForScene(current, currentData, exists, false, dryRun, diagnostics), err
	}
	if dryRun {
		stage, stageErr := s.JobDirectory(jobID)
		if stageErr != nil {
			return s.resultForScene(current, currentData, exists, false, true, diagnostics), stageErr
		}
		// writeJobFile performs this guard for real writes. Dry runs must call
		// the same accepted-plan/catalog validator before reporting success.
		if stageErr := validateBuilderSource(stage, SceneFilePath, string(data)); stageErr != nil {
			return s.sceneResult(scene, currentData, data, exists, false, true, ValidateScene(scene, AssetCatalog{})), stageErr
		}
		return s.sceneResult(scene, currentData, data, true, false, true, ValidateScene(scene, AssetCatalog{})), nil
	}
	if err := s.writeJobFile(ctx, jobID, SceneFilePath, string(data)); err != nil {
		return s.sceneResult(scene, currentData, data, exists, false, false, ValidateScene(scene, AssetCatalog{})), err
	}
	return s.sceneResult(scene, currentData, data, true, true, false, ValidateScene(scene, AssetCatalog{})), nil
}

// InspectScene reads only the active job's staged canonical scene. A missing
// scene is a valid draft state and returns Exists=false.
func (s *Service) InspectScene(ctx context.Context, jobID string) (SceneResult, error) {
	if _, err := s.JobDirectory(jobID); err != nil {
		return SceneResult{}, err
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	scene, data, exists, diagnostics, err := s.readSceneLocked(ctx, jobID)
	return s.resultForScene(scene, data, exists, false, false, diagnostics), err
}

// SetSceneJSON validates and atomically replaces the canonical scene. An empty
// expected hash is create-only; existing scenes require the inspected hash.
func (s *Service) SetSceneJSON(ctx context.Context, jobID, expectedSHA256 string, data []byte, dryRun ...bool) (SceneResult, error) {
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return SceneResult{}, err
	}
	scene, err := DecodeSceneJSON(data)
	if err != nil {
		return SceneResult{}, err
	}
	if err := validateScene(scene, AssetCatalog{}); err != nil {
		return SceneResult{Scene: scene, Path: SceneFilePath, DryRun: dryRunValue(dryRun), Diagnostics: ValidateScene(scene, AssetCatalog{})}, err
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	return s.writeSceneLocked(ctx, jobID, scene, expectedSHA256, dryRunValue(dryRun))
}

// PatchSceneJSON applies ID-keyed upserts/removals and validates the complete
// resulting scene before any file is replaced.
func (s *Service) PatchSceneJSON(ctx context.Context, jobID, expectedSHA256 string, data []byte, dryRun ...bool) (SceneResult, error) {
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return SceneResult{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var patch ScenePatch
	if err := decoder.Decode(&patch); err != nil {
		return SceneResult{}, fmt.Errorf("decode scene patch: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return SceneResult{}, fmt.Errorf("decode scene patch: multiple values")
		}
		return SceneResult{}, fmt.Errorf("decode scene patch: %w", err)
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	current, currentData, exists, diagnostics, err := s.readSceneLocked(ctx, jobID)
	if err != nil {
		return s.resultForScene(current, currentData, exists, false, dryRunValue(dryRun), diagnostics), err
	}
	if !exists {
		return SceneResult{}, fmt.Errorf("%w: scene does not exist", ErrSceneConflict)
	}
	if err := checkExpectedSceneHash(expectedSHA256, currentData, exists); err != nil {
		return s.resultForScene(current, currentData, exists, false, dryRunValue(dryRun), diagnostics), err
	}
	updated, err := applyScenePatch(current, patch)
	if err != nil {
		return SceneResult{}, err
	}
	if err := validateScene(updated, AssetCatalog{}); err != nil {
		return s.sceneResult(updated, currentData, nil, true, false, dryRunValue(dryRun), ValidateScene(updated, AssetCatalog{})), err
	}
	return s.writeSceneLocked(ctx, jobID, updated, expectedSHA256, dryRunValue(dryRun))
}

// GenerateSceneRegionJSON validates the request, performs deterministic local
// regeneration, and uses the same conditional writer as set/patch.
func (s *Service) GenerateSceneRegionJSON(ctx context.Context, jobID, expectedSHA256 string, req GenerateSceneRegionRequest, dryRun ...bool) (SceneResult, error) {
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return SceneResult{}, err
	}
	// Generation must use the accepted plan's actual dimensions, roles and
	// sockets. The request remains JSON-friendly; the catalog is resolved only
	// inside the service and never trusted from agent input.
	if plan, planErr := s.GetPlan(ctx, jobID); planErr != nil {
		return SceneResult{}, planErr
	} else if plan != nil {
		catalog, catalogErr := sceneCatalogForPlan(*plan)
		if catalogErr != nil {
			return SceneResult{}, fmt.Errorf("resolve scene asset catalog: %w", catalogErr)
		}
		req.Assets = catalog
		if strings.TrimSpace(req.AssetID) == "" && strings.TrimSpace(req.AssetRole) != "" {
			for _, asset := range plan.Assets {
				if strings.EqualFold(strings.TrimSpace(asset.Role), strings.TrimSpace(req.AssetRole)) {
					req.AssetID = asset.AssetID
					if asset.AssemblyID != "" {
						req.AssetID = asset.AssemblyID
					}
					break
				}
			}
		}
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	current, currentData, exists, diagnostics, err := s.readSceneLocked(ctx, jobID)
	if err != nil && (current.SchemaVersion == 0 || len(currentData) == 0) {
		return s.resultForScene(current, currentData, exists, false, req.DryRun || dryRunValue(dryRun), diagnostics), err
	}
	if !exists {
		return SceneResult{}, fmt.Errorf("%w: scene does not exist", ErrSceneConflict)
	}
	if err := checkExpectedSceneHash(expectedSHA256, currentData, exists); err != nil {
		return s.resultForScene(current, currentData, exists, false, req.DryRun || dryRunValue(dryRun), diagnostics), err
	}
	stage, stageErr := s.JobDirectory(jobID)
	if stageErr != nil {
		return SceneResult{}, stageErr
	}
	if err := loadSceneMechanics(stage, &current); err != nil {
		return SceneResult{}, err
	}
	updated, err := GenerateSceneRegion(current, req)
	if err != nil {
		return s.sceneResult(current, currentData, nil, true, false, req.DryRun || dryRunValue(dryRun), diagnostics), err
	}
	return s.writeSceneLocked(ctx, jobID, updated, expectedSHA256, req.DryRun || dryRunValue(dryRun))
}

func (s *Service) SetScene(ctx context.Context, jobID, expectedSHA256 string, scene Scene, dryRun bool) (SceneResult, error) {
	data, err := MarshalSceneJSON(scene)
	if err != nil {
		return SceneResult{}, err
	}
	return s.SetSceneJSON(ctx, jobID, expectedSHA256, data, dryRun)
}
