package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopWidgetsAutoSizeByDefault(t *testing.T) {
	source := readDesktopAssetText(t, "js/desktop/main.js")
	renderWidgetsBody := jsFunctionBodyInWindowMenuTest(t, source, "function renderWidgets()")
	for _, marker := range []string{
		"function widgetShouldAutoSize(",
		"function scheduleWidgetAutoSize(",
		"function applyWidgetAutoSize(",
		"function resizeWidgetToContent(",
		"widgetShouldAutoSize(widget)",
		`data-widget-auto-size="true"`,
		"ResizeObserver",
		"--vd-widget-auto-height",
		"--vd-widget-frame-height",
		"WIDGET_AUTO_SIZE_PADDING",
		"WIDGET_FRAME_SCROLLBAR_BUFFER",
		"WIDGET_FRAME_CHROME_BUFFER",
		"WIDGET_WIDTH_GROW_THRESHOLD",
		"function widgetMeasuredContentHeight(",
		"function widgetElementBottom(",
		"function widgetMaxWidth(",
		"function clearWidgetRuntime",
		"state.widgetCleanups",
		"clearInterval",
		"disconnect()",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop widget autosize implementation missing marker %q", marker)
		}
	}
	if strings.Contains(renderWidgetsBody, "height:${bounds.h}px") {
		t.Fatalf("desktop widgets still render stored widget height as a fixed inline height")
	}

	autosizeBody := jsFunctionBodyInWindowMenuTest(t, source, "function applyWidgetAutoSize(card, payload)")
	for _, want := range []string{
		"reportedFrameHeight + WIDGET_FRAME_SCROLLBAR_BUFFER",
		"widgetMeasuredContentHeight(card, data)",
		"reportedFrameHeight > 0 ? 0 : Math.ceil(card.scrollHeight || 0)",
	} {
		if !strings.Contains(autosizeBody, want) {
			t.Fatalf("desktop widget autosize should measure rendered content and leave iframe scrollbar headroom; missing %q", want)
		}
	}

	resizeBody := jsFunctionBodyInWindowMenuTest(t, source, "function resizeWidgetToContent(widgetId, payload)")
	for _, want := range []string{
		"reportedViewportWidth",
		"reportedWidth > reportedViewportWidth + WIDGET_WIDTH_GROW_THRESHOLD",
		"reportedWidth + WIDGET_FRAME_CHROME_BUFFER",
		"widgetMaxWidth(card)",
		"card.style.width",
	} {
		if !strings.Contains(resizeBody, want) {
			t.Fatalf("desktop widget autosize should expand the outer card for iframe chrome; missing %q", want)
		}
	}
}

func TestDesktopWidgetSDKCanReportContentSize(t *testing.T) {
	sdk := readDesktopAssetText(t, "js/desktop/aura-desktop-sdk.js")
	for _, marker := range []string{
		"widgets.resize = options => parentRequest('desktop:widget:resize'",
		"function measureWidgetContentSize(",
		"function startWidgetAutoResize(",
		"body.querySelectorAll('*').forEach(include)",
		"viewportWidth",
		"contentOverflowsViewport",
		"new ResizeObserver",
	} {
		if !strings.Contains(sdk, marker) {
			t.Fatalf("desktop SDK widget autosize support missing marker %q", marker)
		}
	}

	shell := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"case 'desktop:widget:resize':",
		"resizeWidgetToContent(client.widgetId, payload || {})",
	} {
		if !strings.Contains(shell, marker) {
			t.Fatalf("desktop shell widget resize bridge missing marker %q", marker)
		}
	}
}

func TestDesktopWidgetAutoSizeCSSAndManualContract(t *testing.T) {
	css := readAllDesktopCSS(t)
	for _, marker := range []string{
		`.vd-widget[data-widget-auto-size="true"]`,
		"--vd-widget-auto-height",
		"--vd-widget-frame-height",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop widget autosize CSS missing marker %q", marker)
		}
	}

	manual, err := os.ReadFile(filepath.Join("..", "prompts", "tools_manuals", "virtual_desktop.md"))
	if err != nil {
		t.Fatalf("read virtual desktop manual: %v", err)
	}
	if !strings.Contains(string(manual), "AuraDesktop.widgets.resize") {
		t.Fatalf("virtual desktop manual does not document AuraDesktop.widgets.resize for auto-sizing widgets")
	}
}
