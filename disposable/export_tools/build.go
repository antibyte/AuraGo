package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func buildTrainingPack(root, sourceDir string, bootstrap bool) (BuildResult, error) {
	tools, err := loadStrictTools(root)
	if err != nil {
		return BuildResult{}, err
	}
	if len(tools) == 0 {
		return BuildResult{}, fmt.Errorf("native schema snapshot did not contain tools")
	}

	var tiers TierManifest
	var contracts OperationContractManifest
	if bootstrap {
		tiers = buildDefaultTiers(tools)
		if err := applyTiers(tools, tiers); err != nil {
			return BuildResult{}, err
		}
		contracts, err = buildDefaultContracts(root, tools)
		if err != nil {
			return BuildResult{}, err
		}
	} else {
		if err := readJSON(filepath.Join(sourceDir, "tool_tiers.json"), &tiers); err != nil {
			return BuildResult{}, fmt.Errorf("load tool tier manifest: %w", err)
		}
		if err := applyTiers(tools, tiers); err != nil {
			return BuildResult{}, err
		}
		if err := readJSON(filepath.Join(sourceDir, "operation_contracts.json"), &contracts); err != nil {
			return BuildResult{}, fmt.Errorf("load operation contracts: %w", err)
		}
	}

	operationCount, err := validateContracts(tools, contracts)
	if err != nil {
		return BuildResult{}, err
	}
	scenarios, challenge, err := generateScenarios(tools, contracts)
	if err != nil {
		return BuildResult{}, err
	}
	tagged, err := buildTaggedScenarios(scenarios)
	if err != nil {
		return BuildResult{}, err
	}
	result := BuildResult{
		Tools:          tools,
		Tiers:          tiers,
		Contracts:      contracts,
		Scenarios:      scenarios,
		Tagged:         tagged,
		Challenge:      challenge,
		OperationCount: operationCount,
	}
	if err := validateBuildResult(result); err != nil {
		return BuildResult{}, err
	}
	return result, nil
}

func readJSON(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		offset := int64(0)
		switch typed := err.(type) {
		case *json.SyntaxError:
			offset = typed.Offset
		case *json.UnmarshalTypeError:
			offset = typed.Offset
		}
		if offset > 0 && offset <= int64(len(data)) {
			line := bytes.Count(data[:offset], []byte{'\n'}) + 1
			return fmt.Errorf("decode %s:%d: %w", path, line, err)
		}
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func writeSourceManifests(outDir string, result BuildResult) error {
	if err := writeJSON(filepath.Join(outDir, "tool_tiers.json"), result.Tiers); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "operation_contracts.json"), result.Contracts); err != nil {
		return err
	}
	return nil
}
