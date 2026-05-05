package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/page"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	marotocfg "github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/core/entity"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

// PDFSection describes a content section for programmatic PDF creation with Maroto.
type PDFSection struct {
	Type   string     `json:"type"`   // "text", "heading", "table", "list"
	Header string     `json:"header"` // optional section heading
	Body   string     `json:"body"`   // text content / list items (newline-separated)
	Rows   [][]string `json:"rows"`   // table rows (first row = header if type=="table")
}

// ExecuteDocumentCreator is the unified entry point called from agent dispatch.
func ExecuteDocumentCreator(ctx context.Context, cfg *config.DocumentCreatorConfig, operation, title, content, url, filename, paperSize string, landscape bool, sectionsJSON, sourceFilesJSON string) string {
	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = "data/documents"
	}
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Sprintf(`{"status":"error","message":"create output dir: %v"}`, err)
	}

	backend := strings.ToLower(cfg.Backend)
	if backend == "" {
		backend = "maroto"
	}

	switch operation {
	case "create_pdf":
		return executeCreatePDF(ctx, cfg, backend, outputDir, title, content, filename, paperSize, landscape, sectionsJSON)
	case "url_to_pdf":
		return executeGotenbergOnly(ctx, cfg, backend, func() string {
			if url == "" {
				return `{"status":"error","message":"url is required for url_to_pdf"}`
			}
			return GotenbergURLToPDF(ctx, &cfg.Gotenberg, outputDir, url, filename, paperSize, landscape)
		})
	case "html_to_pdf":
		return executeGotenbergOnly(ctx, cfg, backend, func() string {
			if content == "" {
				return `{"status":"error","message":"content (HTML) is required for html_to_pdf"}`
			}
			return GotenbergHTMLToPDF(ctx, &cfg.Gotenberg, outputDir, content, filename, paperSize, landscape)
		})
	case "markdown_to_pdf":
		return executeGotenbergOnly(ctx, cfg, backend, func() string {
			if content == "" {
				return `{"status":"error","message":"content (Markdown) is required for markdown_to_pdf"}`
			}
			return GotenbergMarkdownToPDF(ctx, &cfg.Gotenberg, outputDir, content, filename, paperSize, landscape)
		})
	case "convert_document":
		return executeGotenbergOnly(ctx, cfg, backend, func() string {
			paths, err := parseSourceFiles(sourceFilesJSON)
			if err != nil {
				return documentCreatorError(err.Error())
			}
			if len(paths) == 0 {
				return `{"status":"error","message":"source_files (JSON array with one file path) is required for convert_document"}`
			}
			return GotenbergConvertDocument(ctx, &cfg.Gotenberg, outputDir, paths[0], filename, paperSize, landscape)
		})
	case "merge_pdfs":
		return executeGotenbergOnly(ctx, cfg, backend, func() string {
			paths, err := parseSourceFiles(sourceFilesJSON)
			if err != nil {
				return documentCreatorError(err.Error())
			}
			if len(paths) < 2 {
				return `{"status":"error","message":"source_files (JSON array with at least 2 PDF paths) is required for merge_pdfs"}`
			}
			return GotenbergMergePDFs(ctx, &cfg.Gotenberg, outputDir, paths, filename)
		})
	case "screenshot_url":
		return executeGotenbergOnly(ctx, cfg, backend, func() string {
			if url == "" {
				return `{"status":"error","message":"url is required for screenshot_url"}`
			}
			return GotenbergScreenshotURL(ctx, &cfg.Gotenberg, outputDir, url, filename)
		})
	case "screenshot_html":
		return executeGotenbergOnly(ctx, cfg, backend, func() string {
			if content == "" {
				return `{"status":"error","message":"content (HTML) is required for screenshot_html"}`
			}
			return GotenbergScreenshotHTML(ctx, &cfg.Gotenberg, outputDir, content, filename)
		})
	case "health":
		return executeGotenbergOnly(ctx, cfg, backend, func() string {
			return GotenbergHealth(ctx, &cfg.Gotenberg)
		})
	default:
		return fmt.Sprintf(`{"status":"error","message":"unknown operation: %s. Valid: create_pdf, url_to_pdf, html_to_pdf, markdown_to_pdf, convert_document, merge_pdfs, screenshot_url, screenshot_html, health"}`, operation)
	}
}

// ExecuteDocumentCreatorInWorkspace validates user-provided local source files
// before invoking document conversion operations.
func ExecuteDocumentCreatorInWorkspace(ctx context.Context, cfg *config.DocumentCreatorConfig, workspaceDir, operation, title, content, url, filename, paperSize string, landscape bool, sectionsJSON, sourceFilesJSON string) string {
	switch operation {
	case "convert_document", "merge_pdfs":
		paths, err := parseSourceFiles(sourceFilesJSON)
		if err != nil {
			return documentCreatorError(err.Error())
		}
		if len(paths) > 0 {
			resolved := make([]string, 0, len(paths))
			tmpCfg := &config.Config{}
			tmpCfg.Directories.WorkspaceDir = workspaceDir
			for _, p := range paths {
				rp, err := resolveToolInputPath(p, tmpCfg)
				if err != nil {
					return fmt.Sprintf(`{"status":"error","message":"invalid source path %q: %v"}`, p, err)
				}
				resolved = append(resolved, rp)
			}
			data, _ := json.Marshal(resolved)
			sourceFilesJSON = string(data)
		}
	}
	return ExecuteDocumentCreator(ctx, cfg, operation, title, content, url, filename, paperSize, landscape, sectionsJSON, sourceFilesJSON)
}

