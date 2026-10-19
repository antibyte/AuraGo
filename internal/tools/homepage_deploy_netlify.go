package tools

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// netlifyZipMaxEntries mirrors Netlify's ZIP extraction limit.
	netlifyZipMaxEntries = 25000
	// netlifyZipMaxBytes keeps a deploy inside a sane upload and memory budget.
	netlifyZipMaxBytes int64 = 100 << 20
)

var (
	errNetlifyZipTooManyEntries = errors.New("too many files for a Netlify ZIP deploy (limit 25000)")
	errNetlifyZipTooLarge       = errors.New("static output exceeds the Netlify deploy budget of 100 MiB")
)

// netlifyZipStats describes what one ZIP build published.
type netlifyZipStats struct {
	Files       int
	Bytes       int64
	Skipped     int
	Bundled     int
	SPAFallback bool
}

func homepageNetlifyShouldSkipZipEntry(rel string, info os.FileInfo) bool {
	rel = strings.Trim(strings.ToLower(filepath.ToSlash(rel)), "/")
	if rel == "" || rel == "." {
		return false
	}
	parts := strings.Split(rel, "/")
	skipDirs := map[string]struct{}{
		".git":          {},
		".cache":        {},
		".next":         {},
		".netlify":      {},
		".parcel-cache": {},
		".turbo":        {},
		".vercel":       {},
		".vite":         {},
		"node_modules":  {},
	}
	for _, part := range parts {
		if _, ok := skipDirs[part]; ok {
			return true
		}
	}
	if info != nil && info.IsDir() {
		return false
	}
	base := parts[len(parts)-1]
	// Credential material must never reach a public site, whatever its name.
	if base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".env") {
		return true
	}
	skipFiles := map[string]struct{}{
		".aurago-site-manifest.json": {},
		".git-credentials":           {},
		".htpasswd":                  {},
		".netrc":                     {},
		".npmrc":                     {},
		".pnpmrc":                    {},
		".yarnrc":                    {},
		".yarnrc.yml":                {},
		"config.yaml":                {},
		"config.yml":                 {},
		"credentials.json":           {},
		"id_dsa":                     {},
		"id_ecdsa":                   {},
		"id_ed25519":                 {},
		"id_rsa":                     {},
		"secret.json":                {},
		"secrets.json":               {},
		"service-account.json":       {},
		"vault.bin":                  {},
	}
	if _, ok := skipFiles[base]; ok {
		return true
	}
	for _, suffix := range []string{
		".db", ".sqlite", ".sqlite3", ".sqlite-shm", ".sqlite-wal", ".log",
		".pem", ".key", ".p12", ".pfx", ".crt", ".jks", ".keystore", ".kdbx",
	} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

// netlifyLooksLikeSPA reports whether the deploy tree is a single-page app:
// exactly one document at the root, optionally with a Netlify 404 page.
// Multi-page sites keep Netlify's real 404 instead of a soft 200 fallback.
func netlifyLooksLikeSPA(htmlPaths []string) bool {
	sawIndex := false
	for _, path := range htmlPaths {
		switch strings.ToLower(path) {
		case "index.html":
			sawIndex = true
		case "404.html":
		default:
			return false
		}
	}
	return sawIndex
}

