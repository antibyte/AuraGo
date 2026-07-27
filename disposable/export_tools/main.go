// Command export_tools builds the versioned AuraGo tool-calling training pack.
//
// The checked-in tier and operation-contract manifests are the curated source
// for the generated datasets. Use --bootstrap-contracts only when the native
// schema catalog changes, review the resulting manifests, then regenerate.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type options struct {
	outDir             string
	sourceDir          string
	check              bool
	bootstrapContracts bool
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.outDir, "out", "training", "output directory")
	flag.StringVar(&opts.sourceDir, "source", "", "directory containing curated tier and operation manifests (default: --out)")
	flag.BoolVar(&opts.check, "check", false, "regenerate in a temporary directory and fail when checked-in artifacts differ")
	flag.BoolVar(&opts.bootstrapContracts, "bootstrap-contracts", false, "create deterministic manifests for review when native schemas changed")
	flag.Parse()
	if opts.sourceDir == "" {
		opts.sourceDir = opts.outDir
	}
	return opts
}

func main() {
	opts := parseFlags()
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if opts.check && opts.bootstrapContracts {
		return fmt.Errorf("--check and --bootstrap-contracts are mutually exclusive")
	}

	root, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	if opts.check {
		return checkGeneratedArtifacts(root, opts)
	}
	if err := os.MkdirAll(opts.outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	result, err := buildTrainingPack(root, opts.sourceDir, opts.bootstrapContracts)
	if err != nil {
		return err
	}
	if opts.bootstrapContracts {
		if err := writeSourceManifests(opts.outDir, result); err != nil {
			return err
		}
	}
	if err := writeGeneratedArtifacts(opts.outDir, result); err != nil {
		return err
	}
	fmt.Printf(
		"Exported training schema v%s: %d tools, %d operations, %d training scenarios, %d challenge scenarios to %s\n",
		datasetSchemaVersion,
		len(result.Tools),
		result.OperationCount,
		len(result.Scenarios),
		len(result.Challenge),
		opts.outDir,
	)
	return nil
}
