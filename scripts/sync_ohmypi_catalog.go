//go:build ignore

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/llm/catalogsync"
)

const npmPackageName = "@oh-my-pi/pi-catalog"

func main() {
	version := flag.String("version", "latest", "npm version or dist-tag to sync")
	check := flag.Bool("check", false, "compare generated catalog files without writing")
	write := flag.Bool("write", false, "write generated catalog files")
	flag.Parse()

	if *check == *write {
		exitf("pass exactly one of --check or --write")
	}
	if err := run(*version, *check, *write); err != nil {
		exitf("%v", err)
	}
}

func run(version string, check, write bool) error {
	pkg, err := fetchPackageMetadata(version)
	if err != nil {
		return err
	}
	tarball, err := download(pkg.Dist.Tarball, 128*1024*1024)
	if err != nil {
		return fmt.Errorf("download tarball: %w", err)
	}
	files, err := extractCatalogSources(tarball)
	if err != nil {
		return err
	}
	if len(files.packageJSON) > 0 {
		var tarPkg npmMetadata
		if err := json.Unmarshal(files.packageJSON, &tarPkg); err == nil {
			pkg.mergeMissing(tarPkg)
		}
	}
	snapshot, err := catalogsync.BuildSnapshot(files.modelsJSON, files.descriptorsTS, catalogsync.PackageMetadata{
		Name:          pkg.Name,
		Version:       pkg.Version,
		TarballURL:    pkg.Dist.Tarball,
		License:       pkg.License,
		RepositoryURL: pkg.repositoryURL(),
		Author:        pkg.authorName(),
	})
	if err != nil {
		return err
	}
	if check {
		preserveExistingSyncTimestamp(&snapshot)
	}
	modelsJSON, providersJSON, metadataJSON, err := catalogsync.MarshalSnapshotFiles(snapshot)
	if err != nil {
		return err
	}
	targets := map[string][]byte{
		filepath.Join("internal", "llm", "catalog", "ohmypi_models.json"):    modelsJSON,
		filepath.Join("internal", "llm", "catalog", "ohmypi_providers.json"): providersJSON,
		filepath.Join("internal", "llm", "catalog", "ohmypi_metadata.json"):  metadataJSON,
	}
	if check {
		var changed []string
		for path, content := range targets {
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, content) {
				changed = append(changed, path)
			}
		}
		if len(changed) > 0 {
			return fmt.Errorf("oh-my-pi catalog snapshot is out of date: %s", strings.Join(changed, ", "))
		}
		fmt.Printf("oh-my-pi catalog %s is current\n", pkg.Version)
		return nil
	}
	for path, content := range targets {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	fmt.Printf("synced oh-my-pi catalog %s (%d models, %d providers)\n", pkg.Version, len(snapshot.Models), len(snapshot.Providers))
	return nil
}

func preserveExistingSyncTimestamp(snapshot *catalogsync.Snapshot) {
	path := filepath.Join("internal", "llm", "catalog", "ohmypi_metadata.json")
	current, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var metadata struct {
		Version    string `json:"version"`
		TarballURL string `json:"tarball_url"`
		SyncedAt   string `json:"synced_at"`
	}
	if err := json.Unmarshal(current, &metadata); err != nil {
		return
	}
	if metadata.Version == snapshot.Metadata.Version &&
		metadata.TarballURL == snapshot.Metadata.TarballURL &&
		metadata.SyncedAt != "" {
		snapshot.Metadata.SyncedAt = metadata.SyncedAt
	}
}

type npmMetadata struct {
	Name       string          `json:"name"`
	Version    string          `json:"version"`
	License    string          `json:"license"`
	Author     json.RawMessage `json:"author"`
	Repository json.RawMessage `json:"repository"`
	Dist       struct {
		Tarball string `json:"tarball"`
	} `json:"dist"`
}

func (m *npmMetadata) mergeMissing(other npmMetadata) {
	if m.Name == "" {
		m.Name = other.Name
	}
	if m.Version == "" {
		m.Version = other.Version
	}
	if m.License == "" {
		m.License = other.License
	}
	if len(m.Author) == 0 {
		m.Author = other.Author
	}
	if len(m.Repository) == 0 {
		m.Repository = other.Repository
	}
}

func (m npmMetadata) repositoryURL() string {
	if len(m.Repository) == 0 {
		return ""
	}
	var obj struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(m.Repository, &obj); err == nil && obj.URL != "" {
		return obj.URL
	}
	var raw string
	if err := json.Unmarshal(m.Repository, &raw); err == nil {
		return raw
	}
	return ""
}

func (m npmMetadata) authorName() string {
	if len(m.Author) == 0 {
		return ""
	}
	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(m.Author, &obj); err == nil && obj.Name != "" {
		return obj.Name
	}
	var raw string
	if err := json.Unmarshal(m.Author, &raw); err == nil {
		return raw
	}
	return ""
}

func fetchPackageMetadata(version string) (npmMetadata, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "latest"
	}
	url := "https://registry.npmjs.org/@oh-my-pi%2fpi-catalog/" + version
	body, err := download(url, 8*1024*1024)
	if err != nil {
		return npmMetadata{}, fmt.Errorf("fetch npm metadata: %w", err)
	}
	var pkg npmMetadata
	if err := json.Unmarshal(body, &pkg); err != nil {
		return npmMetadata{}, fmt.Errorf("parse npm metadata: %w", err)
	}
	if pkg.Name == "" {
		pkg.Name = npmPackageName
	}
	if pkg.Dist.Tarball == "" {
		return npmMetadata{}, fmt.Errorf("npm metadata for %s has no dist.tarball", version)
	}
	return pkg, nil
}

func download(url string, maxBytes int64) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) >= maxBytes {
		return nil, fmt.Errorf("response from %s exceeded %d bytes", url, maxBytes)
	}
	return body, nil
}

type catalogSources struct {
	modelsJSON    []byte
	descriptorsTS []byte
	packageJSON   []byte
}

func extractCatalogSources(tarball []byte) (catalogSources, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tarball))
	if err != nil {
		return catalogSources{}, fmt.Errorf("open tarball gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var files catalogSources
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return catalogSources{}, fmt.Errorf("read tarball: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		switch filepath.ToSlash(header.Name) {
		case "package/src/models.json":
			files.modelsJSON, err = io.ReadAll(io.LimitReader(tr, 64*1024*1024))
		case "package/src/provider-models/descriptors.ts":
			files.descriptorsTS, err = io.ReadAll(io.LimitReader(tr, 8*1024*1024))
		case "package/package.json":
			files.packageJSON, err = io.ReadAll(io.LimitReader(tr, 1024*1024))
		default:
			continue
		}
		if err != nil {
			return catalogSources{}, fmt.Errorf("extract %s: %w", header.Name, err)
		}
	}
	if len(files.modelsJSON) == 0 {
		return catalogSources{}, fmt.Errorf("tarball missing package/src/models.json")
	}
	if len(files.descriptorsTS) == 0 {
		return catalogSources{}, fmt.Errorf("tarball missing package/src/provider-models/descriptors.ts")
	}
	if len(files.packageJSON) == 0 {
		return catalogSources{}, fmt.Errorf("tarball missing package/package.json")
	}
	return files, nil
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
