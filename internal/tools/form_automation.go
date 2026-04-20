// Package tools – form_automation: rod-based web form fill and submit.
package tools

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// formResult is the JSON payload returned by ExecuteFormAutomation.
type formResult struct {
	Status     string            `json:"status"`
	Operation  string            `json:"operation"`
	URL        string            `json:"url,omitempty"`
	Fields     []formField       `json:"fields,omitempty"`
	Filled     map[string]string `json:"filled,omitempty"`
	Screenshot string            `json:"screenshot,omitempty"`
	Message    string            `json:"message,omitempty"`
}

// formField represents a discovered form input on a page.
type formField struct {
	Selector    string `json:"selector"`
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Value       string `json:"value,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

func formJSON(r formResult) string {
	b, err := json.Marshal(r)
	if err != nil {
		return `{"status":"error","message":"failed to encode result"}`
	}
	return string(b)
}

// ExecuteFormAutomation interacts with web forms using a headless Chromium browser.
//
// operation     – "fill_submit", "click", "get_fields"
// rawURL        – page URL to load
// fieldsJSON    – JSON map of {CSS_selector: value} for fill_submit (e.g. '{"#username":"alice","#password":"secret"}')
// selector      – CSS selector for click, or submit button for fill_submit (default: first submit button)
// screenshotDir – directory to save post-action screenshot (optional; "" = no screenshot)
func ExecuteFormAutomation(operation, rawURL, fieldsJSON, selector, screenshotDir string) string {
	operation = strings.ToLower(strings.TrimSpace(operation))

	// Validate URL
	if rawURL == "" {
		return formJSON(formResult{Status: "error", Message: "url is required"})
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return formJSON(formResult{Status: "error", Message: "url must be a valid http or https URL"})
	}

	switch operation {
	case "fill_submit", "click", "get_fields":
		// valid
	default:
		return formJSON(formResult{Status: "error", Message: "operation must be 'fill_submit', 'click', or 'get_fields'"})
	}

	// Launch headless browser
	u, launchErr := launcher.New().
		Headless(true).
		NoSandbox(true).
		Launch()
	if launchErr != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("browser launch failed: %v", launchErr)})
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("browser connect failed: %v", err)})
	}
	defer func() {
		_ = browser.Close()
	}()

	page, err := browser.Page(proto.TargetCreateTarget{URL: rawURL})
	if err != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("open page failed: %v", err)})
	}
	if err := page.WaitLoad(); err != nil {
		_ = err // non-fatal
	}

	switch operation {
	case "get_fields":
		return formGetFields(page, rawURL)
	case "click":
		return formClick(page, rawURL, selector, screenshotDir)
	case "fill_submit":
		return formFillSubmit(page, rawURL, fieldsJSON, selector, screenshotDir)
	}
	return formJSON(formResult{Status: "error", Message: "unhandled operation"})
}

// formGetFields lists all input/textarea/select elements on the page.
func formGetFields(page *rod.Page, rawURL string) string {
	// Evaluate JS to collect all form field info
	res, err := page.Eval(`() => {
		const inputs = Array.from(document.querySelectorAll('input, textarea, select, button[type="submit"]'));
		return inputs.map(el => ({
			tag:         el.tagName.toLowerCase(),
			type:        el.type || el.tagName.toLowerCase(),
			name:        el.name || '',
			id:          el.id || '',
			placeholder: el.placeholder || '',
			value:       el.value || '',
			required:    el.required || false,
			selector:    el.id ? '#' + el.id : (el.name ? '[name="' + el.name + '"]' : el.tagName.toLowerCase())
		}));
	}`)
	if err != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("field extraction failed: %v", err)})
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(res.Value.String()), &raw); err != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("parse fields failed: %v", err)})
	}

	fields := make([]formField, 0, len(raw))
	for _, f := range raw {
		ff := formField{
			Selector:    fmt.Sprint(f["selector"]),
			Type:        fmt.Sprint(f["type"]),
			Name:        fmt.Sprint(f["name"]),
			Placeholder: fmt.Sprint(f["placeholder"]),
			Value:       fmt.Sprint(f["value"]),
		}
		if req, ok := f["required"].(bool); ok {
			ff.Required = req
		}
		fields = append(fields, ff)
	}
	return formJSON(formResult{Status: "success", Operation: "get_fields", URL: rawURL, Fields: fields})
}

// formClick clicks a CSS-selected element on the page.
func formClick(page *rod.Page, rawURL, selector, screenshotDir string) string {
	if selector == "" {
		return formJSON(formResult{Status: "error", Message: "selector is required for click operation"})
	}
	el, err := page.Element(selector)
	if err != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("element not found (%s): %v", selector, err)})
	}
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("click failed: %v", err)})
	}
	// Wait briefly for navigation/update
	time.Sleep(500 * time.Millisecond)

	screenshot := takeFormScreenshot(page, screenshotDir, rawURL, "click")
	return formJSON(formResult{
		Status:     "success",
		Operation:  "click",
		URL:        rawURL,
		Screenshot: screenshot,
		Message:    fmt.Sprintf("Clicked element: %s", selector),
	})
}

// formFillSubmit fills form fields and optionally submits.
func formFillSubmit(page *rod.Page, rawURL, fieldsJSON, submitSelector, screenshotDir string) string {
	if fieldsJSON == "" {
		return formJSON(formResult{Status: "error", Message: "fields JSON is required for fill_submit operation"})
	}

	var fields map[string]string
	if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("invalid fields JSON: %v", err)})
	}

	filled := make(map[string]string, len(fields))
	for sel, value := range fields {
		el, err := page.Element(sel)
		if err != nil {
			return formJSON(formResult{Status: "error", Message: fmt.Sprintf("field not found (%s): %v", sel, err)})
		}
		if err := el.Input(value); err != nil {
			return formJSON(formResult{Status: "error", Message: fmt.Sprintf("fill field (%s) failed: %v", sel, err)})
		}
		filled[sel] = value
	}

	// Submit: use provided selector, or find first submit button
	if submitSelector == "" {
		submitSelector = `[type="submit"], button[type="submit"], input[type="submit"]`
	}
	submitEl, err := page.Element(submitSelector)
	if err != nil {
		// No submit element found — just return filled state
		return formJSON(formResult{
			Status:    "success",
			Operation: "fill_submit",
			URL:       rawURL,
			Filled:    filled,
			Message:   "Fields filled; no submit element found — not submitted",
		})
	}
	if err := submitEl.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("submit click failed: %v", err)})
	}
	// Wait for navigation
	_ = page.WaitLoad()

	screenshot := takeFormScreenshot(page, screenshotDir, rawURL, "fill_submit")
	return formJSON(formResult{
		Status:     "success",
		Operation:  "fill_submit",
		URL:        rawURL,
		Filled:     filled,
		Screenshot: screenshot,
		Message:    fmt.Sprintf("Filled %d field(s) and submitted", len(filled)),
	})
}

// takeFormScreenshot captures a PNG of the page and returns the file path.
// Returns "" if screenshotDir is empty or the screenshot fails.
func takeFormScreenshot(page *rod.Page, screenshotDir, rawURL, label string) string {
	if screenshotDir == "" {
		return ""
	}
	if err := os.MkdirAll(screenshotDir, 0o750); err != nil {
		return ""
	}
	parsed, _ := url.ParseRequestURI(rawURL)
	host := ""
	if parsed != nil {
		host = parsed.Hostname()
	}
	ts := time.Now().Format("20060102_150405")
	filename := filepath.Join(screenshotDir, fmt.Sprintf("form_%s_%s_%s.png", label, host, ts))
	quality := 90
	data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format:  proto.PageCaptureScreenshotFormatPng,
		Quality: &quality,
	})
	if err != nil {
		return ""
	}
	if err := os.WriteFile(filename, data, 0o640); err != nil {
		return ""
	}
	return filename
}