// executeCreatePDF handles the create_pdf operation which supports both backends.
func executeCreatePDF(ctx context.Context, cfg *config.DocumentCreatorConfig, backend, outputDir, title, content, filename, paperSize string, landscape bool, sectionsJSON string) string {
	switch backend {
	case "gotenberg":
		// For Gotenberg, convert content/sections to HTML and use html_to_pdf
		html := buildHTMLFromSections(title, content, sectionsJSON)
		return GotenbergHTMLToPDF(ctx, &cfg.Gotenberg, outputDir, html, filename, paperSize, landscape)
	default:
		// Maroto backend
		return createPDFMaroto(outputDir, title, content, filename, paperSize, landscape, sectionsJSON)
	}
}

// executeGotenbergOnly returns a clear error if the backend is not Gotenberg.
func executeGotenbergOnly(ctx context.Context, cfg *config.DocumentCreatorConfig, backend string, fn func() string) string {
	if backend != "gotenberg" {
		return `{"status":"error","message":"This operation requires the Gotenberg backend. Switch backend to 'gotenberg' in config (tools.document_creator.backend) and ensure the Gotenberg Docker sidecar is running."}`
	}
	return fn()
}

// ── Maroto PDF creation ──────────────────────────────────────────────────────

func createPDFMaroto(outputDir, title, content, filename, paperSize string, landscape bool, sectionsJSON string) string {
	if filename == "" {
		filename = fmt.Sprintf("doc_%d", time.Now().Unix())
	}
	filename = filepath.Base(filename)
	if !strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		filename += ".pdf"
	}

	// Parse sections if provided
	var sections []PDFSection
	if sectionsJSON != "" {
		if err := json.Unmarshal([]byte(sectionsJSON), &sections); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"invalid sections JSON: %v"}`, err)
		}
	}

	// Build Maroto document
	cfg := buildMarotoConfig(paperSize, landscape)
	m := maroto.New(cfg)
	doc, err := buildMarotoDocument(m, title, content, sections)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"generate PDF: %v"}`, err)
	}

	outPath := filepath.Join(outputDir, filename)
	if err := doc.Save(outPath); err != nil {
		return fmt.Sprintf(`{"status":"error","message":"save PDF: %v"}`, err)
	}

	webPath := "/files/documents/" + filename
	return fmt.Sprintf(`{"status":"success","file_path":"%s","web_path":"%s","filename":"%s","backend":"maroto"}`,
		outPath, webPath, filename)
}

func buildMarotoConfig(paperSize string, landscape bool) *entity.Config {
	builder := marotocfg.NewBuilder()

	switch strings.ToUpper(paperSize) {
	case "A3":
		builder.WithPageSize(pagesize.A3)
	case "A5":
		builder.WithPageSize(pagesize.A5)
	case "LETTER":
		builder.WithPageSize(pagesize.Letter)
	case "LEGAL":
		builder.WithPageSize(pagesize.Legal)
	case "TABLOID":
		builder.WithPageSize(pagesize.Tabloid)
	default:
		builder.WithPageSize(pagesize.A4)
	}

	if landscape {
		builder.WithOrientation(orientation.Horizontal)
	}

	builder.WithLeftMargin(15)
	builder.WithTopMargin(15)
	builder.WithRightMargin(15)

	return builder.Build()
}

func buildMarotoDocument(m core.Maroto, title, content string, sections []PDFSection) (core.Document, error) {
	// Title page
	if title != "" {
		m.AddPages(page.New().Add(
			row.New(20).Add(
				text.NewCol(12, title, props.Text{
					Size:  18,
					Style: fontstyle.Bold,
					Align: align.Center,
					Top:   5,
				}),
			),
		))
	}

	// Plain content
	if content != "" && len(sections) == 0 {
		m.AddPages(page.New().Add(
			row.New(0).Add(
				text.NewCol(12, content, props.Text{
					Size: 10,
					Top:  3,
				}),
			),
		))
	}

	// Structured sections
	for _, sec := range sections {
		var rows []core.Row

		if sec.Header != "" {
			rows = append(rows, row.New(10).Add(
				text.NewCol(12, sec.Header, props.Text{
					Size:  14,
					Style: fontstyle.Bold,
					Top:   3,
				}),
			))
		}

		switch sec.Type {
		case "table":
			rows = append(rows, buildTableRows(sec.Rows)...)
		case "list":
			items := strings.Split(sec.Body, "\n")
			for _, item := range items {
				item = strings.TrimSpace(item)
				if item == "" {
					continue
				}
				rows = append(rows, row.New(6).Add(
					text.NewCol(12, "• "+item, props.Text{
						Size: 10,
						Top:  1,
						Left: 5,
					}),
				))
			}
		default: // "text" or "heading"
			bodyText := sec.Body
			if bodyText == "" {
				bodyText = sec.Header
			}
			rows = append(rows, row.New(0).Add(
				text.NewCol(12, bodyText, props.Text{
					Size: 10,
					Top:  2,
				}),
			))
		}

		if len(rows) > 0 {
			m.AddPages(page.New().Add(rows...))
		}
	}

	return m.Generate()
}

