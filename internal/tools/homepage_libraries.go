package tools

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/security"
)

const homepageCDNMetadata = "https://data.jsdelivr.com/v1/packages/npm/"
const homepageCDNFiles = "https://cdn.jsdelivr.net/npm/"
const homepageLibraryTailwind = "tailwindcss"

type homepageLibrary struct {
	Package     string
	CSS         string
	Script      string
	Support     string
	Load        string
	LicenseRepo string
}

var homepageLibraries = map[string]homepageLibrary{
	"bootstrap": {Package: "bootstrap", CSS: "dist/css/bootstrap.min.css", Script: "dist/js/bootstrap.bundle.min.js", Load: "defer"},
	"alpinejs":  {Package: "alpinejs", Script: "dist/cdn.min.js", Load: "defer", LicenseRepo: "alpinejs/alpine"},
	"htmx":      {Package: "htmx.org", Script: "dist/htmx.min.js", Load: "defer"},
	"chartjs":   {Package: "chart.js", Script: "dist/chart.umd.min.js", Load: "defer"},
	"swiper":    {Package: "swiper", CSS: "swiper-bundle.min.css", Script: "swiper-bundle.min.js", Load: "defer"},
	"three":     {Package: "three", Script: "build/three.module.js", Support: "build/three.core.js", Load: "module"},
}

var homepageStableLibraryVersion = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)

func InstallHomepageLibrary(cfg HomepageConfig, projectDir, library string) string {
	if library != homepageLibraryTailwind && homepageLibraries[library].Package == "" {
		return ErrorJSON("unknown library; choose bootstrap, alpinejs, htmx, chartjs, swiper, three or tailwindcss")
	}
	ctx, cancel := context.WithTimeout(nativeHTTPContext(cfg.Context), 3*time.Minute)
	defer cancel()
	cfg.Context = ctx
	return runHomepageLibrary(cfg, projectDir, library, "", newHomepageLibraryClient(), compileHomepageTailwind)
}

func CompileHomepageTailwind(cfg HomepageConfig, projectDir, version string) string {
	if !homepageStableLibraryVersion.MatchString(version) {
		return ErrorJSON("version must be the exact installed Tailwind CSS version, for example 4.1.0")
	}
	ctx, cancel := context.WithTimeout(nativeHTTPContext(cfg.Context), 3*time.Minute)
	defer cancel()
	cfg.Context = ctx
	return runHomepageLibrary(cfg, projectDir, homepageLibraryTailwind, version, newHomepageLibraryClient(), compileHomepageTailwind)
}

func newHomepageLibraryClient() *http.Client {
	client := security.NewPublicHTTPClient(25 * time.Second)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return client
}

type homepageLibraryAsset struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
	URL  string `json:"-"`
}

func runHomepageLibrary(cfg HomepageConfig, projectDir, library, version string, client *http.Client, compile func(HomepageConfig, *os.Root, string) ([]byte, error)) string {
	project, err := openHomepageLibraryProject(cfg, projectDir)
	if err != nil {
		return ErrorJSON(err.Error())
	}
	defer project.Close()
	assetRoot, err := homepageLibraryAssetRoot(project)
	if err != nil {
		return ErrorJSON(err.Error())
	}
	ctx := nativeHTTPContext(cfg.Context)
	if library == homepageLibraryTailwind {
		return homepageTailwindLibrary(cfg, project, projectDir, assetRoot, version, client, compile)
	}
	entry := homepageLibraries[library]
	version, assets, err := resolveHomepageCDNLibrary(ctx, client, entry)
	if err != nil {
		return ErrorJSON(err.Error())
	}
	base := path.Join(assetRoot, "vendor", library, version)
	reused, err := publishHomepageLibrary(ctx, project, base, assets, func(asset homepageLibraryAsset) ([]byte, error) {
		return readHomepageLibraryURL(ctx, client, asset.URL, asset.Size)
	})
	if err != nil {
		return ErrorJSON(err.Error())
	}
	if library == "bootstrap" {
		major := homepageStableLibraryVersion.FindStringSubmatch(version)[1]
		if len(major) > 1 || major >= "6" {
			entry.Load = "module"
		}
	}
	return homepageLibraryResult(projectDir, assetRoot, library, version, entry, assets, reused)
}

