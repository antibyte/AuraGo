package cydcreator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strconv"
	"strings"
)

const IconLibraryVersion = 1

// IconMotif identifies the same subject across every style and rendering mode.
type IconMotif struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Motion   string `json:"motion"`
}

type IconCatalog struct {
	Version int         `json:"version"`
	Styles  []string    `json:"styles"`
	Tones   []string    `json:"tones"`
	Sizes   []int       `json:"sizes"`
	Motifs  []IconMotif `json:"motifs"`
}

func Icons() IconCatalog {
	c := IconCatalog{Version: IconLibraryVersion, Styles: []string{"playful", "modern", "comic", "sober"}, Tones: []string{"color", "mono"}, Sizes: []int{24, 32, 48, 64}}
	groups := []struct{ category, ids string }{
		{"navigation", "home menu back forward up down search settings add close"},
		{"status", "folder file save trash check error warning info lock unlock"},
		{"system", "wifi ethernet bluetooth server cloud upload download sync power cpu"},
		{"home", "bulb outlet switch thermometer humidity fan heater door window motion"},
		{"weather", "battery battery_charging energy solar sun moon cloudy rain snow wind"},
		{"media", "play pause stop volume mute microphone camera bell message clock"},
	}
	frames := strings.Fields("wifi upload download bulb fan battery_charging rain snow wind volume microphone bell")
	for _, group := range groups {
		for _, id := range strings.Fields(group.ids) {
			motion := "pulse"
			if slices.Contains(frames, id) {
				motion = "frames"
			} else if id == "settings" || id == "sync" {
				motion = "rotate"
			} else if slices.Contains([]string{"back", "forward", "up", "down", "cloud", "message"}, id) {
				motion = "bob"
			}
			c.Motifs = append(c.Motifs, IconMotif{id, group.category, motion})
		}
	}
	return c
}

type IconRequest struct {
	Style    string `json:"style"`
	Tone     string `json:"tone"`
	Motif    string `json:"motif"`
	Size     int    `json:"size"`
	Animated bool   `json:"animated"`
}

func (r IconRequest) motif() (IconMotif, error) {
	c := Icons()
	if slices.Contains(c.Styles, r.Style) && slices.Contains(c.Tones, r.Tone) && slices.Contains(c.Sizes, r.Size) {
		for _, m := range c.Motifs {
			if m.ID == r.Motif {
				return m, nil
			}
		}
	}
	return IconMotif{}, NewError(CodeBadRequest, "unknown icon variant", nil)
}

func (r IconRequest) key(frame int) string {
	return fmt.Sprintf("%s/%s/%s/%d/%d.png", r.Style, r.Tone, r.Motif, r.Size, frame)
}

type iconFile struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type iconManifest struct {
	Version int                 `json:"version"`
	Files   map[string]iconFile `json:"files"`
}

// IconLibrary reads only the immutable, verified external web resource set.
type IconLibrary struct {
	files    fs.FS
	manifest iconManifest
}

func LoadIconLibrary(files fs.FS) (*IconLibrary, error) {
	if files == nil {
		return nil, NewError(CodeNotFound, "icon library is unavailable", nil)
	}
	f, err := files.Open("img/cyd-icons/manifest.json")
	if err != nil {
		return nil, fmt.Errorf("open icon library: %w", err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, (2<<20)+1))
	if err != nil || len(raw) > 2<<20 {
		return nil, NewError(CodeBadRequest, "invalid icon manifest size", nil)
	}
	l := &IconLibrary{files: files}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&l.manifest); err != nil {
		return nil, fmt.Errorf("decode icon manifest: %w", err)
	}
	if d.Decode(new(any)) != io.EOF || l.manifest.Version != IconLibraryVersion {
		return nil, NewError(CodeBadRequest, "invalid icon manifest version", nil)
	}
	expected := 0
	c := Icons()
	for _, style := range c.Styles {
		for _, tone := range c.Tones {
			for _, m := range c.Motifs {
				for _, size := range c.Sizes {
					n := 1
					if m.Motion == "frames" {
						n = 6
					}
					for frame := 0; frame < n; frame++ {
						key := (IconRequest{Style: style, Tone: tone, Motif: m.ID, Size: size}).key(frame)
						entry, ok := l.manifest.Files[key]
						if !ok || len(entry.SHA256) != 64 || entry.Bytes < 1 || entry.Bytes > MaxAssetUploadBytes {
							return nil, NewError(CodeBadRequest, "incomplete icon library", nil)
						}
						if _, err := hex.DecodeString(entry.SHA256); err != nil {
							return nil, NewError(CodeBadRequest, "invalid icon digest", nil)
						}
						expected++
					}
				}
			}
		}
	}
	if len(l.manifest.Files) != expected {
		return nil, NewError(CodeBadRequest, "unexpected icon library entries", nil)
	}
	return l, nil
}

func (l *IconLibrary) read(r IconRequest, frame int) ([]byte, Asset, error) {
	key := r.key(frame)
	entry, ok := l.manifest.Files[key]
	if !ok {
		return nil, Asset{}, NewError(CodeNotFound, "icon not found", nil)
	}
	f, err := l.files.Open("img/cyd-icons/" + key)
	if err != nil {
		return nil, Asset{}, fmt.Errorf("open icon: %w", err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, entry.Bytes+1))
	if err != nil {
		return nil, Asset{}, fmt.Errorf("read icon: %w", err)
	}
	sum := sha256.Sum256(raw)
	if int64(len(raw)) != entry.Bytes || hex.EncodeToString(sum[:]) != entry.SHA256 {
		return nil, Asset{}, NewError(CodeBadRequest, "icon integrity check failed", nil)
	}
	cfg, err := assetHeader(raw)
	if err != nil || cfg.Width != r.Size || cfg.Height != r.Size {
		return nil, Asset{}, NewError(CodeBadRequest, "invalid icon dimensions", nil)
	}
	_, _, alpha, _, err := InspectAsset(raw)
	if err != nil || !alpha {
		return nil, Asset{}, NewError(CodeBadRequest, "icon must have transparency", nil)
	}
	encoding := ""
	if r.Tone == "mono" {
		encoding = "a8"
	}
	digest := sha256.Sum256(append([]byte(encoding+"\x00"), raw...))
	id := hex.EncodeToString(digest[:16])
	a := Asset{ID: "a_icon_" + id, File: "assets/icon_" + id + ".png", W: r.Size, H: r.Size, Alpha: true, Encoding: encoding, Icon: strconv.Itoa(IconLibraryVersion) + "/" + key}
	return raw, a, nil
}
