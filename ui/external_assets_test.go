package ui

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// AuraGo ships every font, image and library it renders itself; no page may pull them from the Internet.
var (
	externalAssetHosts   = []string{"fonts.googleapis.com", "fonts.gstatic.com", "googletagmanager.com", "google-analytics.com", "perplexity.ai/fonts"}
	externalCSSLoad      = regexp.MustCompile(`(?i)@import\s+(?:url\()?\s*['"]?(?:https?:)?//|url\(\s*['"]?(?:https?:)?//`)
	externalHTMLElement  = regexp.MustCompile(`(?is)<(?:script|img|iframe|video|audio|source|embed)\b[^>]*?\ssrc\s*=\s*['"]?(?:https?:)?//`)
	htmlLinkTag          = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	htmlLinkLoadingRel   = regexp.MustCompile(`(?i)\srel\s*=\s*['"]?[^'">]*\b(?:stylesheet|preload|modulepreload|prefetch|preconnect|dns-prefetch|icon|manifest)\b`)
	htmlLinkExternalHref = regexp.MustCompile(`(?i)\shref\s*=\s*['"]?(?:https?:)?//`)
	// A Tailwind arbitrary-value class compiled into the sheets engine; no element uses it.
	externalAssetAllowed = []string{"https://example.com/a-b.svg"}
)

func TestShippedWebAssetsLoadNothingExternal(t *testing.T) {
	var failures []string
	for _, root := range []string{".", filepath.Join("..", "internal", "desktop", "bundled_apps")} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || d.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".html" && ext != ".htm" && ext != ".css" && ext != ".js" && ext != ".mjs" {
				return nil
			}
			if strings.Contains(d.Name(), ".test.") || strings.Contains(d.Name(), "_test.") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(data)
			for _, allowed := range externalAssetAllowed {
				text = strings.ReplaceAll(text, allowed, "")
			}
			for _, host := range externalAssetHosts {
				if strings.Contains(text, host) {
					failures = append(failures, path+": references "+host)
				}
			}
			switch ext {
			case ".css":
				if m := externalCSSLoad.FindString(text); m != "" {
					failures = append(failures, path+": remote stylesheet load "+m)
				}
			case ".html", ".htm":
				if m := externalHTMLElement.FindString(text); m != "" {
					failures = append(failures, path+": remote element "+truncateForReport(m))
				}
				for _, tag := range htmlLinkTag.FindAllString(text, -1) {
					if htmlLinkLoadingRel.MatchString(tag) && htmlLinkExternalHref.MatchString(tag) {
						failures = append(failures, path+": remote link "+truncateForReport(tag))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	for _, failure := range failures {
		t.Error(failure)
	}
}

func truncateForReport(s string) string {
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}
