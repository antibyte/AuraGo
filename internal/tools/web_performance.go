package tools

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// PerfMetrics holds Core Web Vitals and related performance data.
type PerfMetrics struct {
	URL               string           `json:"url"`
	TTFB              float64          `json:"ttfb_ms"`
	FCP               float64          `json:"fcp_ms"`
	DOMContentLoaded  float64          `json:"dom_content_loaded_ms"`
	LoadComplete      float64          `json:"load_complete_ms"`
	DOMInteractive    float64          `json:"dom_interactive_ms"`
	ResourceCount     int              `json:"resource_count"`
	TransferSizeBytes int              `json:"transfer_size_bytes"`
	DOMElements       int              `json:"dom_elements"`
	JSHeapUsedBytes   int              `json:"js_heap_used_bytes,omitempty"`
	JSHeapTotalBytes  int              `json:"js_heap_total_bytes,omitempty"`
	LargestResources  []ResourceTiming `json:"largest_resources,omitempty"`
}

// ResourceTiming holds timing for a single resource.
type ResourceTiming struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	DurationMs   float64 `json:"duration_ms"`
	TransferSize int     `json:"transfer_size_bytes"`
}

type webPerfResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func webPerfJSON(r webPerfResult) string {
	b, _ := json.Marshal(r)
	return string(b)
}

// WebPerformanceAudit measures page load performance using a headless browser.
func WebPerformanceAudit(rawURL string, viewport string) string {
	// Validate URL
	if rawURL == "" {
		return webPerfJSON(webPerfResult{Status: "error", Message: "url is required"})
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return webPerfJSON(webPerfResult{Status: "error", Message: "url must be a valid http or https URL"})
	}

	// Parse viewport
	vpWidth, vpHeight := 1280, 720
	if viewport != "" {
		if n, _ := fmt.Sscanf(viewport, "%dx%d", &vpWidth, &vpHeight); n != 2 {
			return webPerfJSON(webPerfResult{Status: "error", Message: "viewport must be in format 'WIDTHxHEIGHT' (e.g. '1920x1080')"})
		}
		if vpWidth < 320 || vpWidth > 3840 || vpHeight < 240 || vpHeight > 2160 {
			return webPerfJSON(webPerfResult{Status: "error", Message: "viewport dimensions must be between 320x240 and 3840x2160"})
		}
	}

	// Launch headless browser
	u, err := launcher.New().
		Headless(true).
		NoSandbox(true).
		Launch()
	if err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("browser launch failed: %v", err)})
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("browser connect failed: %v", err)})
	}
	defer browser.MustClose()

	// Set viewport
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("create page failed: %v", err)})
	}

	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  vpWidth,
		Height: vpHeight,
	})
	if err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("set viewport failed: %v", err)})
	}

	// Navigate and wait
	if err := page.Navigate(rawURL); err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("navigation failed: %v", err)})
	}

	// Wait for page load with timeout
	done := make(chan error, 1)
	go func() {
		done <- page.WaitLoad()
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		// Continue with partial results
	}

	// Allow a moment for late resources
	time.Sleep(500 * time.Millisecond)

	// Collect performance metrics via JavaScript
	jsCode := `() => {
		const result = {};
		
		// Navigation Timing
		const nav = performance.getEntriesByType('navigation');
		if (nav.length > 0) {
			const n = nav[0];
			result.ttfb = n.responseStart - n.startTime;
			result.dom_content_loaded = n.domContentLoadedEventEnd - n.startTime;
			result.load_complete = n.loadEventEnd - n.startTime;
			result.dom_interactive = n.domInteractive - n.startTime;
		}
		
		// First Contentful Paint
		const paintEntries = performance.getEntriesByType('paint');
		for (const p of paintEntries) {
			if (p.name === 'first-contentful-paint') {
				result.fcp = p.startTime;
			}
		}
		
		// Resource summary
		const resources = performance.getEntriesByType('resource');
		result.resource_count = resources.length;
		result.transfer_size = resources.reduce((sum, r) => sum + (r.transferSize || 0), 0);
		
		// Top 5 largest resources by transfer size
		const sorted = [...resources]
			.filter(r => r.transferSize > 0)
			.sort((a, b) => b.transferSize - a.transferSize)
			.slice(0, 5);
		result.largest_resources = sorted.map(r => ({
			name: r.name.length > 120 ? r.name.substring(0, 120) + '...' : r.name,
			type: r.initiatorType,
			duration: r.duration,
			transfer_size: r.transferSize
		}));
		
		// DOM element count
		result.dom_elements = document.getElementsByTagName('*').length;
		
		// JS Heap (Chrome only)
		if (performance.memory) {
			result.js_heap_used = performance.memory.usedJSHeapSize;
			result.js_heap_total = performance.memory.totalJSHeapSize;
		}
		
		return result;
	}`

	evalResult, err := page.Eval(jsCode)
	if err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("failed to collect metrics: %v", err)})
	}

	// Parse the result via gson's JSON helper
	jsData := evalResult.Value.Map()

	metrics := PerfMetrics{URL: rawURL}

	// Extract numeric values safely via gson
	getFloat := func(key string) float64 {
		if v, ok := jsData[key]; ok {
			if f, ok := v.Raw().(float64); ok {
				return f
			}
		}
		return 0
	}
	getInt := func(key string) int {
		return int(getFloat(key))
	}

	metrics.TTFB = getFloat("ttfb")
	metrics.FCP = getFloat("fcp")
	metrics.DOMContentLoaded = getFloat("dom_content_loaded")
	metrics.LoadComplete = getFloat("load_complete")
	metrics.DOMInteractive = getFloat("dom_interactive")
	metrics.ResourceCount = getInt("resource_count")
	metrics.TransferSizeBytes = getInt("transfer_size")
	metrics.DOMElements = getInt("dom_elements")
	metrics.JSHeapUsedBytes = getInt("js_heap_used")
	metrics.JSHeapTotalBytes = getInt("js_heap_total")

	// Parse largest resources
	if lr, ok := jsData["largest_resources"]; ok {
		for _, item := range lr.Arr() {
			m := item.Map()
			rt := ResourceTiming{}
			if v, ok := m["name"]; ok {
				rt.Name = v.Str()
			}
			if v, ok := m["type"]; ok {
				rt.Type = v.Str()
			}
			if v, ok := m["duration"]; ok {
				if f, ok := v.Raw().(float64); ok {
					rt.DurationMs = f
				}
			}
			if v, ok := m["transfer_size"]; ok {
				if f, ok := v.Raw().(float64); ok {
					rt.TransferSize = int(f)
				}
			}
			metrics.LargestResources = append(metrics.LargestResources, rt)
		}
	}

	// Build summary message
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Performance audit for %s: ", parsed.Hostname()))
	if metrics.TTFB > 0 {
		summary.WriteString(fmt.Sprintf("TTFB=%.0fms ", metrics.TTFB))
	}
	if metrics.FCP > 0 {
		summary.WriteString(fmt.Sprintf("FCP=%.0fms ", metrics.FCP))
	}
	if metrics.LoadComplete > 0 {
		summary.WriteString(fmt.Sprintf("Load=%.0fms ", metrics.LoadComplete))
	}
	summary.WriteString(fmt.Sprintf("Resources=%d DOM=%d", metrics.ResourceCount, metrics.DOMElements))

	return webPerfJSON(webPerfResult{
		Status:  "success",
		Message: summary.String(),
		Data:    metrics,
	})
}
