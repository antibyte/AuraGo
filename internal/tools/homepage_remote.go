package tools

import (
	"aurago/internal/security"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func homepageDeploymentTargetKey(rec HomepageDeploymentRecord) string {
	identity := rec.ProviderTargetID
	if identity == "" {
		identity = rec.URL
		if u, err := url.Parse(rec.URL); err == nil && u.Host != "" {
			identity = u.Scheme + "://" + u.Host
		}
		identity += "|" + rec.RemotePath
	}
	account := firstLedgerString(rec.Metadata, "account_id", "account", "team_id")
	environment := firstLedgerString(rec.Metadata, "target", "environment")
	if environment == "" && rec.Provider == "vercel" {
		environment = "preview"
	}
	encoded, _ := json.Marshal([]string{identity, account, environment})
	return hashContent(encoded)
}

// Public provider URLs never inherit the homelab loopback override.
var homepagePublicProbeClient = func(ctx context.Context, raw string) (*http.Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return security.NewStrictPublicHTTPClientForURL(raw, 12*time.Second)
}

func homepageRemoteProbeClient(ctx context.Context, provider, raw string) (*http.Client, error) {
	switch provider {
	case "local", "caddy", "sftp":
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, fmt.Errorf("invalid configured homepage origin")
		}
		return homepageConfiguredProbeClient(raw)

	default:
		return homepagePublicProbeClient(ctx, raw)
	}
}

var homepageConfiguredProbeClient = func(raw string) (*http.Client, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid configured homepage origin")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Transport: transport, Timeout: 12 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != u.Scheme || !strings.EqualFold(req.URL.Host, u.Host) {
			return fmt.Errorf("homepage redirect leaves configured origin")
		}
		return nil
	}}, nil
}

// These are also used by automatic server/config callers, not just the agent dispatcher.
func HomepageManagedWebServerStart(cfg HomepageConfig, db *sql.DB, projectDir, buildDir, reason string, logger *slog.Logger) string {
	return ExecuteHomepageOperation(cfg, db, HomepageOperation{Name: "webserver_start", ProjectDir: projectDir, Reason: reason, Actor: "system", Source: "homepage_server", FilesMayChange: true}, logger, func(c HomepageConfig) string {
		dir, build := projectDir, buildDir
		if dir == "" {
			dir = c.ManagedProjectDir
			if saved, savedBuild := loadWebserverState(c.WorkspacePath); saved == dir && build == "" {
				build = savedBuild
			}
		}
		raw := HomepageWebServerStart(c, dir, build, logger)
		parsed, outcome := homepageOperationOutcome(raw)
		if outcome != "succeeded" {
			return raw
		}
		dir = firstLedgerString(parsed, "project_dir")
		if dir == "" {
			dir = projectDir
		}
		if dir == "" {
			dir, _ = findServableProject(c.WorkspacePath)
		}
		if dir != "" {
			warnings, err := RecordHomepageDeploymentFromResultStrict(c, db, dir, "caddy", buildDir, raw, logger)
			if err != nil {
				return homepageOperationPersistenceError(parsed, homepageCurrentOperation(c), err)
			}
			if len(warnings) > 0 {
				parsed["ledger_warnings"] = warnings
				encoded, _ := json.Marshal(parsed)
				return string(encoded)
			}
		}
		return raw
	})
}
func HomepageManagedWebServerStop(cfg HomepageConfig, db *sql.DB, reason string, logger *slog.Logger) string {
	return ExecuteHomepageOperation(cfg, db, HomepageOperation{Name: "webserver_stop", Reason: reason, Actor: "system", Source: "homepage_server"}, logger, func(c HomepageConfig) string { return HomepageWebServerStop(c, logger) })
}
