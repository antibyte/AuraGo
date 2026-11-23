package chibisensei

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"

	"aurago/internal/chibisensei/trainer"
)

// jobTopLevelKeys is the allow-list of job.json keys (schema v1).
var jobTopLevelKeys = []string{"v", "run_id", "created", "aurago_version", "profile", "profile_sha", "recipe",
	"task_kind", "labels", "method", "data", "hyper", "budget", "outputs", "deps", "log"}

// allowedURLHosts are the only hosts the trainer and the notebook may name.
var allowedURLHosts = map[string]bool{"pypi.org": true, "files.pythonhosted.org": true, "huggingface.co": true,
	"codeload.github.com": true, "github.com": true, "download.pytorch.org": true}

// jobURLExempt lists job.json paths whose URLs are metadata the trainer never fetches.
var jobURLExempt = map[string]bool{"profile.license.url": true}

var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"authorization header", regexp.MustCompile(`(?i)authorization`)},
	{"bearer credential", regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/-]{8,}`)},
	{"kaggle variable assignment", regexp.MustCompile(`\bKAGGLE_[A-Z0-9_]+["']?\]?\s*[:=]\s*\S`)},
	{"secret assignment", regexp.MustCompile(`(?i)\b[a-z0-9_]*(token|secret|password|passwd|api_?key)[a-z0-9_]*["']?\s*[:=]\s*["'][A-Za-z0-9_-]{16,}["']`)},
	{"hugging face token", regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}`)},
	{"github token", regexp.MustCompile(`\b(ghp|gho|ghs|ghu)_[A-Za-z0-9]{30,}|\bgithub_pat_[A-Za-z0-9_]{20,}`)},
	{"api key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`)},
	{"aws key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"slack token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
	{"windows path", regexp.MustCompile(`(?i)\b[a-z]:(\\|/(users|windows|program))`)},
	{"home path", regexp.MustCompile(`/home/[A-Za-z0-9_.-]+|/Users/[A-Za-z0-9_.-]+|/root/|~/`)},
}

var urlPattern = regexp.MustCompile("(?i)\\bhttps?://[^\\s\"'<>()\\[\\]{}\\\\`]+")

var b64Assign = regexp.MustCompile(`(?m)^(JOB_B64|SCRIPT_B64) = "([A-Za-z0-9+/=]*)"$`)

// secretFindings names the forbidden patterns in text (never the matched value).
func secretFindings(text string) []string {
	var out []string
	for _, p := range secretPatterns {
		if p.re.MatchString(text) {
			out = append(out, p.name)
		}
	}
	return out
}

// urlFindings reports URLs that are not https or not on an allowed host.
func urlFindings(text string) []string {
	var out []string
	for _, raw := range urlPattern.FindAllString(text, -1) {
		// Only scheme and host matter; a path with a format placeholder ("%s") is not a valid URL escape.
		raw = strings.TrimRight(raw, ".,;:!?")
		hostAt := strings.Index(raw, "://") + 3
		if i := strings.IndexAny(raw[hostAt:], "/?#"); i >= 0 {
			raw = raw[:hostAt+i]
		}
		u, err := url.Parse(raw)
		switch {
		case err != nil:
			out = append(out, "unparsable URL")
		case !strings.EqualFold(u.Scheme, "https"):
			out = append(out, "plain http URL")
		case !allowedURLHosts[strings.ToLower(u.Hostname())]:
			out = append(out, "URL host "+u.Hostname())
		}
	}
	return out
}

func walkJSONStrings(v any, path string, fn func(path, s string)) {
	switch x := v.(type) {
	case string:
		fn(path, x)
	case map[string]any:
		for k, e := range x {
			p := k
			if path != "" {
				p = path + "." + k
			}
			walkJSONStrings(e, p, fn)
		}
	case []any:
		for _, e := range x {
			walkJSONStrings(e, path, fn)
		}
	}
}

// jobFindings checks job.json: exact top-level keys, no secret shape anywhere, allowed URLs only.
func jobFindings(job []byte) []string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(job, &top); err != nil {
		return []string{"job.json is not a JSON object"}
	}
	var out []string
	allowed := map[string]bool{}
	for _, k := range jobTopLevelKeys {
		allowed[k] = true
		if _, ok := top[k]; !ok {
			out = append(out, "missing key "+k)
		}
	}
	for k := range top {
		if !allowed[k] {
			out = append(out, "unexpected key "+k)
		}
	}
	out = append(out, secretFindings(string(job))...)
	var v any
	_ = json.Unmarshal(job, &v)
	walkJSONStrings(v, "", func(path, s string) {
		if !jobURLExempt[path] {
			out = append(out, urlFindings(s)...)
		}
	})
	sort.Strings(out)
	return out
}

