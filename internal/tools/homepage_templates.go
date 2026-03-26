package tools

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ─── Homepage Project Templates ──────────────────────────────────────────
//
// Starter content templates applied after framework scaffolding.
// Each template writes additional files into the project directory.

// homepageTemplateFile represents a file to create in the project.
type homepageTemplateFile struct {
	path    string // relative to project root
	content string
}

// applyHomepageTemplate writes template starter files into the project directory.
// It works both locally (via filesystem) and in Docker (via DockerExec + base64).
// This is best-effort — template application failures don't fail the init.
func applyHomepageTemplate(cfg HomepageConfig, projectName, template string, logger *slog.Logger) {
	files := getTemplateFiles(template, projectName)
	if len(files) == 0 {
		logger.Warn("[Homepage] Unknown template, skipping", "template", template)
		return
	}

	logger.Info("[Homepage] Applying template", "template", template, "files", len(files))

	if !checkDockerAvailable(cfg.DockerHost) {
		// Local mode: write files directly
		if cfg.WorkspacePath == "" {
			return
		}
		for _, f := range files {
			fullPath := filepath.Join(cfg.WorkspacePath, projectName, filepath.FromSlash(f.path))
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				logger.Warn("[Homepage] Template: failed to create dir", "path", f.path, "error", err)
				continue
			}
			if err := os.WriteFile(fullPath, []byte(f.content), 0644); err != nil {
				logger.Warn("[Homepage] Template: failed to write", "path", f.path, "error", err)
			}
		}
		return
	}

	// Docker mode: write files via base64 + DockerExec
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	for _, f := range files {
		encoded := base64.StdEncoding.EncodeToString([]byte(f.content))
		dir := filepath.Dir(f.path)
		cmd := fmt.Sprintf("mkdir -p /workspace/%s/%s && echo '%s' | base64 -d > /workspace/%s/%s",
			projectName, dir, encoded, projectName, f.path)
		DockerExec(dockerCfg, homepageContainerName, cmd, "")
	}
}

// getTemplateFiles returns the starter files for a given template name.
func getTemplateFiles(template, projectName string) []homepageTemplateFile {
	switch strings.ToLower(template) {
	case "portfolio":
		return portfolioTemplate(projectName)
	case "blog":
		return blogTemplate(projectName)
	case "landing":
		return landingTemplate(projectName)
	case "dashboard":
		return dashboardTemplate(projectName)
	default:
		return nil
	}
}

func portfolioTemplate(name string) []homepageTemplateFile {
	return []homepageTemplateFile{
		{path: "src/styles/portfolio.css", content: `/* Portfolio Template */
:root {
  --primary: #2563eb;
  --bg: #0f172a;
  --text: #e2e8f0;
  --card-bg: #1e293b;
}
body { font-family: 'Inter', system-ui, sans-serif; background: var(--bg); color: var(--text); margin: 0; }
.hero { min-height: 80vh; display: flex; align-items: center; justify-content: center; text-align: center; padding: 2rem; }
.hero h1 { font-size: 3.5rem; margin-bottom: 1rem; background: linear-gradient(135deg, var(--primary), #7c3aed); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
.hero p { font-size: 1.25rem; opacity: 0.8; max-width: 600px; margin: 0 auto; }
.projects { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 2rem; padding: 4rem 2rem; max-width: 1200px; margin: 0 auto; }
.project-card { background: var(--card-bg); border-radius: 12px; padding: 2rem; transition: transform 0.2s; }
.project-card:hover { transform: translateY(-4px); }
.project-card h3 { color: var(--primary); margin-top: 0; }
.skills { display: flex; flex-wrap: wrap; gap: 0.5rem; padding: 2rem 2rem 4rem; max-width: 1200px; margin: 0 auto; }
.skill-tag { background: var(--card-bg); padding: 0.5rem 1rem; border-radius: 999px; font-size: 0.875rem; }
.contact { text-align: center; padding: 4rem 2rem; }
.contact a { color: var(--primary); text-decoration: none; font-size: 1.125rem; }
`},
		{path: "TEMPLATE_README.md", content: `# Portfolio Template

This project was initialized with the **portfolio** template.

## Structure
- ` + "`src/styles/portfolio.css`" + ` — Dark-theme portfolio styles with hero, project cards, skills grid, and contact section.

## Sections
1. **Hero** — Full-height intro with gradient heading
2. **Projects** — Responsive grid of project cards
3. **Skills** — Tag-based skill display
4. **Contact** — Simple contact link section

## Customization
- Edit CSS custom properties in ` + "`:root`" + ` to change the color scheme
- Add project cards using the ` + "`.project-card`" + ` class
- Import ` + "`portfolio.css`" + ` in your main page/layout
`},
	}
}