// buildNetlifyZip packages a deploy directory. Only regular files are read, so
// a symlink in the build output can never publish a file from outside the tree.
func buildNetlifyZip(cfg HomepageConfig, deployPath string, logger *slog.Logger) ([]byte, netlifyZipStats, error) {
	var stats netlifyZipStats
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	// Track which special Netlify config files are already present in the project.
	var hasHeaders, hasRedirects bool
	writtenZipPaths := make(map[string]struct{})
	var htmlPaths []string

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
		{generatedImageAssetRefRegex, "generated_images", "assets"},
		{generatedVideoRefRegex, "generated_videos", "files/generated_videos"},
		{audioFileRefRegex, "audio", "files/audio"},
		{documentFileRefRegex, "documents", "files/documents"},
	}

	walkErr := filepath.Walk(deployPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(deployPath, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			if homepageNetlifyShouldSkipZipEntry(rel, info) {
				return filepath.SkipDir
			}
			return nil
		}
		// Lstat semantics: a symlink is never IsRegular, so its target is never read.
		if !info.Mode().IsRegular() {
			stats.Skipped++
			return nil
		}
		if homepageNetlifyShouldSkipZipEntry(rel, info) {
			stats.Skipped++
			return nil
		}
		if stats.Files >= netlifyZipMaxEntries {
			return errNetlifyZipTooManyEntries
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		stats.Bytes += int64(len(data))
		if stats.Bytes > netlifyZipMaxBytes {
			return errNetlifyZipTooLarge
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		writtenZipPaths[rel] = struct{}{}
		stats.Files++
		if rel == "_headers" {
			hasHeaders = true
		}
		if rel == "_redirects" {
			hasRedirects = true
		}
		lowerRel := strings.ToLower(rel)
		if strings.HasSuffix(lowerRel, ".html") || strings.HasSuffix(lowerRel, ".htm") {
			htmlPaths = append(htmlPaths, rel)
		}
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
		return nil, stats, walkErr
	}

	// Bundle any referenced assets into the ZIP so Netlify can serve
	// them at the same /files/<subdir>/<name> path.
	if cfg.DataDir != "" && len(referencedAssets) > 0 {
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
			if _, exists := writtenZipPaths[zipPath]; exists {
				continue
			}
			if iw, werr := zw.Create(zipPath); werr == nil {
				_, _ = iw.Write(srcData)
				writtenZipPaths[zipPath] = struct{}{}
				stats.Bundled++
			}
		}
		if stats.Bundled > 0 {
			logger.Info("[Homepage] Bundled referenced assets into deployment", "count", stats.Bundled)
		}
	}

	// Inject a _headers file if the project doesn't already have one. This is
	// the single MIME source for the deploy; a generated netlify.toml would
	// duplicate the same rules in a second syntax.
	if !hasHeaders {
		headersContent := "/*.html\n  Content-Type: text/html; charset=UTF-8\n/*.css\n  Content-Type: text/css; charset=UTF-8\n/*.js\n  Content-Type: application/javascript; charset=UTF-8\n"
		if w, err := zw.Create("_headers"); err == nil {
			_, _ = w.Write([]byte(headersContent))
		}
	}

	// A single-page app needs a rewrite so deep links reach index.html. A
	// multi-page site must keep Netlify's 404 instead of a soft 200.
	stats.SPAFallback = !hasRedirects && netlifyLooksLikeSPA(htmlPaths)
	if stats.SPAFallback {
		if w, err := zw.Create("_redirects"); err == nil {
			_, _ = w.Write([]byte("/*    /index.html   200\n"))
		}
	}

	if err := zw.Close(); err != nil {
		return nil, stats, err
	}
	return zipBuf.Bytes(), stats, nil
}