// notebookFindings checks the notebook document; the decoded job and script are checked by the caller.
func notebookFindings(nb, job, script []byte) []string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(nb, &top); err != nil {
		return []string{"notebook is not a JSON object"}
	}
	var out []string
	for k := range top {
		if k != "cells" && k != "metadata" && k != "nbformat" && k != "nbformat_minor" {
			out = append(out, "unexpected notebook key "+k)
		}
	}
	var doc struct {
		Cells []struct {
			CellType string   `json:"cell_type"`
			Source   []string `json:"source"`
		} `json:"cells"`
		NBFormat int `json:"nbformat"`
	}
	if err := json.Unmarshal(nb, &doc); err != nil || doc.NBFormat != 4 || len(doc.Cells) != 2 ||
		doc.Cells[0].CellType != "markdown" || doc.Cells[1].CellType != "code" {
		return append(out, "notebook is not the two-cell nbformat 4 document")
	}
	code := strings.Join(doc.Cells[1].Source, "")
	matches := b64Assign.FindAllStringSubmatch(code, -1)
	if len(matches) != 2 {
		out = append(out, "code cell must hold exactly JOB_B64 and SCRIPT_B64")
	}
	decoded := map[string][]byte{}
	for _, m := range matches {
		b, err := base64.StdEncoding.DecodeString(m[2])
		if err != nil {
			out = append(out, m[1]+" is not base64")
			continue
		}
		decoded[m[1]] = b
	}
	if !bytes.Equal(decoded["JOB_B64"], job) {
		out = append(out, "JOB_B64 does not decode to job.json")
	}
	if !bytes.Equal(decoded["SCRIPT_B64"], script) {
		out = append(out, "SCRIPT_B64 does not decode to the trainer")
	}
	for _, text := range []string{b64Assign.ReplaceAllString(code, `$1 = ""`), strings.Join(doc.Cells[0].Source, ""),
		string(top["metadata"])} {
		out = append(out, secretFindings(text)...)
		out = append(out, urlFindings(text)...)
	}
	sort.Strings(out)
	return out
}

func TestTrainingPayloadHasNoSecrets(t *testing.T) {
	c := mustCatalog(t)
	lock, err := trainer.LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	script := trainer.Script()
	if f := append(secretFindings(string(script)), urlFindings(string(script))...); len(f) > 0 {
		t.Errorf("trainer script: %v", f)
	}
	for _, gc := range goldenCases {
		job, err := goldenJob(t, c, gc, lock).JSON()
		if err != nil {
			t.Fatal(err)
		}
		if f := jobFindings(job); len(f) > 0 {
			t.Errorf("%s job.json: %v", gc.profile, f)
		}
		nb, err := BuildNotebook(goldenRunID, job, script)
		if err != nil {
			t.Fatal(err)
		}
		if f := notebookFindings(nb, job, script); len(f) > 0 {
			t.Errorf("%s notebook: %v", gc.profile, f)
		}
	}
}

func TestPayloadScannerCatchesPlantedValues(t *testing.T) {
	bad := []string{
		"headers = {'Authorization': x}", "Bearer abcdefghijklmnop", `os.environ["KAGGLE_KEY"] = key`,
		`KAGGLE_USERNAME = "someone"`, `hf_token = "abcdefghijklmnopqrstuvwx"`, "hf_" + strings.Repeat("a", 34),
		"sk-" + strings.Repeat("b", 24), "AKIA" + strings.Repeat("C", 16), "-----BEGIN OPENSSH PRIVATE KEY-----",
		`C:\Users\someone\aurago`, "c:/Users/someone", "/home/someone/data", "/Users/someone/data",
		"~/.kaggle/kaggle.json", "https://evil.example.com/x.tar.gz", "http://pypi.org/simple",
	}
	for _, s := range bad {
		if len(secretFindings(s))+len(urlFindings(s)) == 0 {
			t.Errorf("not caught: %q", s)
		}
	}
	good := []string{
		"https://huggingface.co/Qwen/Qwen3.5-0.8B", trainer.LlamaCppURLPrefix + "b9994", "https://pypi.org/simple/peft/",
		"/kaggle/working/chibi-status.json", `"max_new_tokens": 256`, "tokenizer = AutoTokenizer.from_pretrained(path)",
		`os.environ.pop("HF_TOKEN", None)`, `for k in ("HF_TOKEN", "HUGGING_FACE_HUB_TOKEN"): os.environ.pop(k, None)`, `if name.startswith("KAGGLE_"): del os.environ[name]`,
	}
	for _, s := range good {
		if f := append(secretFindings(s), urlFindings(s)...); len(f) > 0 {
			t.Errorf("false positive %q: %v", s, f)
		}
	}
}