func blogTemplate(name string) []homepageTemplateFile {
	return []homepageTemplateFile{
		{path: "src/styles/blog.css", content: `/* Blog Template */
:root {
  --primary: #059669;
  --bg: #fafafa;
  --text: #1e293b;
  --card-bg: #ffffff;
  --border: #e2e8f0;
}
body { font-family: 'Georgia', serif; background: var(--bg); color: var(--text); margin: 0; line-height: 1.8; }
header { border-bottom: 1px solid var(--border); padding: 1.5rem 2rem; display: flex; justify-content: space-between; align-items: center; max-width: 800px; margin: 0 auto; }
header h1 { font-size: 1.5rem; margin: 0; }
nav a { margin-left: 1.5rem; color: var(--text); text-decoration: none; }
nav a:hover { color: var(--primary); }
main { max-width: 800px; margin: 0 auto; padding: 2rem; }
article { margin-bottom: 3rem; padding-bottom: 2rem; border-bottom: 1px solid var(--border); }
article h2 { font-size: 1.75rem; margin-bottom: 0.5rem; }
article h2 a { color: inherit; text-decoration: none; }
article h2 a:hover { color: var(--primary); }
.meta { color: #64748b; font-size: 0.875rem; margin-bottom: 1rem; }
.tag { display: inline-block; background: var(--primary); color: white; padding: 0.2rem 0.6rem; border-radius: 4px; font-size: 0.75rem; margin-right: 0.5rem; font-family: sans-serif; }
.read-more { color: var(--primary); font-family: sans-serif; font-size: 0.875rem; }
footer { text-align: center; padding: 2rem; color: #94a3b8; font-size: 0.875rem; border-top: 1px solid var(--border); }
`},
		{path: "TEMPLATE_README.md", content: `# Blog Template

This project was initialized with the **blog** template.

## Structure
- ` + "`src/styles/blog.css`" + ` — Clean, readable blog layout with header, article list, metadata, tags, and footer.

## Sections
1. **Header** — Blog title + navigation
2. **Article List** — Post previews with dates, tags, and read-more links
3. **Footer** — Simple copyright

## Customization
- Adjust ` + "`:root`" + ` CSS variables for your brand colors
- Articles use semantic ` + "`<article>`" + ` elements
- Tags use the ` + "`.tag`" + ` class
`},
	}
}

func landingTemplate(name string) []homepageTemplateFile {
	return []homepageTemplateFile{
		{path: "src/styles/landing.css", content: `/* Landing Page Template */
:root {
  --primary: #6366f1;
  --primary-dark: #4f46e5;
  --bg: #ffffff;
  --text: #1e293b;
  --muted: #64748b;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: 'Inter', system-ui, sans-serif; background: var(--bg); color: var(--text); }
.hero-landing { min-height: 90vh; display: flex; align-items: center; justify-content: center; text-align: center; padding: 2rem; background: linear-gradient(135deg, #eef2ff, #faf5ff); }
.hero-landing h1 { font-size: 3rem; font-weight: 800; line-height: 1.2; max-width: 700px; }
.hero-landing p { font-size: 1.25rem; color: var(--muted); max-width: 500px; margin: 1.5rem auto; }
.cta-btn { display: inline-block; background: var(--primary); color: white; padding: 0.875rem 2rem; border-radius: 8px; font-size: 1rem; font-weight: 600; text-decoration: none; transition: background 0.2s; }
.cta-btn:hover { background: var(--primary-dark); }
.features { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 2rem; padding: 5rem 2rem; max-width: 1100px; margin: 0 auto; }
.feature { text-align: center; padding: 2rem; }
.feature-icon { font-size: 2.5rem; margin-bottom: 1rem; }
.feature h3 { font-size: 1.25rem; margin-bottom: 0.5rem; }
.feature p { color: var(--muted); }
.social-proof { background: #f8fafc; padding: 4rem 2rem; text-align: center; }
.social-proof h2 { margin-bottom: 2rem; }
.testimonials { display: flex; gap: 2rem; justify-content: center; flex-wrap: wrap; max-width: 900px; margin: 0 auto; }
.testimonial { background: white; padding: 1.5rem; border-radius: 12px; max-width: 280px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
.cta-section { text-align: center; padding: 5rem 2rem; }
`},
		{path: "TEMPLATE_README.md", content: `# Landing Page Template

This project was initialized with the **landing** template.

## Structure
- ` + "`src/styles/landing.css`" + ` — Marketing landing page with hero, features grid, social proof, and CTA sections.

## Sections
1. **Hero** — Full-height hero with headline, subtext, and CTA button
2. **Features** — 3-column responsive feature grid with icons
3. **Social Proof** — Testimonial cards
4. **CTA Section** — Final call-to-action

## Customization
- Edit ` + "`:root`" + ` variables for brand colors
- Replace feature icons (emoji or SVG)
- Add real testimonials
`},
	}
}

