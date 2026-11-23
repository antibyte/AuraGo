package trainer

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
)

const validLock = `pip peft==0.21.3
transformers
llama_cpp_tag b9994
llama_cpp_url https://codeload.github.com/ggml-org/llama.cpp/tar.gz/refs/tags/b9994
llama_cpp_sha256 8e8873031f05ec06ebf464a66fa4295edcc449d3068f3e94c6be88be7f632390
`

func TestLoadLockEmbedded(t *testing.T) {
	l, err := LoadLock()
	if err != nil {
		t.Fatalf("LoadLock: %v", err)
	}
	if len(l.Pip) != 1 || l.Pip[0] != "peft==0.22.0" || l.Transformers != "" || l.LlamaCppTag != "b9994" ||
		l.LlamaCppURL != LlamaCppURLPrefix+"b9994" {
		t.Fatalf("lock = %+v", l)
	}
	if l.LlamaCppSHA256 != "8e8873031f05ec06ebf464a66fa4295edcc449d3068f3e94c6be88be7f632390" {
		t.Fatalf("llama_cpp_sha256 = %q", l.LlamaCppSHA256)
	}
}

func TestParseLockValid(t *testing.T) {
	if _, err := ParseLock([]byte(validLock)); err != nil {
		t.Fatalf("valid lock rejected: %v", err)
	}
}

func TestParseLockRejects(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"unknown key", validLock + "foo bar\n", "unknown key"},
		{"transformers twice", validLock + "transformers 4.50.0\n", "duplicate key"},
		{"unpinned pip", strings.Replace(validLock, "pip peft==0.21.3", "pip peft>=0.21", 1), "exact name==version"},
		{"torch pip", validLock + "pip torch==2.9.0\n", "must not pin torch"},
		{"pip twice", validLock + "pip peft==0.21.3\n", "twice"},
		{"bad tag", strings.Replace(validLock, "llama_cpp_tag b9994", "llama_cpp_tag master", 1), "b<number>"},
		{"url tag mismatch", strings.Replace(validLock, "refs/tags/b9994", "refs/tags/b9993", 1), "llama_cpp_url must be"},
		{"bad sha", strings.Replace(validLock, "llama_cpp_sha256 8e88", "llama_cpp_sha256 XYZ8e88", 1), "64 lower-case hex"},
		{"sha empty", strings.Replace(validLock, "llama_cpp_sha256 8e8873031f05ec06ebf464a66fa4295edcc449d3068f3e94c6be88be7f632390", "llama_cpp_sha256 ", 1), "64 lower-case hex"},
		{"url missing", strings.Replace(validLock, "llama_cpp_url https://codeload.github.com/ggml-org/llama.cpp/tar.gz/refs/tags/b9994\n", "", 1), "missing key"},
		{"three fields", validLock + "pip a b\n", `expected "key value"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseLock([]byte(c.text))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want substring %q", err, c.want)
			}
		})
	}
}

func TestScriptEmbed(t *testing.T) {
	sum := ScriptSHA256()
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(sum) {
		t.Fatalf("ScriptSHA256 = %q", sum)
	}
	s := Script()
	h := sha256.Sum256(s)
	if hex.EncodeToString(h[:]) != sum {
		t.Fatal("ScriptSHA256 does not match Script()")
	}
	if len(s) == 0 {
		t.Fatal("empty script")
	}
	s[0] ^= 0xff
	if Script()[0] == s[0] {
		t.Fatal("Script() must return a copy")
	}
	if !ValidPin("peft==0.21.3") || ValidPin("peft==0.21.3 --index-url x") || ValidPin("git+https://x") {
		t.Fatal("ValidPin results are wrong")
	}
}
