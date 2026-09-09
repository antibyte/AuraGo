// assetpack creates the one deterministic, platform-independent web resource set.
package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"aurago/internal/webassets"
)

type productionRoot struct {
	Source      string   `json:"source"`
	Target      string   `json:"target"`
	Directories []string `json:"directories"`
	Extensions  []string `json:"extensions"`
	Exclude     []string `json:"exclude"`
}

func main() {
	out := flag.String("out", "deploy", "artifact output directory")
	stage := flag.String("stage", "assets/web", "installed asset root")
	release := flag.String("release", "", "immutable GitHub release tag; empty disables recovery download")
	flag.Parse()
	if err := build(*out, *stage, *release); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build(out, stage, release string) error {
	var spec struct {
		Version int              `json:"version"`
		Roots   []productionRoot `json:"roots"`
	}
	data, err := os.ReadFile("assets/web-assets.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		return err
	}
	if spec.Version != 1 {
		return fmt.Errorf("unsupported production manifest")
	}
	if release != "" && (strings.ContainsAny(release, "/\\ \t\r\n?&#") || release == "latest") {
		return fmt.Errorf("release must be an immutable tag")
	}
	files := map[string][]byte{}
	for _, root := range spec.Roots {
		err := fs.WalkDir(os.DirFS(root.Source), ".", func(name string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if name == "." {
				return nil
			}
			first := strings.Split(name, "/")[0]
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") || slices.Contains(root.Exclude, first) || (len(root.Directories) > 0 && !slices.Contains(root.Directories, first)) {
					return fs.SkipDir
				}
				return nil
			}
			license := strings.HasPrefix(strings.ToUpper(d.Name()), "LICENSE") || strings.HasPrefix(d.Name(), "THIRD_PARTY")
			if !license && !slices.Contains(root.Extensions, strings.ToLower(path.Ext(name))) {
				return nil
			}
			if strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") || strings.HasSuffix(name, ".map") {
				return nil
			}
			if !d.Type().IsRegular() {
				return fmt.Errorf("non-regular production asset %s/%s", root.Source, name)
			}
			b, err := os.ReadFile(filepath.Join(root.Source, filepath.FromSlash(name)))
			if err != nil {
				return err
			}
			switch strings.ToLower(path.Ext(name)) {
			case ".html", ".css", ".js", ".mjs", ".json", ".txt", ".md", ".svg", ".webmanifest":
				b = []byte(strings.ReplaceAll(string(b), "\r\n", "\n"))
			}
			key := path.Join(root.Target, name)
			if _, ok := files[key]; ok {
				return fmt.Errorf("duplicate asset %s", key)
			}
			files[key] = b
			return nil
		})
		if err != nil {
			return err
		}
	}
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	m := webassets.Manifest{Version: 1}
	var expanded int64
	for _, k := range keys {
		b := files[k]
		m.Files = append(m.Files, webassets.Entry{Path: k, Size: int64(len(b)), SHA256: webassets.Digest(b)})
		expanded += int64(len(b))
	}
	manifest, err := json.Marshal(m)
	if err != nil {
		return err
	}
	id := webassets.Digest(manifest)
	if _, err := webassets.ParseManifest(manifest, id); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	archiveName := "aurago-web-assets-" + id + ".tar.gz"
	archive, err := os.CreateTemp(out, ".assets-*")
	if err != nil {
		return err
	}
	defer os.Remove(archive.Name())
	defer archive.Close()
	gz, err := gzip.NewWriterLevel(archive, gzip.BestSpeed)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(gz)
	write := func(name string, b []byte) error {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(b)), Typeflag: tar.TypeReg, Format: tar.FormatPAX}); err != nil {
			return err
		}
		_, err := tw.Write(b)
		return err
	}
	if err := write(webassets.ManifestName, manifest); err != nil {
		return err
	}
	for _, k := range keys {
		if err := write(k, files[k]); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := archive.Close(); err != nil {
		return err
	}
	archiveData, err := os.ReadFile(archive.Name())
	if err != nil {
		return err
	}
	pin := webassets.Pin{ID: id, SHA256: webassets.Digest(archiveData), Bytes: int64(len(archiveData))}
	if release != "" {
		pin.URL = "https://github.com/antibyte/AuraGo/releases/download/" + release + "/" + archiveName
	}
	if err := os.Rename(archive.Name(), filepath.Join(out, archiveName)); err != nil {
		return err
	}
	// Install using the same verifier as a downloaded/offline release.
	if stage != "" {
		f, err := os.Open(filepath.Join(out, archiveName))
		if err != nil {
			return err
		}
		s := webassets.Open(stage, pin)
		err = s.Install(context.Background(), f)
		f.Close()
		s.Close()
		if err != nil {
			return err
		}
	}
	metadata, _ := json.MarshalIndent(pin, "", "  ")
	if err := os.WriteFile(filepath.Join(out, "web-assets.json"), append(metadata, '\n'), 0644); err != nil {
		return err
	}
	flags := fmt.Sprintf("-X aurago/internal/webassets.SetID=%s -X aurago/internal/webassets.ArchiveSHA256=%s -X aurago/internal/webassets.ArchiveBytes=%d", id, pin.SHA256, pin.Bytes)
	if pin.URL != "" {
		flags += " -X aurago/internal/webassets.DownloadURL=" + pin.URL
	}
	if err := os.WriteFile(filepath.Join(out, "web-assets.ldflags"), []byte(flags), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "web-assets.name"), []byte(archiveName), 0644); err != nil {
		return err
	}
	fmt.Printf("Assets %s: %d files, %.2f MB raw, %.2f MB archive\n", id, len(keys), float64(expanded)/1e6, float64(pin.Bytes)/1e6)
	return nil
}