func dashboardTemplate(name string) []homepageTemplateFile {
	return []homepageTemplateFile{
		{path: "src/styles/dashboard.css", content: `/* Dashboard Template */
:root {
  --primary: #3b82f6;
  --sidebar-bg: #111827;
  --sidebar-text: #9ca3af;
  --sidebar-active: #ffffff;
  --content-bg: #f3f4f6;
  --card-bg: #ffffff;
  --text: #1f2937;
  --border: #e5e7eb;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: 'Inter', system-ui, sans-serif; }
.dashboard { display: grid; grid-template-columns: 250px 1fr; min-height: 100vh; }
.sidebar { background: var(--sidebar-bg); color: var(--sidebar-text); padding: 1.5rem; }
.sidebar h2 { color: var(--sidebar-active); font-size: 1.25rem; margin-bottom: 2rem; }
.sidebar nav a { display: block; padding: 0.75rem 1rem; color: var(--sidebar-text); text-decoration: none; border-radius: 8px; margin-bottom: 0.25rem; }
.sidebar nav a:hover, .sidebar nav a.active { background: rgba(255,255,255,0.1); color: var(--sidebar-active); }
.main-content { background: var(--content-bg); padding: 2rem; }
.top-bar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem; }
.top-bar h1 { font-size: 1.5rem; color: var(--text); }
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1.5rem; margin-bottom: 2rem; }
.stat-card { background: var(--card-bg); padding: 1.5rem; border-radius: 12px; box-shadow: 0 1px 3px rgba(0,0,0,0.06); }
.stat-card .label { font-size: 0.875rem; color: #6b7280; }
.stat-card .value { font-size: 2rem; font-weight: 700; color: var(--text); }
.stat-card .change { font-size: 0.875rem; color: #10b981; }
.stat-card .change.negative { color: #ef4444; }
.data-table { width: 100%; background: var(--card-bg); border-radius: 12px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.06); }
.data-table th, .data-table td { padding: 1rem; text-align: left; border-bottom: 1px solid var(--border); }
.data-table th { font-size: 0.875rem; color: #6b7280; font-weight: 600; background: #f9fafb; }
.badge { display: inline-block; padding: 0.25rem 0.75rem; border-radius: 999px; font-size: 0.75rem; font-weight: 600; }
.badge-green { background: #dcfce7; color: #166534; }
.badge-yellow { background: #fef3c7; color: #92400e; }
.badge-red { background: #fef2f2; color: #991b1b; }
@media (max-width: 768px) { .dashboard { grid-template-columns: 1fr; } .sidebar { display: none; } }
`},
		{path: "TEMPLATE_README.md", content: `# Dashboard Template

This project was initialized with the **dashboard** template.

## Structure
- ` + "`src/styles/dashboard.css`" + ` — Admin dashboard layout with sidebar, stats cards, and data table.

## Sections
1. **Sidebar** — Fixed navigation with active state
2. **Top Bar** — Page title and actions
3. **Stats Grid** — KPI cards with values and change indicators
4. **Data Table** — Styled table with status badges

## Customization
- Edit ` + "`:root`" + ` variables for brand colors
- Add navigation items to the sidebar
- Stats cards use ` + "`.stat-card`" + ` with ` + "`.label`" + `, ` + "`.value`" + `, ` + "`.change`" + ` children
- Table uses ` + "`.badge-green`" + `, ` + "`.badge-yellow`" + `, ` + "`.badge-red`" + ` for status
- Responsive: sidebar collapses on mobile
`},
	}
}
