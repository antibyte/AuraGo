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
		"card.dataset.widgetAutoSize = 'true'",
		"ResizeObserver",
		"--vd-widget-auto-height",
		"--vd-widget-frame-height",
		"WIDGET_AUTO_SIZE_PADDING",
		"WIDGET_FRAME_SCROLLBAR_BUFFER",
		"WIDGET_FRAME_CHROME_BUFFER",
		"WIDGET_WIDTH_GROW_THRESHOLD",
		"WIDGET_AUTO_WIDTH_MAX",
		"function widgetMeasuredContentHeight(",
		"function widgetElementBottom(",
		"function widgetMaxWidth(",
		"function widgetPreferredWidth(",
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

	runtime := readDesktopAssetText(t, "js/desktop/core/widget-autosize-runtime.js")
	for _, marker := range []string{
		"WIDGET_FRAME_SHRINK_THRESHOLD",
		"function stableWidgetFrameHeight(",
	} {
		if !strings.Contains(runtime, marker) {
			t.Fatalf("desktop widget runtime should keep small iframe height shrink corrections stable; missing %q", marker)
		}
	}

	autosizeBody := jsFunctionBodyInWindowMenuTest(t, runtime, "function applyWidgetAutoSize(card, payload)")
	for _, want := range []string{
		"reportedFrameHeight + WIDGET_FRAME_SCROLLBAR_BUFFER",
		"stableWidgetFrameHeight(card, frameHeight)",
		"widgetMeasuredContentHeight(card, data)",
		"reportedFrameHeight > 0 ? 0 : Math.ceil(card.scrollHeight || 0)",
		"setWidgetPixelVar(card, '--vd-widget-auto-height'",
	} {
		if !strings.Contains(autosizeBody, want) {
			t.Fatalf("desktop widget autosize should measure rendered content and leave iframe scrollbar headroom; missing %q", want)
		}
	}

	scheduleBody := jsFunctionBodyInWindowMenuTest(t, runtime, "function scheduleWidgetAutoSize(card, widget)")
	for _, want := range []string{
		"applyWidgetAutoSize(card, card._widgetLastResizePayload || {})",
		"const isFrameWidget = !!card.querySelector('.vd-widget-frame-wrap')",
		"if (!isFrameWidget) observer.observe(card)",
	} {
		if !strings.Contains(scheduleBody, want) {
			t.Fatalf("desktop widget autosize should avoid iframe resize observer feedback loops; missing %q", want)
		}
	}

	resizeBody := jsFunctionBodyInWindowMenuTest(t, source, "function resizeWidgetToContent(widgetId, payload)")
	for _, want := range []string{
		"card._widgetLastResizePayload = data",
		"reportedViewportWidth",
		"reportedWidth > reportedViewportWidth + WIDGET_WIDTH_GROW_THRESHOLD",
		"widgetPreferredWidth(card)",
		"setWidgetWidthIfChanged(card, nextWidth)",
		"reportedWidth + WIDGET_FRAME_CHROME_BUFFER",
		"widgetMaxWidth(card)",
	} {
		if !strings.Contains(resizeBody, want) {
			t.Fatalf("desktop widget autosize should expand the outer card for iframe chrome; missing %q", want)
		}
	}

	persistBody := jsFunctionBodyInWindowMenuTest(t, source, "async function persistWidgetBounds(widget, card)")
	for _, want := range []string{
		"if (widgetShouldAutoSize(widget))",
		"delete updated.w",
		"delete updated.h",
	} {
		if !strings.Contains(persistBody, want) {
			t.Fatalf("desktop widget drag persistence should not save autosized dimensions; missing %q", want)
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
		"lastWidgetResizePayload",
		"lastWidgetResizePostAt",
		"shouldSendWidgetResize",
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
		"transition: none",
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
