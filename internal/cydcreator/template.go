package cydcreator

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// TemplateVersion is the version of the embedded device runtime
// (template/lib/cyd_template). Projects record it and are never migrated
// automatically.
const TemplateVersion = "1.0.0"

// DeviceLanguages are the locales the device runtime has strings for; every
// other project language falls back to English on the device.
var DeviceLanguages = []string{"cs", "da", "de", "en", "es", "fr", "it", "nl", "no", "pl", "pt", "sv"}

//go:embed template
var templateFS embed.FS

// TemplateFS returns the embedded template tree rooted at template/:
// lib/cyd_template/** (copied into every generated tree), scaffold/src/app/**
// (copied once at project creation) and strings.json (device strings, used
// by the generator for glyph collection).
func TemplateFS() fs.FS {
	sub, err := fs.Sub(templateFS, "template")
	if err != nil {
		panic(err) // the directory is embedded at compile time
	}
	return sub
}

// DeviceLanguage maps a project language to the language the device runtime
// uses (the project language when the runtime has strings for it, else "en").
func DeviceLanguage(projectLanguage string) string {
	if slices.Contains(DeviceLanguages, projectLanguage) {
		return projectLanguage
	}
	return "en"
}

// templateStringsFile is the shape of template/strings.json.
type templateStringsFile struct {
	Keys    []string                     `json:"keys"`
	Strings map[string]map[string]string `json:"strings"`
}

func loadTemplateStrings() (templateStringsFile, error) {
	var f templateStringsFile
	data, err := fs.ReadFile(TemplateFS(), "strings.json")
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("template strings.json: %w", err)
	}
	return f, nil
}

// TemplateStrings returns the device strings of the runtime for a project
// language (after DeviceLanguage fallback), in strings.json key order.
func TemplateStrings(projectLanguage string) ([]string, error) {
	f, err := loadTemplateStrings()
	if err != nil {
		return nil, err
	}
	lang := DeviceLanguage(projectLanguage)
	table, ok := f.Strings[lang]
	if !ok {
		return nil, fmt.Errorf("template strings.json has no locale %q", lang)
	}
	out := make([]string, 0, len(f.Keys))
	for _, k := range f.Keys {
		out = append(out, table[k])
	}
	return out, nil
}

// renderStringsHeader renders lib/cyd_template/config/cyd_strings.h from
// strings.json. The header is committed; TestTemplateStringsHeaderUpToDate
// keeps both in sync.
func renderStringsHeader(f templateStringsFile) string {
	var b strings.Builder
	b.WriteString("// cyd_strings.h - device strings of the CYD Creator runtime.\n")
	b.WriteString("// Rendered from internal/cydcreator/template/strings.json; do not edit by hand\n")
	b.WriteString("// (run: CYD_UPDATE_STRINGS=1 go test ./internal/cydcreator/ -run TestTemplateStringsHeaderUpToDate).\n")
	b.WriteString("// The generated include/board_config.h defines exactly one CYD_LANG_<XX> macro.\n")
	b.WriteString("#pragma once\n")
	b.WriteString("#include \"board_config.h\"\n\n")
	langs := make([]string, 0, len(f.Strings))
	for _, l := range DeviceLanguages {
		if l != "en" {
			langs = append(langs, l)
		}
	}
	for i, l := range langs {
		if i == 0 {
			fmt.Fprintf(&b, "#if defined(CYD_LANG_%s)\n", strings.ToUpper(l))
		} else {
			fmt.Fprintf(&b, "#elif defined(CYD_LANG_%s)\n", strings.ToUpper(l))
		}
		writeStringDefines(&b, f.Keys, f.Strings[l])
	}
	b.WriteString("#else  // en\n")
	writeStringDefines(&b, f.Keys, f.Strings["en"])
	b.WriteString("#endif\n")
	return b.String()
}

func writeStringDefines(b *strings.Builder, keys []string, table map[string]string) {
	for _, k := range keys {
		fmt.Fprintf(b, "#define CYD_S_%s %s\n", k, cStringLiteral(table[k]))
	}
}

// cStringLiteral quotes s as a C string literal; non-ASCII stays raw UTF-8.
// Control characters (and DEL) become three-digit octal escapes, and the
// second of two consecutive question marks is escaped so that no trigraph
// such as ??= can form.
func cStringLiteral(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	var prev rune
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\%03o`, r)
		case r == '?' && prev == '?':
			b.WriteString(`\?`)
		default:
			b.WriteRune(r)
		}
		prev = r
	}
	b.WriteByte('"')
	return b.String()
}
