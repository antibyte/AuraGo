package tools

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// HomepageInitTargetCheck refuses init_project for a name whose directory already
// holds content, so a scaffold can never overwrite an existing site. db is optional;
// when given, the error names the managed_site_id of a registered project.
func HomepageInitTargetCheck(cfg HomepageConfig, db *sql.DB, name string) error {
	if cfg.WorkspacePath == "" {
		return nil
	}
	if name == "" {
		name = "my-site"
	}
	if validateHomepageProjectName(name) != nil {
		return nil // the scaffold reports invalid names itself
	}
	root := homepageLocalRoot(cfg, name)
	info, err := os.Stat(root)
	if err != nil {
		return nil
	}
	if info.IsDir() {
		entries, readErr := os.ReadDir(root)
		if readErr == nil && len(entries) == 0 {
			return nil
		}
	}
	hint := "choose a new name, or edit the existing site"
	if db != nil {
		if proj, getErr := GetProjectByDir(db, name); getErr == nil && proj != nil && proj.ID > 0 {
			hint = fmt.Sprintf("choose a new name, or edit the existing site (managed_site_id %d)", proj.ID)
		}
	}
	if !info.IsDir() {
		return fmt.Errorf("project path %q already exists as a file; %s", name, hint)
	}
	return fmt.Errorf("project directory %q already exists and is not empty; %s", name, hint)
}

func homepageInitTargetError(cfg HomepageConfig, name string) error {
	return HomepageInitTargetCheck(cfg, nil, name)
}

func homepageNormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.TrimSuffix(raw, "/")
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
		u.Scheme = strings.ToLower(u.Scheme)
		u.Host = strings.ToLower(u.Host)
		return u.String()
	}
	return raw
}

// HomepageLighthouseSite returns the managed site a Lighthouse run for rawURL
// belongs to. managedSiteID wins when set (and must exist); otherwise rawURL must
// equal, after normalization, the URL of exactly one project or deploy target.
// ok is false when the attribution is missing or ambiguous.
func HomepageLighthouseSite(db *sql.DB, managedSiteID int64, rawURL string) (int64, bool) {
	if db == nil {
		return 0, false
	}
	if managedSiteID > 0 {
		site, err := GetHomepageManagedSite(db, managedSiteID)
		if err != nil {
			return 0, false
		}
		return site.ID, true
	}
	want := homepageNormalizeURL(rawURL)
	if want == "" {
		return 0, false
	}
	sites, err := ListHomepageManagedSites(db)
	if err != nil {
		return 0, false
	}
	var matched []int64
	for _, site := range sites {
		candidates := []string{site.URL, site.LastDeployURL}
		if targets, tErr := ListHomepageManagedDeployTargets(db, site.ID); tErr == nil {
			for _, t := range targets {
				candidates = append(candidates, t.URL)
			}
		}
		for _, c := range candidates {
			if c != "" && homepageNormalizeURL(c) == want {
				matched = append(matched, site.ID)
				break
			}
		}
	}
	if len(matched) != 1 {
		return 0, false
	}
	return matched[0], true
}

// HomepageRegisterPrecheck applies the site identity rules to homepage_registry
// register before RegisterProject runs. A site of the same workspace whose folded
// project_dir equals projectDir is a duplicate (duplicateID > 0); an ancestor or
// descendant of another active local site is rejected with the overlap error.
func HomepageRegisterPrecheck(db *sql.DB, cfg HomepageConfig, projectDir string) (duplicateID int64, err error) {
	dir, err := NormalizeHomepageProjectIdentity(projectDir, false)
	if err != nil || dir == "" || db == nil {
		return 0, err
	}
	sites, err := ListHomepageManagedSites(db)
	if err != nil {
		return 0, err
	}
	for _, site := range sites {
		if site.ProjectDir != "" && homepageSiteInWorkspace(site, cfg) && homepageFoldDir(site.ProjectDir) == homepageFoldDir(dir) {
			return site.ID, nil
		}
	}
	return 0, homepageCheckProjectOverlap(sites, cfg, dir, 0)
}

// WithHomepageOperation lets quick publication events share the correlation id of
// the homepage operation running in cfg.
func (c CloudflareTunnelConfig) WithHomepageOperation(cfg HomepageConfig) CloudflareTunnelConfig {
	c.quickOperation = homepageCurrentOperation(cfg)
	return c
}
