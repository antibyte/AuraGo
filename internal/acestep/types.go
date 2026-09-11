// Package acestep owns the private, managed ACE-Step music service.
package acestep

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

const (
	Owner         = "acestep"
	ContainerName = "aurago-acestep"
	ModelVolume   = "aurago_acestep_models"
	CacheVolume   = "aurago_acestep_cache"
	VaultKey      = "acestep_runtime_key"
	listenPort    = "18083"
	maxAudioBytes = 64 << 20
)

//go:embed release.json
var releaseJSON []byte

type releaseManifest struct {
	Upstream string            `json:"upstream_commit"`
	Images   map[string]string `json:"images"`
	Models   json.RawMessage   `json:"models"`
}

func manifest() releaseManifest {
	var result releaseManifest
	_ = json.Unmarshal(releaseJSON, &result)
	return result
}

func fingerprint(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type Params struct {
	Prompt          string  `json:"prompt"`
	Lyrics          string  `json:"lyrics"`
	Instrumental    bool    `json:"instrumental"`
	Title           string  `json:"title"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	BPM             int     `json:"bpm,omitempty"`
	VocalLanguage   string  `json:"vocal_language,omitempty"`
	Seed            *int64  `json:"seed,omitempty"`
}

var languagePattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z]{2,4})?$`)

func (p Params) Validate(maxDuration int, lm bool) (Params, error) {
	if strings.TrimSpace(p.Prompt) == "" || len(p.Prompt) > 16000 || len(p.Lyrics) > 32000 || len(p.Title) > 800 {
		return p, fmt.Errorf("invalid_music_text")
	}
	if p.DurationSeconds == 0 {
		p.DurationSeconds = 120
	}
	if maxDuration <= 0 || maxDuration > 600 {
		maxDuration = 600
	}
	if math.IsNaN(p.DurationSeconds) || math.IsInf(p.DurationSeconds, 0) || p.DurationSeconds < 10 || p.DurationSeconds > float64(maxDuration) {
		return p, fmt.Errorf("music_duration_out_of_range")
	}
	if p.BPM != 0 && (p.BPM < 30 || p.BPM > 300) {
		return p, fmt.Errorf("music_bpm_out_of_range")
	}
	if p.Seed != nil && (*p.Seed < 0 || *p.Seed > math.MaxInt32) {
		return p, fmt.Errorf("music_seed_out_of_range")
	}
	if p.VocalLanguage != "" && !languagePattern.MatchString(p.VocalLanguage) {
		return p, fmt.Errorf("invalid_vocal_language")
	}
	if !p.Instrumental && strings.TrimSpace(p.Lyrics) == "" && !lm {
		return p, fmt.Errorf("lyrics_required")
	}
	return p, nil
}

type Device struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Backend     string   `json:"backend"`
	Index       int      `json:"index"`
	TotalGB     float64  `json:"total_gb"`
	FreeGB      float64  `json:"free_gb"`
	Driver      string   `json:"driver,omitempty"`
	UUID        string   `json:"uuid,omitempty"`
	RenderNodes []string `json:"render_nodes,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	Verified    bool     `json:"verified"`
}

type Profile struct {
	Device         Device  `json:"device"`
	Model          string  `json:"model"`
	LMModel        string  `json:"lm_model"`
	LMBackend      string  `json:"lm_backend"`
	MaxDuration    int     `json:"max_duration"`
	Offload        bool    `json:"offload"`
	OffloadDiT     bool    `json:"offload_dit"`
	Quantization   string  `json:"quantization"`
	Compile        bool    `json:"compile"`
	FlashAttention bool    `json:"flash_attention"`
	RAMGB          float64 `json:"ram_gb"`
	DiskGB         float64 `json:"disk_gb"`
	Fingerprint    string  `json:"fingerprint"`
}

type Status struct {
	State           string   `json:"state"`
	Enabled         bool     `json:"enabled"`
	Ready           bool     `json:"ready"`
	Pending         bool     `json:"pending"`
	ErrorCode       string   `json:"error_code,omitempty"`
	Profile         *Profile `json:"profile,omitempty"`
	Devices         []Device `json:"devices"`
	DownloadedBytes int64    `json:"downloaded_bytes"`
	TotalBytes      int64    `json:"total_bytes"`
	Image           string   `json:"image,omitempty"`
	ReleaseReady    bool     `json:"release_ready"`
}

type Audio struct {
	Filename   string
	Path       string
	DurationMs int64
	Model      string
	Size       int64
}

func IsResourceName(name string) bool {
	name = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/")
	return name == ContainerName || strings.HasPrefix(name, ContainerName+"-") || name == ModelVolume || name == CacheVolume
}
