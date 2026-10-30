package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func wave0Read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// Config modules are classic scripts sharing one global scope, so a top-level
// let/const/class (and function) declared in two modules breaks the second one.
func TestConfigModulesDeclareNoDuplicateTopLevelNames(t *testing.T) {
	files, err := filepath.Glob("cfg/*.js")
	if err != nil || len(files) == 0 {
		t.Fatalf("no cfg modules found: %v", err)
	}
	decl := regexp.MustCompile(`^(?:async\s+)?(let|const|var|class|function\*?)\s+([A-Za-z_$][\w$]*)`)
	// Pre-existing duplicate function names that are tolerated (none at the time of writing).
	allowedFunctions := map[string]bool{}
	seen := map[string]string{}
	for _, file := range files {
		for _, line := range strings.Split(wave0Read(t, file), "\n") {
			m := decl.FindStringSubmatch(strings.TrimRight(line, "\r"))
			if m == nil {
				continue
			}
			kind, name := m[1], m[2]
			if prev, ok := seen[name]; ok && prev != file {
				if strings.HasPrefix(kind, "function") && allowedFunctions[name] {
					continue
				}
				t.Errorf("top-level %s %q declared in both %s and %s", kind, name, prev, file)
				continue
			}
			seen[name] = file
		}
	}
}

func TestConfigWave0RenderSectionDoesNotFallBackWhenRendererMissing(t *testing.T) {
	main := wave0Read(t, "js/config/main.js")
	if !strings.Contains(main, "typeof fn === 'function'") || !strings.Contains(main, "is missing from module") {
		t.Fatal("renderSection must report a missing module renderer instead of using the generic renderer")
	}
	for _, key := range []string{"'allow_unsafe_host_execution'", "'action_policy'"} {
		if !strings.Contains(main, key+",") {
			t.Fatalf("AGENT_SKIP_KEYS must contain %s", key)
		}
	}
	if !strings.Contains(main, "CFG_OBJECT_ARRAY_PATHS") || !strings.Contains(main, "'agent.action_policy.allow'") {
		t.Fatal("empty object arrays must render as JSON textareas")
	}
	if !strings.Contains(main, "'.read_only'") {
		t.Fatal("readonly toggles must fall back to the read_only help key")
	}
}

func TestConfigWave0ToastAndAnsibleAction(t *testing.T) {
	core := wave0Read(t, "js/shared/shared-core.js")
	if !strings.Contains(core, "type === 'warn'") || !strings.Contains(core, "6000") {
		t.Fatal("showToast must normalise warn and keep error toasts longer")
	}
	actions := wave0Read(t, "js/config/integration_actions.js")
	if !strings.Contains(actions, "ansible.mode") || !strings.Contains(actions, "credentialReady") {
		t.Fatal("the Ansible test must only require url/token outside local mode")
	}
}

func TestConfigWave0LocaleKeys(t *testing.T) {
	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"config.precision.validation_number_required", "config.precision.validation_number_invalid",
		"config.precision.validation_integer_required", "config.precision.validation_json_invalid",
	}
	for _, lang := range langs {
		text := wave0Read(t, "lang/config/common/"+lang+".json")
		for _, key := range keys {
			if !strings.Contains(text, `"`+key+`"`) {
				t.Errorf("%s missing %s", lang, key)
			}
		}
		if strings.Contains(text, "config.field.json_array_hint") && lang == "en" && strings.Contains(text, "comma-separated list of values.\"") {
			t.Error("json_array_hint must not promise comma lists")
		}
	}
}

const wave0StateHarness = `
const fs = require('fs');
const vm = require('vm');
function el(attrs) {
  return Object.assign({ dataset: {}, classList: { contains: () => false }, step: '', type: 'number', tagName: 'INPUT' }, attrs);
}
const win = { };
const ctx = vm.createContext({ window: win, document: { dispatchEvent() {} }, CustomEvent: function () {}, console });
vm.runInContext(fs.readFileSync(process.argv[2], 'utf8'), ctx);
const S = win.AuraConfigState;
function run(elements, config) {
  S.init(config);
  S.syncFromDOM({ querySelectorAll: () => elements });
  return S.validate();
}
const out = {};
let v = run([el({ dataset: { path: 'a.n' }, value: '' })], { a: { n: 5 } });
out.emptyInvalid = !v.valid && v.errors[0].code === 'number_required';
v = run([el({ dataset: { path: 'a.n' }, value: '' })], { a: {} });
out.emptyUnsetOk = v.valid && S.get('a.n') === undefined;
v = run([el({ dataset: { path: 'a.n' }, value: '2.7', step: '1' })], { a: { n: 5 } });
out.fractionInvalid = !v.valid && v.errors[0].code === 'integer_required';
v = run([el({ dataset: { path: 'a.n' }, value: '0', step: '1' })], { a: { n: 5 } });
out.zeroOk = v.valid && S.get('a.n') === 0;
v = run([el({ dataset: { path: 'a.f' }, value: '0.25', step: '0.01' })], { a: { f: 1 } });
out.floatOk = v.valid && S.get('a.f') === 0.25;
v = run([el({ dataset: { path: 'a.j', type: 'json' }, type: 'textarea', value: '[{"x": 1' })], { a: { j: [] } });
out.badJsonInvalid = !v.valid && v.errors[0].code === 'json_invalid' && typeof S.get('a.j') === 'string';
v = run([el({ dataset: { path: 'a.j', type: 'json' }, type: 'textarea', value: '[{"x": 1}]' })], { a: { j: [] } });
out.goodJsonOk = v.valid && Array.isArray(S.get('a.j'));
console.log(JSON.stringify(out));
`

func TestConfigWave0StateNumberAndJSONValidation(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(script, []byte(wave0StateHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	state, _ := filepath.Abs("js/config/state.js")
	out, err := exec.Command(node, script, state).CombinedOutput()
	if err != nil {
		t.Fatalf("node harness failed: %v\n%s", err, out)
	}
	for _, name := range []string{"emptyInvalid", "emptyUnsetOk", "fractionInvalid", "zeroOk", "floatOk", "badJsonInvalid", "goodJsonOk"} {
		if !strings.Contains(string(out), `"`+name+`":true`) {
			t.Errorf("%s failed: %s", name, out)
		}
	}
}
