package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCrawlerRejectsPrivateStartURL(t *testing.T) {
	raw := ExecuteCrawler("http://169.254.169.254/latest/meta-data", 1, 1, "", "")
	if !strings.Contains(raw, "SSRF protection") {
		t.Fatalf("crawler result = %s, want SSRF rejection", raw)
	}
}

func TestFormAutomationRejectsPrivateURLBeforeBrowserLaunch(t *testing.T) {
	raw := ExecuteFormAutomation("get_fields", "http://127.0.0.1:1/login", "", "", "")
	var result formResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Status != "error" || !strings.Contains(result.Message, "SSRF protection") {
		t.Fatalf("form result = %+v, want SSRF rejection", result)
	}
}

func TestChromecastMediaURLRejectsPrivateURL(t *testing.T) {
	err := validateChromecastMediaURL("http://169.254.169.254/latest/meta-data")
	if err == nil || !strings.Contains(err.Error(), "SSRF protection") {
		t.Fatalf("validateChromecastMediaURL error = %v, want SSRF rejection", err)
	}
}
