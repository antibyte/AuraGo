// Package trainer embeds the Chibi Sensei training script and its dependency pins. It must never import
// aurago/internal/chibisensei, which imports it.
package trainer

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

//go:embed chibi_train.py
var script []byte

//go:embed requirements.lock
var lockText []byte

// LlamaCppURLPrefix is the only allowed source of the converter tarball; the tag is appended.
const LlamaCppURLPrefix = "https://codeload.github.com/ggml-org/llama.cpp/tar.gz/refs/tags/"

// Script returns a copy of the embedded chibi_train.py.
func Script() []byte { return bytes.Clone(script) }

// ScriptSHA256 is the hex sha256 of the embedded script.
func ScriptSHA256() string {
	sum := sha256.Sum256(script)
	return hex.EncodeToString(sum[:])
}

// Lock is the parsed requirements.lock.
type Lock struct {
	Pip            []string // "name==version" lines
	Transformers   string   // "" keeps the preinstalled version
	LlamaCppTag    string
	LlamaCppURL    string
	LlamaCppSHA256 string // mandatory: the trainer refuses an unverified tarball
}

var (
	pinPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}==[0-9][0-9A-Za-z.+!-]{0,31}$`)
	versionPattern = regexp.MustCompile(`^[0-9][0-9A-Za-z.+!-]{0,31}$`)
	tagPattern     = regexp.MustCompile(`^b[0-9]{1,6}$`)
	sha256Pattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ValidPin reports whether s is an exact pip pin "name==version" without options, URLs or markers.
func ValidPin(s string) bool { return pinPattern.MatchString(s) }

// LoadLock parses the embedded requirements.lock.
func LoadLock() (Lock, error) { return ParseLock(lockText) }

// ParseLock parses the lock format. Keys: pip (repeatable), transformers, llama_cpp_tag, llama_cpp_url and
// llama_cpp_sha256 (each exactly once; transformers may have an empty value).
func ParseLock(b []byte) (Lock, error) {
	var l Lock
	seen := map[string]bool{}
	for i, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) > 2 {
			return Lock{}, fmt.Errorf("requirements.lock line %d: expected \"key value\"", i+1)
		}
		key, value := f[0], ""
		if len(f) == 2 {
			value = f[1]
		}
		if key != "pip" {
			if seen[key] {
				return Lock{}, fmt.Errorf("requirements.lock line %d: duplicate key %q", i+1, key)
			}
			seen[key] = true
		}
		switch key {
		case "pip":
			l.Pip = append(l.Pip, value)
		case "transformers":
			l.Transformers = value
		case "llama_cpp_tag":
			l.LlamaCppTag = value
		case "llama_cpp_url":
			l.LlamaCppURL = value
		case "llama_cpp_sha256":
			l.LlamaCppSHA256 = value
		default:
			return Lock{}, fmt.Errorf("requirements.lock line %d: unknown key %q", i+1, key)
		}
	}
	for _, k := range []string{"transformers", "llama_cpp_tag", "llama_cpp_url", "llama_cpp_sha256"} {
		if !seen[k] {
			return Lock{}, fmt.Errorf("requirements.lock: missing key %q", k)
		}
	}
	if err := l.Validate(); err != nil {
		return Lock{}, fmt.Errorf("requirements.lock: %w", err)
	}
	return l, nil
}

// Validate checks the pins: exact pip pins, no torch or transformers pip line, no package twice, the
// converter URL derived from the tag and an optional sha256.
func (l Lock) Validate() error {
	names := map[string]bool{}
	for _, p := range l.Pip {
		if !ValidPin(p) {
			return fmt.Errorf("pip %q is not an exact name==version pin", p)
		}
		name := strings.ToLower(strings.NewReplacer("_", "-", ".", "-").Replace(strings.SplitN(p, "==", 2)[0]))
		if name == "torch" || name == "transformers" {
			return fmt.Errorf("pip must not pin %s", name)
		}
		if names[name] {
			return fmt.Errorf("pip pins %s twice", name)
		}
		names[name] = true
	}
	if l.Transformers != "" && !versionPattern.MatchString(l.Transformers) {
		return fmt.Errorf("transformers %q is not a version", l.Transformers)
	}
	if !tagPattern.MatchString(l.LlamaCppTag) {
		return fmt.Errorf("llama_cpp_tag %q is not a b<number> tag", l.LlamaCppTag)
	}
	if l.LlamaCppURL != LlamaCppURLPrefix+l.LlamaCppTag {
		return fmt.Errorf("llama_cpp_url must be %s%s", LlamaCppURLPrefix, l.LlamaCppTag)
	}
	if !sha256Pattern.MatchString(l.LlamaCppSHA256) {
		return fmt.Errorf("llama_cpp_sha256 is not 64 lower-case hex characters")
	}
	return nil
}