// mergeNetlifyDeployObservation copies a poll response into the deploy envelope.
// deploy_url is the immutable deploy permalink and url is the published site
// URL; keeping them apart stops a draft deploy from verifying the live site.
func mergeNetlifyDeployObservation(dst, observation map[string]interface{}) {
	set := func(dstKey, srcKey string) {
		if value := strVal(observation, srcKey); value != "" {
			dst[dstKey] = value
		}
	}
	set("deploy_url", "deploy_url")
	if siteURL := strVal(observation, "url"); siteURL != "" {
		dst["site_url"] = siteURL
	}
	set("state", "state")
	set("deploy_branch", "branch")
	set("deploy_context", "context")
	set("deploy_title", "title")
	set("deploy_created_at", "created_at")
	set("deploy_error_message", "error_message")
	set("deploy_id", "id")
	if attempts, ok := observation["attempts"]; ok {
		dst["deploy_attempts"] = attempts
	}
}

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
		if err := json.Unmarshal([]byte(buildResult), &br); err != nil {
			// An unreadable build answer is not a success: never deploy blindly.
			return errJSON("Homepage build returned an unreadable result: %v", err)
		}
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
	if manifest, manifestErr := BuildHomepageArtifactManifest(cfg, 0, projectDir, 0, "", candidate.BuildDir); manifestErr == nil {
		if writeErr := WriteHomepageArtifactManifest(cfg, manifest); writeErr != nil {
			logger.Warn("[Homepage] Netlify: failed to write deployment manifest", "project_dir", projectDir, "build_dir", candidate.BuildDir, "error", writeErr)
		}
	} else {
		logger.Warn("[Homepage] Netlify: failed to build deployment manifest", "project_dir", projectDir, "build_dir", candidate.BuildDir, "error", manifestErr)
	}

	logger.Info("[Homepage] Packaging for Netlify deploy", "path", deployPath)

	zipBytes, zipStats, zipErr := buildNetlifyZip(cfg, deployPath, logger)
	if zipErr != nil {
		return errJSON("Failed to create ZIP: %v", zipErr)
	}
	if len(zipBytes) == 0 {
		return errJSON("ZIP is empty — check that %q contains files", deployPath)
	}
	logger.Info("[Homepage] Netlify ZIP ready",
		"bytes", zipStats.Bytes,
		"compressed_bytes", len(zipBytes),
		"files", zipStats.Files,
		"skipped", zipStats.Skipped,
		"spa_fallback", zipStats.SPAFallback)

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
					resolvedID = newID
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
	if strVal(dr, "site_id") == "" {
		dr["site_id"] = resolvedID
	}
	if fallbackProjectDir != "" {
		dr["fallback_project_dir"] = fallbackProjectDir
		dr["fallback_build_dir"] = candidate.BuildDir
	}
	deployID := firstNonEmptyString(strVal(dr, "deploy_id"), strVal(dr, "id"))
	if deployID == "" {
		return errJSON("Netlify deploy response missing deploy_id")
	}
	dr["deploy_id"] = deployID
	waitResult := NetlifyWaitForDeploy(nfCfg, deployID, netlifyDeployPollAttempts, netlifyDeployPollInterval)
	var wr map[string]interface{}
	if json.Unmarshal([]byte(waitResult), &wr) != nil {
		return errJSON("Netlify deploy status could not be parsed: %s", waitResult)
	}
	if wr["status"] == "error" {
		msg := firstNonEmptyString(strVal(wr, "message"), strVal(wr, "error_message"), waitResult)
		return errJSON("Netlify deploy failed after upload: %s", msg)
	}
	mergeNetlifyDeployObservation(dr, wr)

	// A deploy is only reported as published when it was not a draft. A draft
	// never changes the site, so the ledger must not record it as a deployment.
	dr["published"] = !draft
	dr["spa_fallback"] = zipStats.SPAFallback
	// Render providers that only report the published site URL land in url or
	// site_url; the deploy permalink always wins when it exists.
	verifyURL := firstNonEmptyString(
		strVal(dr, "deploy_url"),
		strVal(dr, "deploy_deploy_url"),
		strVal(dr, "url"),
		strVal(dr, "site_url"),
	)
	if verifyURL == "" {
		return errJSON("Netlify deploy completed but no verification URL was returned")
	}
	verify := homepageVerifyDeploymentURL(verifyURL, cfg.Context)
	if verify.Status != "ok" {
		if !draft {
			return errJSON("Netlify deploy completed but live verification failed: %s", verify.Message)
		}
		// A draft deploy's permalink may require a Netlify login. Report the
		// deploy without claiming a live verification or a publication.
		dr["verified"] = false
		dr["verification_note"] = "draft deploy: the deploy permalink could not be verified (" + verify.Message + ")"
		out, _ := json.Marshal(dr)
		return string(out)
	}
	dr["verified"] = true
	dr["verified_url"] = verify.URL
	out, _ := json.Marshal(dr)
	return string(out)
}
