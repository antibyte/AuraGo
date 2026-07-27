package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func checkGeneratedArtifacts(root string, opts options) error {
	tmp, err := os.MkdirTemp("", "aurago-training-check-*")
	if err != nil {
		return fmt.Errorf("create check directory: %w", err)
	}
	defer os.RemoveAll(tmp)

	result, err := buildTrainingPack(root, opts.sourceDir, false)
	if err != nil {
		return err
	}
	if err := writeGeneratedArtifacts(tmp, result); err != nil {
		return err
	}
	var expectedManifest artifactManifest
	expectedManifestData, err := os.ReadFile(filepath.Join(tmp, "dataset_manifest.json"))
	if err != nil {
		return fmt.Errorf("read regenerated manifest: %w", err)
	}
	if err := json.Unmarshal(expectedManifestData, &expectedManifest); err != nil {
		return fmt.Errorf("decode regenerated manifest: %w", err)
	}
	var drift []string
	for _, name := range generatedArtifactNamesWithoutManifest() {
		actual, actualErr := sha256File(filepath.Join(opts.outDir, name))
		if actualErr != nil || actual != expectedManifest.SHA256[name] {
			drift = append(drift, name)
		}
	}
	actualManifest, actualErr := os.ReadFile(filepath.Join(opts.outDir, "dataset_manifest.json"))
	if actualErr != nil || !bytes.Equal(actualManifest, expectedManifestData) {
		drift = append(drift, "dataset_manifest.json")
	}
	if len(drift) > 0 {
		return fmt.Errorf("generated training artifacts are stale or missing: %v; run go run ./disposable/export_tools --out %s", drift, opts.outDir)
	}
	fmt.Printf("Training artifacts are current: %d files checked\n", len(generatedArtifactNames()))
	return nil
}