func openHomepageLibraryProject(cfg HomepageConfig, projectDir string) (*os.Root, error) {
	canonical, err := NormalizeHomepageProjectIdentity(projectDir, false)
	if err != nil {
		return nil, err
	}
	if cfg.WorkspacePath == "" {
		return nil, fmt.Errorf("homepage workspace_path is required")
	}
	workspace, err := os.OpenRoot(cfg.WorkspacePath)
	if err != nil {
		return nil, fmt.Errorf("open Homepage workspace: %w", err)
	}
	defer workspace.Close()
	if err := validateHereNowDirectoryComponents(workspace, canonical); err != nil {
		return nil, fmt.Errorf("library installation requires an existing project directory: %w", err)
	}
	return workspace.OpenRoot(canonical)
}

func homepageLibraryAssetRoot(project *os.Root) (string, error) {
	var packages struct {
		Dependencies    map[string]json.RawMessage
		DevDependencies map[string]json.RawMessage
	}
	data, err := readHomepageLibraryFile(project, "package.json")
	if err == nil {
		if err := json.Unmarshal(data, &packages); err != nil {
			return "", fmt.Errorf("read project package.json: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if packages.Dependencies["@sveltejs/kit"] != nil || packages.DevDependencies["@sveltejs/kit"] != nil {
		return "static", nil
	}
	for _, pkg := range []string{"next", "vite", "astro", "nuxt", "react-scripts"} {
		if packages.Dependencies[pkg] != nil || packages.DevDependencies[pkg] != nil {
			return "public", nil
		}
	}
	switch detectHomepageFramework(project.Name()) {
	case "nextjs", "vite", "astro", "nuxtjs":
		return "public", nil
	}
	if info, err := project.Lstat("index.html"); err == nil && info.Mode().IsRegular() {
		return ".", nil
	}
	return "", fmt.Errorf("library installation requires static HTML or a supported project using a standard public asset directory")
}

func resolveHomepageCDNLibrary(ctx context.Context, client *http.Client, library homepageLibrary) (string, []homepageLibraryAsset, error) {
	var resolved struct{ Version string }
	if err := readHomepageLibraryJSON(ctx, client, homepageCDNMetadata+library.Package+"/resolved?specifier=latest", &resolved); err != nil {
		return "", nil, err
	}
	if !homepageStableLibraryVersion.MatchString(resolved.Version) {
		return "", nil, fmt.Errorf("CDN did not resolve a stable version of %s", library.Package)
	}
	npmRelease := library.Package + "@" + resolved.Version
	files, err := homepageLibraryManifest(ctx, client, homepageCDNMetadata+npmRelease, homepageCDNFiles+npmRelease)
	if err != nil {
		return "", nil, err
	}
	licenseFiles := files
	if library.LicenseRepo != "" {
		release := library.LicenseRepo + "@v" + resolved.Version
		licenseFiles, err = homepageLibraryManifest(ctx, client, "https://data.jsdelivr.com/v1/packages/gh/"+release, "https://cdn.jsdelivr.net/gh/"+release)
		if err != nil {
			return "", nil, err
		}
	}
	license := homepageLibraryAsset{}
	for _, candidate := range []string{"/LICENSE", "/LICENSE.md", "/LICENSE.txt", "/LICENCE", "/license"} {
		if asset, found := licenseFiles[candidate]; found {
			license = asset
			break
		}
	}
	if license.Name == "" {
		return "", nil, fmt.Errorf("%s %s has no license file in its CDN release metadata", library.Package, resolved.Version)
	}
	var assets []homepageLibraryAsset
	for _, name := range []string{library.CSS, library.Script, library.Support} {
		if name == "" {
			continue
		}
		asset, found := files["/"+name]
		if !found {
			return "", nil, fmt.Errorf("%s %s is missing %s in its CDN release metadata", library.Package, resolved.Version, name)
		}
		assets = append(assets, asset)
	}
	assets = append(assets, license)
	for _, asset := range assets {
		hash, err := base64.StdEncoding.DecodeString(asset.Hash)
		if err != nil || len(hash) != sha256.Size || asset.Size < 1 || asset.Size > maxHomepageWriteFileSize {
			return "", nil, fmt.Errorf("%s %s has invalid CDN metadata for %s", library.Package, resolved.Version, asset.Name)
		}
	}
	return resolved.Version, assets, nil
}

func homepageLibraryManifest(ctx context.Context, client *http.Client, metadataURL, downloadBase string) (map[string]homepageLibraryAsset, error) {
	var manifest struct{ Files []homepageLibraryAsset }
	if err := readHomepageLibraryJSON(ctx, client, metadataURL+"?structure=flat", &manifest); err != nil {
		return nil, err
	}
	files := make(map[string]homepageLibraryAsset, len(manifest.Files))
	for _, asset := range manifest.Files {
		asset.URL = downloadBase + asset.Name
		files[asset.Name] = asset
	}
	return files, nil
}

func readHomepageLibraryJSON(ctx context.Context, client *http.Client, url string, target any) error {
	data, err := readHomepageLibraryURL(ctx, client, url, maxHomepageWriteFileSize)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("invalid library release metadata: %w", err)
	}
	return nil
}

func readHomepageLibraryURL(ctx context.Context, client *http.Client, url string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "AuraGo Homepage Studio (https://github.com/antibyte/AuraGo)")
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch library asset: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > maxBytes {
		return nil, fmt.Errorf("library source returned HTTP %d or exceeded the size limit", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("library response was empty or exceeded the size limit")
	}
	return data, nil
}

func readHomepageLibraryFile(root *os.Root, name string) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxHomepageWriteFileSize {
		return nil, fmt.Errorf("expected a bounded regular file at %s", name)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("file changed while opening %s", name)
	}
	return homepageReadDocument(file)
}

