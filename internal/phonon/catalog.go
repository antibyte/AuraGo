// Package phonon owns the pinned, offline CPU speech runtime and voice catalog.
package phonon

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed manifest.json
var manifestJSON []byte

type File struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Voice struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Language  string `json:"language"`
	Locale    string `json:"locale"`
	Gender    string `json:"gender"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Bundled   bool   `json:"bundled"`
	Installed bool   `json:"installed"`
}

type catalogManifest struct {
	ModelRevision  string  `json:"model_revision"`
	ModelFiles     []File  `json:"model_files"`
	VoicesRepo     string  `json:"voices_repo"`
	VoicesRevision string  `json:"voices_revision"`
	Voices         []Voice `json:"voices"`
	RuntimeVersion string  `json:"runtime_version"`
	Runtimes       []struct {
		Target       string `json:"target"`
		BinarySHA256 string `json:"binary_sha256"`
		BinarySize   int64  `json:"binary_size"`
	} `json:"runtimes"`
}

var manifest = func() catalogManifest {
	var m catalogManifest
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		panic("invalid bundled Phonon manifest")
	}
	return m
}()

func Revision() string {
	return manifest.RuntimeVersion + ":" + manifest.ModelRevision + ":" + manifest.VoicesRevision
}

func BundleID() string {
	return manifest.RuntimeVersion + "-" + manifest.ModelRevision[:12] + "-" + manifest.VoicesRevision[:12]
}

// Language deliberately rejects unsupported tags instead of normalizing them to English.
func Language(language string) string {
	base := strings.Split(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(language)), "_", "-"), "-")[0]
	switch base {
	case "en", "fr", "de", "es", "pt":
		return base
	}
	return ""
}

func Lookup(id string) (Voice, bool) {
	for _, voice := range manifest.Voices {
		if voice.ID == id {
			return voice, true
		}
	}
	return Voice{}, false
}

// DefaultVoice maps reviewed built-in characters; unspecified/custom personas use a female voice.
func DefaultVoice(language, persona string) string {
	gender := "female"
	switch strings.ToLower(strings.TrimSpace(persona)) {
	case "servant", "professional", "thinker", "evil", "psycho", "friend", "terminator":
		gender = "male"
	}
	for _, voice := range manifest.Voices {
		if voice.Bundled && voice.Language == Language(language) && voice.Gender == gender {
			return voice.ID
		}
	}
	return ""
}