func buildTableRows(data [][]string) []core.Row {
	if len(data) == 0 {
		return nil
	}

	var rows []core.Row

	// Header row
	if len(data) > 0 {
		headerCols := make([]core.Col, 0, len(data[0]))
		colSize := 12 / len(data[0])
		if colSize < 1 {
			colSize = 1
		}
		for _, cell := range data[0] {
			headerCols = append(headerCols, text.NewCol(colSize, cell, props.Text{
				Size:  10,
				Style: fontstyle.Bold,
				Top:   2,
			}))
		}
		rows = append(rows, row.New(8).Add(headerCols...))
	}

	// Data rows
	for i := 1; i < len(data); i++ {
		dataCols := make([]core.Col, 0, len(data[i]))
		colSize := 12 / len(data[i])
		if colSize < 1 {
			colSize = 1
		}
		for _, cell := range data[i] {
			dataCols = append(dataCols, text.NewCol(colSize, cell, props.Text{
				Size: 10,
				Top:  2,
			}))
		}
		rows = append(rows, row.New(7).Add(dataCols...))
	}

	return rows
}

// ── HTML builder for Gotenberg create_pdf ────────────────────────────────────

func buildHTMLFromSections(title, content, sectionsJSON string) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 2cm; line-height: 1.6; color: #333; }
h1 { color: #1a1a1a; border-bottom: 2px solid #333; padding-bottom: 10px; }
h2 { color: #2a2a2a; margin-top: 1.5em; }
table { border-collapse: collapse; width: 100%; margin: 1em 0; }
th, td { border: 1px solid #ddd; padding: 8px 12px; text-align: left; }
  th { background: #f5f5f5; font-weight: 600; }
  tr:nth-child(even) { background: #fafafa; }
  ul { padding-left: 1.5em; }
  li { margin: 0.3em 0; }
  .plain-content { white-space: pre-wrap; }
  </style></head><body>`)

	if title != "" {
		sb.WriteString("<h1>")
		sb.WriteString(escapeHTML(title))
		sb.WriteString("</h1>")
	}

	if content != "" {
		sb.WriteString(`<div class="plain-content">`)
		sb.WriteString(escapeHTML(content))
		sb.WriteString("</div>")
	}

	var sections []PDFSection
	if sectionsJSON != "" {
		if err := json.Unmarshal([]byte(sectionsJSON), &sections); err == nil {
			for _, sec := range sections {
				if sec.Header != "" {
					sb.WriteString("<h2>")
					sb.WriteString(escapeHTML(sec.Header))
					sb.WriteString("</h2>")
				}
				switch sec.Type {
				case "table":
					sb.WriteString("<table>")
					for i, r := range sec.Rows {
						sb.WriteString("<tr>")
						tag := "td"
						if i == 0 {
							tag = "th"
						}
						for _, cell := range r {
							sb.WriteString("<" + tag + ">")
							sb.WriteString(escapeHTML(cell))
							sb.WriteString("</" + tag + ">")
						}
						sb.WriteString("</tr>")
					}
					sb.WriteString("</table>")
				case "list":
					sb.WriteString("<ul>")
					for _, item := range strings.Split(sec.Body, "\n") {
						item = strings.TrimSpace(item)
						if item != "" {
							sb.WriteString("<li>")
							sb.WriteString(escapeHTML(item))
							sb.WriteString("</li>")
						}
					}
					sb.WriteString("</ul>")
				default:
					sb.WriteString("<p>")
					sb.WriteString(escapeHTML(sec.Body))
					sb.WriteString("</p>")
				}
			}
		}
	}

	sb.WriteString("</body></html>")
	return sb.String()
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func parseSourceFiles(jsonStr string) ([]string, error) {
	if strings.TrimSpace(jsonStr) == "" {
		return nil, nil
	}
	var paths []string
	if err := json.Unmarshal([]byte(jsonStr), &paths); err != nil {
		return nil, fmt.Errorf("invalid source_files JSON: %w", err)
	}
	return paths, nil
}

func documentCreatorError(message string) string {
	payload, _ := json.Marshal(map[string]string{
		"status":  "error",
		"message": message,
	})
	return string(payload)
}