func (asset homepageLibraryAsset) matches(data []byte) bool {
	hash := sha256.Sum256(data)
	return int64(len(data)) == asset.Size && base64.StdEncoding.EncodeToString(hash[:]) == asset.Hash
}

func publishHomepageLibrary(ctx context.Context, project *os.Root, base string, assets []homepageLibraryAsset, fetch func(homepageLibraryAsset) ([]byte, error)) (bool, error) {
	parent := path.Dir(base)
	if err := makeHomepageLibraryDirectory(project, parent); err != nil {
		return false, err
	}
	if _, err := project.Lstat(base); err == nil {
		if err := validateHereNowDirectoryComponents(project, base); err != nil {
			return false, err
		}
		for _, asset := range assets {
			data, err := readHomepageLibraryFile(project, path.Join(base, path.Base(asset.Name)))
			if err != nil || !asset.matches(data) {
				return false, fmt.Errorf("existing library file %s failed verification; inspect the project files before retrying", path.Base(asset.Name))
			}
		}
		return true, ctx.Err()
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	stage := path.Join(parent, ".aurago-library-"+rand.Text())
	if err := project.Mkdir(stage, 0755); err != nil {
		return false, err
	}
	defer project.RemoveAll(stage)
	for _, asset := range assets {
		data, err := fetch(asset)
		if err != nil {
			return false, err
		}
		if !asset.matches(data) {
			return false, fmt.Errorf("library file %s failed size or SHA-256 verification", path.Base(asset.Name))
		}
		if err := writeHomepageRevisionFile(ctx, project, path.Join(stage, path.Base(asset.Name)), data, 0644); err != nil {
			return false, err
		}
	}
	if _, err := project.Lstat(base); !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("library destination changed during installation")
	}
	return false, fileutil.RenameRootContext(ctx, project, stage, base)
}

func makeHomepageLibraryDirectory(project *os.Root, dir string) error {
	current := "."
	for _, part := range strings.Split(dir, "/") {
		current = path.Join(current, part)
		if err := project.Mkdir(filepath.FromSlash(current), 0755); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		if err := validateHereNowDirectoryComponents(project, current); err != nil {
			return err
		}
	}
	return nil
}

func homepageLibraryResult(projectDir, assetRoot, library, version string, entry homepageLibrary, assets []homepageLibraryAsset, reused bool) string {
	base := path.Join("vendor", library, version)
	result := map[string]any{"status": "ok", "project_dir": projectDir, "library": library, "version": version, "reused": reused}
	var files []string
	for _, asset := range assets {
		files = append(files, path.Join(projectDir, assetRoot, base, path.Base(asset.Name)))
	}
	result["files"] = files
	if entry.CSS != "" {
		result["stylesheet"] = path.Join(base, path.Base(entry.CSS))
	}
	if entry.Script != "" {
		result["script"] = path.Join(base, path.Base(entry.Script))
		result["script_mode"] = entry.Load
	}
	encoded, _ := json.Marshal(result)
	return string(encoded)
}
