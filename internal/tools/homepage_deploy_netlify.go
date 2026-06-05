package tools

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func HomepageDeployNetlify(cfg HomepageConfig, nfCfg NetlifyConfig, projectDir, buildDir, siteID, title string, draft bool, logger *slog.Logger) string {
	if nfCfg.Token == "" {
		return errJSON("Netlify token is required")
	}
	if cfg.WorkspacePath == "" {
		return errJSON("Homepage workspace path is not configured")
	}
	if projectDir != "" && projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	if buildDir != "" && buildDir != "." {
		if err := sanitizeProjectDir(buildDir); err != nil {
			return errJSON("%v", err)
		}
	}
	if projectDir == "" {
		projectDir = "."
	}
	fallbackProjectDir := ""
	var fallbackCandidate *homepageDeployCandidate

	// Try to build first; ignore failure for plain-HTML projects that have no build script.
	if buildDir == "" {
		// For Next.js projects, ensure output: 'export' is configured before building.
		// Without it, `next build` produces only a .next/ server bundle that Netlify cannot serve.
		if cfg.WorkspacePath != "" {
			projectRoot := filepath.Join(cfg.WorkspacePath, projectDir)
			if isNextJsProject(projectRoot) {
				if patched := ensureNextJsStaticExport(projectRoot, logger); patched {
					logger.Info("[Homepage] Netlify: Next.js config patched for static export — building now")
				}
			}
		}
		logger.Info("[Homepage] Attempting build before Netlify deploy", "dir", projectDir)
		buildResult := HomepageBuildWithAutoFix(cfg, projectDir, logger)
		var br map[string]interface{}
		if err := json.Unmarshal([]byte(buildResult), &br); err == nil {
			if s, _ := br["status"].(string); s != "error" {
				// Build succeeded — detect the output directory.
				buildDir = detectBuildDir(cfg, projectDir)
			} else {
				if fallbackDir, candidate, ok := homepageNetlifyStaticFallbackCandidate(cfg, projectDir, logger); ok {
					logger.Info("[Homepage] Netlify: build failed, deploying static fallback",
						"original_project_dir", projectDir,
						"fallback_project_dir", fallbackDir,
						"fallback_build_dir", candidate.BuildDir)
					projectDir = fallbackDir
					buildDir = candidate.BuildDir
					fallbackProjectDir = fallbackDir
					fallbackCandidate = &candidate
				} else {
					return decorateHomepageBuildFailure(buildResult, projectDir)
				}
			}
		}
	}

	// If detectBuildDir returned ".next", the build was a Next.js server build (not static).
	// Patch the config and rebuild to produce a proper static output directory.
	if buildDir == ".next" && cfg.WorkspacePath != "" {
		projectRoot := filepath.Join(cfg.WorkspacePath, projectDir)
		logger.Info("[Homepage] Netlify: .next server build detected — patching Next.js config for static export and rebuilding")
		ensureNextJsStaticExport(projectRoot, logger)
		rebuildResult := HomepageBuildWithAutoFix(cfg, projectDir, logger)
		var rb map[string]interface{}
		if err := json.Unmarshal([]byte(rebuildResult), &rb); err == nil {
			if s, _ := rb["status"].(string); s != "error" {
				buildDir = detectBuildDir(cfg, projectDir)
			}
		}
		// If still .next after rebuild, fall through to project root (deploy will likely fail,
		// but at least we tried and the error will be visible in the deploy result).
		if buildDir == ".next" {
			buildDir = ""
		}
	}

	var candidate homepageDeployCandidate
	if fallbackCandidate != nil {
		candidate = *fallbackCandidate
	} else {
		var candidateErr error
		candidate, candidateErr = homepageDetectDeployCandidate(cfg, projectDir, buildDir, "")
		if candidateErr != nil {
			return errJSON("No valid Netlify deploy output for %q: %v. Netlify ZIP deploys require a static directory with index.html.", projectDir, candidateErr)
		}
	}
	deployPath := candidate.Path

	// Verify the deploy path exists.
	if _, err := os.Stat(deployPath); err != nil {
		return errJSON("Deploy path does not exist: %s. project_dir must be relative to the homepage workspace, and homepage project files must be created with homepage write_file/read_file instead of the filesystem tool. For static sites, ensure the project root or a dist/build/out directory contains index.html. Local published sites are served by container %q from /srv, not /var/www/html.", deployPath, homepageWebContainer)
	}

	logger.Info("[Homepage] Packaging for Netlify deploy", "path", deployPath)

	// Create an in-memory ZIP of the deploy directory.
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	// Track which special Netlify config files are already present in the project.
	var hasHeaders, hasNetlifyToml, hasRedirects bool

	// Collect /files/<subdir>/<name> references found in HTML/CSS/JS files so we
	// can bundle the actual files into the ZIP (they're served by AuraGo locally
	// but must be included as static assets for Netlify to serve them).
	type assetRef struct {
		subdir  string
		name    string
		zipPath string
	}
	referencedAssets := make(map[assetRef]struct{})
	assetRegexes := []struct {
		re        *regexp.Regexp
		subdir    string
		zipPrefix string
	}{
		{generatedImageRefRegex, "generated_images", "files/generated_images"},
		{legacyRootGeneratedImageRefRegex, "generated_images", ""},
		{generatedVideoRefRegex, "generated_videos", "files/generated_videos"},
		{audioFileRefRegex, "audio", "files/audio"},
		{documentFileRefRegex, "documents", "files/documents"},
	}

	walkErr := filepath.Walk(deployPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(deployPath, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "_headers" {
			hasHeaders = true
		}
		if rel == "netlify.toml" {
			hasNetlifyToml = true
		}
		if rel == "_redirects" {
			hasRedirects = true
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lowerRel := strings.ToLower(rel)
		if strings.HasSuffix(lowerRel, ".html") || strings.HasSuffix(lowerRel, ".htm") ||
			strings.HasSuffix(lowerRel, ".css") || strings.HasSuffix(lowerRel, ".js") {
			for _, ar := range assetRegexes {
				for _, m := range ar.re.FindAllSubmatch(data, -1) {
					if len(m) > 1 {
						referencedAssets[assetRef{subdir: ar.subdir, name: string(m[1]), zipPath: ar.zipPrefix}] = struct{}{}
					}
				}
			}
		}
		_, err = w.Write(data)
		return err
	})
	if walkErr != nil {
		return errJSON("Failed to create ZIP: %v", walkErr)
	}

	// Bundle any referenced assets into the ZIP so Netlify can serve
	// them at the same /files/<subdir>/<name> path.
	if cfg.DataDir != "" && len(referencedAssets) > 0 {
		bundled := 0
		for ref := range referencedAssets {
			srcPath := filepath.Join(cfg.DataDir, ref.subdir, filepath.Base(ref.name))
			srcData, readErr := os.ReadFile(srcPath)
			if readErr != nil {
				logger.Warn("[Homepage] Could not bundle asset", "subdir", ref.subdir, "file", ref.name, "error", readErr)
				continue
			}
			zipPath := filepath.Base(ref.name)
			if ref.zipPath != "" {
				zipPath = filepath.ToSlash(filepath.Join(ref.zipPath, zipPath))
			}
			if iw, werr := zw.Create(zipPath); werr == nil {
				_, _ = iw.Write(srcData)
				bundled++
			}
		}
		if bundled > 0 {
			logger.Info("[Homepage] Bundled referenced assets into deployment", "count", bundled)
		}
	}

	// Inject a _headers file if the project doesn't already have one.
	// When deploying via the Netlify ZIP API, MIME types are sometimes not
	// inferred from file extensions — explicit Content-Type headers fix this.
	if !hasHeaders {
		headersContent := "/*.html\n  Content-Type: text/html; charset=UTF-8\n/*.css\n  Content-Type: text/css; charset=UTF-8\n/*.js\n  Content-Type: application/javascript; charset=UTF-8\n"
		if w, err := zw.Create("_headers"); err == nil {
			_, _ = w.Write([]byte(headersContent))
		}
	}

	// Inject a minimal netlify.toml if the project doesn't already have one.
	if !hasNetlifyToml {
		tomlContent := "[[headers]]\n  for = \"/*.html\"\n  [headers.values]\n    Content-Type = \"text/html; charset=UTF-8\"\n\n[[headers]]\n  for = \"/*.css\"\n  [headers.values]\n    Content-Type = \"text/css; charset=UTF-8\"\n\n[[headers]]\n  for = \"/*.js\"\n  [headers.values]\n    Content-Type = \"application/javascript; charset=UTF-8\"\n\n[[redirects]]\n  from = \"/*\"\n  to = \"/index.html\"\n  status = 200\n"
		if w, err := zw.Create("netlify.toml"); err == nil {
			_, _ = w.Write([]byte(tomlContent))
		}
	}

	// Inject a _redirects file for SPA routing if the project doesn't have one.
	// This ensures single-page apps (React, Next.js static, Vue, etc.) serve
	// index.html for all routes instead of returning Netlify's default 404.
	if !hasRedirects {
		if w, err := zw.Create("_redirects"); err == nil {
			_, _ = w.Write([]byte("/*    /index.html   200\n"))
		}
	}

	if err := zw.Close(); err != nil {
		return errJSON("Failed to finalise ZIP: %v", err)
	}

	zipBytes := zipBuf.Bytes()
	if len(zipBytes) == 0 {
		return errJSON("ZIP is empty — check that %q contains files", deployPath)
	}

	// If siteID is a human-readable name (not a UUID), resolve it to a UUID first.
	// This avoids the Netlify API 404 that occurs when a name is passed where a UUID is expected.
	resolvedID := siteID
	if !looksLikeUUID(siteID) && siteID != "" {
		logger.Info("[Homepage] Resolving site name to UUID", "name", siteID)
		if uuid := netlifyResolveNameToID(nfCfg, siteID); uuid != "" {
			logger.Info("[Homepage] Site resolved", "name", siteID, "uuid", uuid)
			resolvedID = uuid
		}
		// If uuid == "", site doesn't exist yet — auto-create below before deploy.
	}
	if resolvedID == "" {
		resolvedID = strings.TrimSpace(nfCfg.DefaultSiteID)
	}
	if resolvedID == "" {
		if !nfCfg.AllowSiteManagement {
			return errJSON("Netlify site_id is required because netlify.allow_site_management is false and no default_site_id is configured")
		}
		siteName := homepageProviderNameFromProjectDir(projectDir)
		logger.Info("[Homepage] No Netlify site_id configured, auto-creating site", "name", siteName)
		createResult := NetlifyCreateSite(nfCfg, siteName, "")
		var cr map[string]interface{}
		if json.Unmarshal([]byte(createResult), &cr) != nil || cr["status"] != "ok" {
			return errJSON("Netlify site_id is missing and auto-creation failed: %s", createResult)
		}
		if id := strVal(cr, "id"); id != "" {
			resolvedID = id
		}
		if resolvedID == "" {
			return errJSON("Netlify site auto-creation did not return a site ID: %s", createResult)
		}
	}

	logger.Info("[Homepage] Deploying to Netlify", "site_id", resolvedID, "bytes", len(zipBytes), "draft", draft)
	deployResult := NetlifyDeployZip(nfCfg, resolvedID, title, draft, zipBytes)

	// If Netlify returned 404, the site doesn't exist yet — auto-create and retry.
	// Only do this when siteID was a name (not a UUID), to avoid recreating a deleted site by UUID.
	var dr map[string]interface{}
	if json.Unmarshal([]byte(deployResult), &dr) == nil {
		if code, _ := dr["http_code"].(float64); code == 404 && !looksLikeUUID(siteID) && nfCfg.AllowSiteManagement {
			logger.Info("[Homepage] Site not found, auto-creating", "name", siteID)
			createResult := NetlifyCreateSite(nfCfg, siteID, "")
			var cr map[string]interface{}
			if json.Unmarshal([]byte(createResult), &cr) == nil && cr["status"] == "ok" {
				newID, _ := cr["id"].(string)
				newDomain, _ := cr["default_domain"].(string)
				if newID != "" {
					logger.Info("[Homepage] Site created, retrying deploy", "site_id", newID, "domain", newDomain)
					deployResult = NetlifyDeployZip(nfCfg, newID, title, draft, zipBytes)
					// Annotate success with the auto-created site info
					var rr map[string]interface{}
					if json.Unmarshal([]byte(deployResult), &rr) == nil {
						rr["auto_created_site"] = true
						rr["new_site_id"] = newID
						rr["new_site_domain"] = newDomain
						if b, merr := json.Marshal(rr); merr == nil {
							deployResult = string(b)
						}
					}
				}
			} else {
				// Return create error to the agent
				return errJSON("Site %q not found and auto-creation failed: %s", siteID, createResult)
			}
		}
	}
	if json.Unmarshal([]byte(deployResult), &dr) != nil || dr["status"] == "error" {
		return deployResult
	}
	if fallbackProjectDir != "" {
		dr["fallback_project_dir"] = fallbackProjectDir
		dr["fallback_build_dir"] = candidate.BuildDir
	}
	deployID := strVal(dr, "id")
	if deployID != "" {
		waitResult := NetlifyWaitForDeploy(nfCfg, deployID, netlifyDeployPollAttempts, netlifyDeployPollInterval)
		var wr map[string]interface{}
		if json.Unmarshal([]byte(waitResult), &wr) == nil {
			if wr["status"] == "error" {
				return errJSON("Netlify deploy failed after upload: %s", strVal(wr, "message"))
			}
			for k, v := range wr {
				if k != "status" {
					dr["deploy_"+k] = v
				}
			}
		}
	}
	verifyURL := strVal(dr, "deploy_url")
	if verifyURL == "" {
		verifyURL = strVal(dr, "url")
	}
	if verifyURL == "" {
		verifyURL = strVal(dr, "deploy_deploy_url")
	}
	if verifyURL == "" {
		verifyURL = strVal(dr, "deploy_url")
	}
	if verifyURL != "" {
		verify := homepageVerifyDeploymentURL(verifyURL)
		if verify.Status != "ok" {
			return errJSON("Netlify deploy completed but live verification failed: %s", verify.Message)
		}
		dr["verified"] = true
		dr["verified_url"] = verify.URL
	}
	out, _ := json.Marshal(dr)
	return string(out)
}
