package storymaker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"
)

// MusicMoods are the presets of music.mood. MusicPrompt has one template for each.
var MusicMoods = []string{"upbeat-corporate", "calm-ambient", "inspiring", "playful", "tech-pulse", "dramatic", "lo-fi", "acoustic"}

// musicTemplates are first guesses. They feed a paid call and the music key, so changing one makes
// every generated track of that mood missing.
var musicTemplates = map[string]string{
	"upbeat-corporate": "Upbeat corporate background music, bright piano and clean guitar, steady light drums, positive and professional",
	"calm-ambient":     "Calm ambient background music, soft pads and slow textures, no drums, quiet and spacious",
	"inspiring":        "Inspiring cinematic background music, warm strings and piano, gentle build, hopeful",
	"playful":          "Playful light background music, pizzicato strings, marimba and soft percussion, cheerful and bouncy",
	"tech-pulse":       "Modern technology background music, pulsing synth bass, clean electronic drums, focused and forward moving",
	"dramatic":         "Dramatic cinematic background music, low strings and deep percussion, tense and serious",
	"lo-fi":            "Lo-fi background music, mellow electric piano, soft vinyl texture, relaxed slow beat",
	"acoustic":         "Warm acoustic background music, fingerpicked guitar and light percussion, friendly and natural",
}

// MusicPrompt builds the generation prompt: the mood's template, then ". " and the user's prompt
// when both exist. A prompt without a mood stays as written.
func MusicPrompt(mood, prompt string) string {
	template, ok := musicTemplates[mood]
	switch {
	case !ok:
		return prompt
	case prompt == "":
		return template
	}
	return template + ". " + prompt
}

func musicKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

// MusicKeyGenerated is the key of generated music. The film length is not part of any key.
func MusicKeyGenerated(mood, prompt string) string {
	return musicKey("generated", mood, prompt)
}

// MusicKeyFile is the key of a desktop file source.
func MusicKeyFile(path string, size int64, mtime time.Time) string {
	return musicKey("file", path, strconv.FormatInt(size, 10), strconv.FormatInt(mtime.UnixNano(), 10))
}

// MusicKeyTrack is the key of a media registry track.
func MusicKeyTrack(id string, size int64, mtime time.Time) string {
	return musicKey("track", id, strconv.FormatInt(size, 10), strconv.FormatInt(mtime.UnixNano(), 10))
}

// MusicVolume is the bed volume of a story, 0.25 when absent.
func MusicVolume(m *Music) float64 {
	if m == nil || m.Volume == nil {
		return 0.25
	}
	return *m.Volume
}

// MusicMeta is audio/music/<key>.meta.json. It holds the duration, so /narration never decodes a track.
type MusicMeta struct {
	V          int    `json:"v"`           // 1
	Ext        string `json:"ext"`         // wav or mp3
	DurationMS int    `json:"duration_ms"` // 1..3600000
}

func validMusicMeta(m MusicMeta) bool {
	return m.V == 1 && validMusicExt(m.Ext) && m.DurationMS >= 1 && m.DurationMS <= 3600000
}

func validMusicExt(ext string) bool { return ext == "wav" || ext == "mp3" }

func validMusicKey(key string) bool {
	if len(key) != 64 {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// validMusicTrack reports a positive decimal media id. ParseInt alone would accept a sign.
func validMusicTrack(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return err == nil && n > 0
}

func (p *ProjectStore) musicFile(id, name string) string { return p.file(id, "audio", "music", name) }

func (p *ProjectStore) checkMusic(id, key string) error {
	if err := p.checkID(id); err != nil {
		return err
	}
	if !validMusicKey(key) {
		return fmt.Errorf("%w: music key %q", ErrInvalidArgument, clip(key, 80))
	}
	return nil
}

// WriteMusic writes audio/music/<key>.<ext>. The name is a content key, so an existing file is not
// an error. Store.WriteFile takes bytes: this serves SeedMusic and small tracks.
func (p *ProjectStore) WriteMusic(ctx context.Context, id, key, ext string, data []byte) error {
	if err := p.checkMusic(id, key); err != nil {
		return err
	}
	if !validMusicExt(ext) {
		return fmt.Errorf("%w: music extension %q", ErrInvalidArgument, clip(ext, 20))
	}
	if _, err := p.st.WriteFile(ctx, p.musicFile(id, key+"."+ext), data, MustNotExist); err != nil && !errors.Is(err, ErrConflict) {
		return fmt.Errorf("storymaker: write music: %w", err)
	}
	return nil
}

// WriteMusicMeta writes audio/music/<key>.meta.json and replaces an older one.
func (p *ProjectStore) WriteMusicMeta(ctx context.Context, id, key string, m MusicMeta) error {
	if err := p.checkMusic(id, key); err != nil {
		return err
	}
	if !validMusicMeta(m) {
		return fmt.Errorf("%w: music meta %+v", ErrInvalidArgument, m)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("storymaker: encode music meta: %w", err)
	}
	if _, err := p.st.WriteFile(ctx, p.musicFile(id, key+".meta.json"), append(data, '\n'), ""); err != nil {
		return fmt.Errorf("storymaker: write music meta: %w", err)
	}
	return nil
}

// MusicInfo reads the meta file and finds the audio file. meta.V is 0 when the meta is absent or
// invalid, or names another extension than the file on disk; audioExt is "" without audio.
func (p *ProjectStore) MusicInfo(ctx context.Context, id, key string) (meta MusicMeta, audioExt string, err error) {
	if err := p.checkMusic(id, key); err != nil {
		return MusicMeta{}, "", err
	}
	data, _, err := p.st.ReadFile(ctx, p.musicFile(id, key+".meta.json"))
	switch {
	case err == nil:
		if json.Unmarshal(data, &meta) != nil || !validMusicMeta(meta) {
			meta = MusicMeta{}
		}
	case !errors.Is(err, fs.ErrNotExist):
		return MusicMeta{}, "", fmt.Errorf("storymaker: read music meta: %w", err)
	}
	names, err := p.st.List(ctx, p.file(id, "audio", "music"))
	if err != nil {
		return MusicMeta{}, "", fmt.Errorf("storymaker: list music: %w", err)
	}
	has := func(ext string) bool { return containsName(names, key+"."+ext) }
	switch {
	case meta.V != 0 && has(meta.Ext):
		audioExt = meta.Ext
	case has("wav"):
		audioExt = "wav"
	case has("mp3"):
		audioExt = "mp3"
	}
	if audioExt != "" && meta.V != 0 && meta.Ext != audioExt {
		meta = MusicMeta{}
	}
	return meta, audioExt, nil
}
