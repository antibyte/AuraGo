package ui

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestVercelConfigDeployToggleRequiresExplicitTrue(t *testing.T) {
	src, err := os.ReadFile("cfg/vercel.js")
	if err != nil {
		t.Fatalf("read cfg/vercel.js: %v", err)
	}
	if strings.Contains(string(src), "cfg.allow_deploy !== false") {
		t.Fatal("Vercel deploy toggle must not render active when allow_deploy is missing")
	}
	if !strings.Contains(string(src), "cfg.allow_deploy === true") {
		t.Fatal("Vercel deploy toggle must require allow_deploy === true")
	}
}

func TestVercelConfigTemplateDisablesDeployByDefault(t *testing.T) {
	src, err := os.ReadFile("../config_template.yaml")
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	re := regexp.MustCompile(`(?ms)^vercel:\s.*?^\s+allow_deploy:\s+false\b`)
	if !re.Match(src) {
		t.Fatal("config_template.yaml must set vercel.allow_deploy: false for new installations")
	}
}
