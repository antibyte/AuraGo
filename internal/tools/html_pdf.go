package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"

	"github.com/go-rod/rod/lib/proto"
)

// ErrHTMLPDFUnavailable means that no renderer produced a PDF: Gotenberg is not the document backend or failed,
// and the built-in browser could not render either.
var ErrHTMLPDFUnavailable = errors.New("no PDF renderer is available")

// htmlPDFMaxBytes bounds a rendered PDF.
const htmlPDFMaxBytes = 64 << 20

// The seams let unit tests choose the renderers. htmlPDFBrowserBin is for tests only; empty uses rod's lookup.
var (
	htmlPDFGotenberg  = RenderReportHTMLPDF
	htmlPDFChromium   = renderHTMLPDFChromium
	htmlPDFBrowserBin string
)

// RenderHTMLPDF renders one self-contained HTML page to a PDF whose sheet sizes come from the page's own @page
// rules. The page is data: both renderers get it behind restrictRemoteContent's policy, so it runs no script and
// loads nothing that is not inline. Gotenberg renders when it is the document backend; the built-in browser
// renders otherwise, and when Gotenberg fails.
func RenderHTMLPDF(ctx context.Context, cfg *config.Config, page string) ([]byte, error) {
	var failures []string
	if cfg != nil && cfg.Tools.DocumentCreator.Enabled && cfg.Tools.DocumentCreator.Backend == "gotenberg" {
		data, err := htmlPDFGotenberg(ctx, &cfg.Tools.DocumentCreator.Gotenberg, page)
		if err == nil {
			return data, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		failures = append(failures, "gotenberg: "+err.Error())
	}
	data, err := htmlPDFChromium(ctx, restrictRemoteContent(page))
	if err == nil {
		return data, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	failures = append(failures, "browser: "+err.Error())
	return nil, fmt.Errorf("%w (%s)", ErrHTMLPDFUnavailable, strings.Join(failures, "; "))
}

// renderHTMLPDFChromium prints the page in a fresh egress browser. The page comes in as document content, never as
// a URL, and scripts stay off even where the policy would not reach.
func renderHTMLPDFChromium(ctx context.Context, page string) ([]byte, error) {
	eb, err := security.LaunchEgressBrowser(ctx, security.EgressBrowserOptions{Sandbox: true, Bin: htmlPDFBrowserBin})
	if err != nil {
		return nil, fmt.Errorf("browser launch failed: %w", err)
	}
	defer func() { _ = eb.Close() }()
	target, err := eb.Browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, eb.Wrap(err)
	}
	target = target.Context(ctx)
	if err := (proto.EmulationSetScriptExecutionDisabled{Value: true}).Call(target); err != nil {
		return nil, eb.Wrap(err)
	}
	if err := target.SetDocumentContent(page); err != nil {
		return nil, eb.Wrap(err)
	}
	// Fonts and pictures are inline, but decoding them takes a moment.
	if _, err := target.Eval(`async () => { await document.fonts.ready; await Promise.all([...document.images].map(image => image.decode().catch(() => {}))); }`); err != nil {
		return nil, eb.Wrap(err)
	}
	stream, err := target.PDF(&proto.PagePrintToPDF{PrintBackground: true, PreferCSSPageSize: true})
	if err != nil {
		return nil, eb.Wrap(err)
	}
	data, err := io.ReadAll(io.LimitReader(stream, htmlPDFMaxBytes+1))
	if err != nil {
		return nil, eb.Wrap(err)
	}
	if cause := eb.Err(); cause != nil {
		return nil, cause
	}
	if len(data) > htmlPDFMaxBytes {
		return nil, fmt.Errorf("PDF exceeds %d MiB", htmlPDFMaxBytes>>20)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, errors.New("the browser returned a non-PDF response")
	}
	return data, nil
}
