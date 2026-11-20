package tools

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"aurago/internal/security"
)

const homepageTailwindCSS = "tailwind.css"

func homepageTailwindLibrary(cfg HomepageConfig, project *os.Root, projectDir, assetRoot, version string, client *http.Client, compile func(HomepageConfig, *os.Root, string) ([]byte, error)) string {
	ctx := nativeHTTPContext(cfg.Context)
	refresh := version != ""
	if !refresh {
		var release struct{ Version string }
		if err := readHomepageLibraryJSON(ctx, client, "https://registry.npmjs.org/@tailwindcss%2fcli/latest", &release); err != nil {
			return ErrorJSON(err.Error())
		}
		if !homepageStableLibraryVersion.MatchString(release.Version) {
			return ErrorJSON("npm did not resolve a stable Tailwind CSS version")
		}
		version = release.Version
	}
	base := path.Join(assetRoot, "vendor", homepageLibraryTailwind, version)
	if refresh {
		if err := validateHereNowDirectoryComponents(project, base); err != nil {
			return ErrorJSON("install that Tailwind CSS version in this project before compiling it")
		}
		if _, err := readHomepageLibraryFile(project, path.Join(base, homepageTailwindCSS)); err != nil {
			return ErrorJSON("the installed Tailwind CSS file is missing or unsafe")
		}
	}
	css, err := compile(cfg, project, version)
	if err != nil {
		return ErrorJSON(err.Error())
	}
	if err := ctx.Err(); err != nil {
		return ErrorJSON(err.Error())
	}
	if len(bytes.TrimSpace(css)) == 0 || len(css) > maxHomepageWriteFileSize {
		return ErrorJSON("Tailwind CSS output is empty or exceeds the project file limit")
	}
	hash := sha256.Sum256(css)
	assets := []homepageLibraryAsset{{Name: homepageTailwindCSS, Hash: base64.StdEncoding.EncodeToString(hash[:]), Size: int64(len(css))}}
	var reused bool
	if refresh {
		name := path.Join(base, homepageTailwindCSS)
		var current []byte
		current, err = readHomepageLibraryFile(project, name)
		if err == nil {
			reused = bytes.Equal(current, css)
			if !reused {
				err = writeHomepageRevisionFile(ctx, project, name, css, 0644)
			}
		}
	} else {
		reused, err = publishHomepageLibrary(ctx, project, base, assets, func(homepageLibraryAsset) ([]byte, error) { return css, nil })
	}
	if err != nil {
		return ErrorJSON(err.Error())
	}
	return homepageLibraryResult(projectDir, assetRoot, homepageLibraryTailwind, version, homepageLibrary{CSS: homepageTailwindCSS}, assets, reused)
}

func compileHomepageTailwind(cfg HomepageConfig, project *os.Root, version string) ([]byte, error) {
	if err := requireDockerMutationPermission(); err != nil {
		return nil, fmt.Errorf("Tailwind CSS needs the managed Homepage dev container: %w", err)
	}
	ctx := nativeHTTPContext(cfg.Context)
	if !checkDockerAvailable(cfg.DockerHost, ctx) {
		return nil, fmt.Errorf("Tailwind CSS needs a running Docker engine and the managed Homepage dev container")
	}
	stage := ".aurago-tailwind-" + rand.Text()
	if err := project.Mkdir(stage, 0700); err != nil {
		return nil, err
	}
	defer project.RemoveAll(stage)
	if err := stageHomepageTailwindSources(cfg, project, stage); err != nil {
		return nil, err
	}
	if err := project.WriteFile(path.Join(stage, "package.json"), []byte(`{"private":true}`), 0600); err != nil {
		return nil, err
	}
	if err := project.WriteFile(path.Join(stage, "input.css"), []byte(`@import "tailwindcss" source("./source");`), 0600); err != nil {
		return nil, err
	}
	projectDir, err := filepath.Rel(cfg.WorkspacePath, project.Name())
	if err != nil {
		return nil, err
	}
	projectDir, err = NormalizeHomepageProjectIdentity(projectDir, false)
	if err != nil {
		return nil, err
	}
	stagingPath := path.Join(homepageWorkspaceMount, projectDir, stage)
	docker := DockerConfig{Context: ctx, Host: cfg.DockerHost}
	command := "cd " + shellSingleQuote(stagingPath) +
		" && timeout 100s npm install --ignore-scripts --no-audit --no-fund --save-exact --fetch-retries=0 --registry=https://registry.npmjs.org --userconfig=/dev/null --globalconfig=/dev/null " +
		shellSingleQuote("tailwindcss@"+version) + " " + shellSingleQuote("@tailwindcss/cli@"+version) +
		" && timeout 35s ./node_modules/.bin/tailwindcss -i input.css -o output.css --minify"
	result := homepageDockerExecFunc(docker, homepageContainerName, command, "")
	exitCode, _ := homepageDockerExecResult(result)
	if exitCode != 0 || !homepageBuildSucceeded(result) {
		return nil, fmt.Errorf("Tailwind CSS compilation failed; check that the Homepage dev container is running and can reach the npm registry")
	}
	return readHomepageLibraryFile(project, path.Join(stage, "output.css"))
}

func stageHomepageTailwindSources(cfg HomepageConfig, project *os.Root, stage string) error {
	if err := project.Mkdir(path.Join(stage, "source"), 0700); err != nil {
		return err
	}
	ctx := nativeHTTPContext(cfg.Context)
	count, total := 0, 0
	return fs.WalkDir(project.FS(), ".", func(rel string, item fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		skip := shouldExcludePath(rel) || homepageNonPublicArtifact(rel) || hereNowSensitivePath(rel)
		if item.IsDir() {
			if skip || path.Base(rel) == "vendor" || path.Base(rel) == "out" {
				return fs.SkipDir
			}
			return nil
		}
		if skip || !item.Type().IsRegular() {
			return nil
		}
		switch strings.ToLower(path.Ext(rel)) {
		case ".html", ".htm", ".js", ".jsx", ".ts", ".tsx", ".mjs", ".mts", ".cjs", ".cts", ".astro", ".vue", ".svelte", ".md", ".mdx":
		default:
			return nil
		}
		data, err := readHomepageLibraryFile(project, rel)
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("Tailwind source changed during compilation: %s", rel)
		}
		if err != nil {
			return err
		}
		secretScanner := &hereNowSecretScanner{}
		if _, err := secretScanner.Write(data); err != nil || security.Scrub(security.RedactSensitiveInfo(string(data))) != string(data) {
			return fmt.Errorf("Tailwind source %s appears to contain credentials; move credentials out of website source before compiling", rel)
		}
		count++
		total += len(data)
		if count > 1000 || total > 16<<20 {
			return fmt.Errorf("Tailwind source exceeds the 1,000-file or 16 MiB limit")
		}
		destination := path.Join(stage, "source", rel)
		if err := project.MkdirAll(path.Dir(destination), 0700); err != nil {
			return err
		}
		return project.WriteFile(destination, data, 0600)
	})
}
