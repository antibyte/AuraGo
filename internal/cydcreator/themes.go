package cydcreator

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// Theme is one device theme (spec §5). The designer and the generator read the
// same values; nothing hard-codes them.
type Theme struct {
	ID          string      `json:"id"`           // clear night terminal sandstorm dark_sun synthwave high_contrast
	Font        string      `json:"font"`         // montserrat | jetbrains_mono
	FontSize    int         `json:"font_size"`    // base size, one of FontSizes
	ButtonStyle string      `json:"button_style"` // filled | outline | bracket
	Tokens      ThemeTokens `json:"tokens"`
}

// ThemeTokens are the color ("#RRGGBB", uppercase) and shape tokens.
type ThemeTokens struct {
	Bg          string `json:"bg"`
	Surface     string `json:"surface"`
	SurfaceAlt  string `json:"surface_alt"`
	Text        string `json:"text"`
	TextMuted   string `json:"text_muted"`
	Accent      string `json:"accent"`
	OnAccent    string `json:"on_accent"`
	Success     string `json:"success"`
	Warning     string `json:"warning"`
	Danger      string `json:"danger"`
	Border      string `json:"border"`
	Focus       string `json:"focus"`
	BorderWidth int    `json:"border_width"`
	Radius      int    `json:"radius"`
}

//go:embed themes.json
var themesRaw []byte

var (
	themesOnce  sync.Once
	themesList  []Theme
	themesCanon []byte
	themesErr   error

	themeColorPattern = regexp.MustCompile(`^#[0-9A-F]{6}$`)
	themeIDPattern    = regexp.MustCompile(`^[a-z][a-z_]{1,31}$`)
)

func loadThemes() {
	themesOnce.Do(func() {
		themesList, themesErr = parseThemes(themesRaw)
		if themesErr == nil {
			themesCanon, themesErr = json.Marshal(themesList)
		}
	})
}

// Themes returns the embedded themes in file order (clear first = default).
func Themes() ([]Theme, error) {
	loadThemes()
	if themesErr != nil {
		return nil, themesErr
	}
	return slices.Clone(themesList), nil
}

// ThemeByID returns one theme; unknown ids return *Error CYD_BAD_REQUEST.
func ThemeByID(id string) (Theme, error) {
	themes, err := Themes()
	if err != nil {
		return Theme{}, err
	}
	for _, th := range themes {
		if th.ID == id {
			return th, nil
		}
	}
	return Theme{}, NewError(CodeBadRequest, fmt.Sprintf("unknown theme %q", id), map[string]any{"theme": id})
}

// ThemesJSON returns the canonical (compact) JSON array for the API. It is nil
// only when the embedded file is broken, which TestThemesMatchSpec forbids.
func ThemesJSON() []byte {
	loadThemes()
	return slices.Clone(themesCanon)
}

func parseThemes(data []byte) ([]Theme, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var themes []Theme
	if err := dec.Decode(&themes); err != nil {
		return nil, fmt.Errorf("decode themes.json: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("decode themes.json: trailing data")
	}
	if len(themes) == 0 {
		return nil, errors.New("themes.json is empty")
	}
	seen := map[string]bool{}
	for _, th := range themes {
		if err := validateTheme(th); err != nil {
			return nil, err
		}
		if seen[th.ID] {
			return nil, fmt.Errorf("theme %q is defined twice", th.ID)
		}
		seen[th.ID] = true
	}
	return themes, nil
}

func validateTheme(th Theme) error {
	var probs []string
	if !themeIDPattern.MatchString(th.ID) {
		probs = append(probs, "id must match "+themeIDPattern.String())
	}
	if !slices.Contains(FontFamilies, th.Font) {
		probs = append(probs, fmt.Sprintf("font %q is unknown", th.Font))
	}
	if !slices.Contains(FontSizes, th.FontSize) {
		probs = append(probs, fmt.Sprintf("font_size %d is not one of %v", th.FontSize, FontSizes))
	}
	if th.ButtonStyle != "filled" && th.ButtonStyle != "outline" && th.ButtonStyle != "bracket" {
		probs = append(probs, fmt.Sprintf("button_style %q must be filled, outline or bracket", th.ButtonStyle))
	}
	tk := th.Tokens
	colors := []struct{ name, value string }{
		{"bg", tk.Bg}, {"surface", tk.Surface}, {"surface_alt", tk.SurfaceAlt}, {"text", tk.Text},
		{"text_muted", tk.TextMuted}, {"accent", tk.Accent}, {"on_accent", tk.OnAccent}, {"success", tk.Success},
		{"warning", tk.Warning}, {"danger", tk.Danger}, {"border", tk.Border}, {"focus", tk.Focus},
	}
	for _, c := range colors {
		if !themeColorPattern.MatchString(c.value) {
			probs = append(probs, fmt.Sprintf("tokens.%s %q must be #RRGGBB (uppercase)", c.name, c.value))
		}
	}
	if tk.BorderWidth < 0 || tk.BorderWidth > 8 {
		probs = append(probs, "tokens.border_width must be within 0..8")
	}
	if tk.Radius < 0 || tk.Radius > 64 {
		probs = append(probs, "tokens.radius must be within 0..64")
	}
	if len(probs) > 0 {
		return fmt.Errorf("theme %q: %s", th.ID, strings.Join(probs, "; "))
	}
	return nil
}
