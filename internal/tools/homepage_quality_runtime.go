package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/security"

	"github.com/go-rod/rod/lib/proto"
)

const homepagePlaywrightModule = "/opt/aurago-homepage-tools/node_modules/playwright"

var homepageLaunchCheckBrowser = security.LaunchEgressBrowser

// HomepageCheckJS observes page errors through the public-only browser proxy.
func HomepageCheckJS(ctx context.Context, cfg HomepageConfig, rawURL string, logger *slog.Logger) string {
	if rawURL == "" {
		return ErrorJSON("url is required for check_js")
	}
	if !isValidHomepageURLAllowPrivate(rawURL) {
		return ErrorJSON("invalid URL: must be a valid HTTP(S) URL without shell metacharacters")
	}
	if err := security.ValidatePublicHTTPURL(rawURL); err != nil {
		return ErrorJSONf("check_js requires a public URL: %v", err)
	}
	ctx, cancel := context.WithTimeout(nativeHTTPContext(ctx, cfg.Context), 35*time.Second)
	defer cancel()
	if logger != nil {
		logger.Info("[Homepage] CheckJS", "url", rawURL)
	}
	browser, err := homepageLaunchCheckBrowser(ctx, security.EgressBrowserOptions{Sandbox: true, IgnoreLoopbackOverride: true})
	if err != nil {
		return ErrorJSONf("browser launch failed: %v", err)
	}
	defer browser.Close()
	page, err := browser.Browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return ErrorJSONf("JavaScript check failed: %v", browser.Wrap(err))
	}

	type diagnostic struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	diagnostics := make([]diagnostic, 0, 20)
	errorCount := 0
	record := func(kind, message string) {
		errorCount++
		if len(diagnostics) < cap(diagnostics) {
			diagnostics = append(diagnostics, diagnostic{kind, truncateStr(message, 2048)})
		}
	}
	eventPage, stopEvents := page.WithCancel()
	waitEvents := eventPage.EachEvent(func(event *proto.RuntimeExceptionThrown) {
		message := event.ExceptionDetails.Text
		if exception := event.ExceptionDetails.Exception; exception != nil && exception.Description != "" {
			message = exception.Description
		}
		record("error", message)
	}, func(event *proto.RuntimeConsoleAPICalled) {
		if event.Type == proto.RuntimeConsoleAPICalledTypeError {
			args := make([]string, 0, len(event.Args))
			for _, arg := range event.Args {
				if arg.Description != "" {
					args = append(args, arg.Description)
				} else {
					args = append(args, arg.Value.Str())
				}
			}
			record("console-error", strings.Join(args, " "))
		}
	})
	eventsDone := make(chan struct{})
	go func() {
		defer close(eventsDone)
		waitEvents()
	}()
	if err = page.Navigate(rawURL); err == nil {
		err = page.WaitLoad()
	}
	if err == nil {
		err = waitToolHTTP(ctx, 2*time.Second)
	}
	stopEvents()
	<-eventsDone
	if cause := browser.Err(); cause != nil {
		return ErrorJSONf("JavaScript check was cut off: %v", cause)
	}
	if err != nil {
		return ErrorJSONf("JavaScript check failed: %v", browser.Wrap(err))
	}
	status, exitCode := "ok", 0
	if errorCount > 0 {
		status, exitCode = "error", 1
	}
	out, _ := json.Marshal(map[string]any{
		"status": status, "exit_code": exitCode, "errorCount": errorCount, "errors": diagnostics,
	})
	return string(out)
}

func homepageQualityResult(raw string) string {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil || result == nil {
		return errJSON("Quality check returned an unreadable result")
	}
	if result["status"] == "error" {
		return raw
	}
	code, ok := result["exit_code"].(float64)
	if !ok || code != 0 {
		result["status"] = "error"
		result["message"] = "Quality check failed or did not report an exit code; inspect output and exit_code"
	}
	out, _ := json.Marshal(result)
	return string(out)
}

func homepageLintCommand(projectPath string) string {
	// A pipe to head would replace the check's exit status with head's success.
	return "cd " + shellSingleQuote(projectPath) + ` || exit $?
if [ -f tsconfig.json ]; then
  echo '=== TypeScript Check ==='
  if [ -x ./node_modules/.bin/tsc ]; then
    ./node_modules/.bin/tsc --noEmit 2>&1 || exit $?
  elif command -v tsc >/dev/null 2>&1; then
    tsc --noEmit 2>&1 || exit $?
  else
    echo 'TypeScript is not installed. Install the project dependencies before linting.'
    exit 127
  fi
fi
echo '=== ESLint ==='
if [ -x ./node_modules/.bin/eslint ]; then
  ./node_modules/.bin/eslint . --format stylish 2>&1
elif command -v eslint >/dev/null 2>&1; then
  eslint . --format stylish 2>&1
else
  echo 'ESLint is not installed. Install and configure the project linter before linting.'
  exit 127
fi`
}
