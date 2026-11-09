package tools

import "encoding/json"

const homepagePlaywrightModule = "/opt/aurago-homepage-tools/node_modules/playwright"

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
