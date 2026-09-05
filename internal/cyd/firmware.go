package cyd

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	FactoryMagic   = "AGCY"
	FactoryVersion = 1
	FactorySize    = 4096
	FactoryOffset  = 0x1F0000
)

var flashParts = []FlashPart{
	{Name: "bootloader.bin", Offset: 0x1000},
	{Name: "partitions.bin", Offset: 0x8000},
	{Name: "boot_app0.bin", Offset: 0xE000},
	{Name: "firmware.bin", Offset: 0x10000},
}

// FlashPart is one ESP32 image in the web-flasher manifest.
type FlashPart struct {
	Name   string `json:"name"`
	Offset int    `json:"offset"`
}

// VariantInfo describes a board firmware pack the web flasher can install.
type VariantInfo struct {
	ID        string      `json:"id"`
	Label     string      `json:"label"`
	Version   string      `json:"version"`
	Available bool        `json:"available"`
	Missing   []string    `json:"missing,omitempty"`
	Dir       string      `json:"-"`
	Parts     []FlashPart `json:"parts"`
}

var variantLabels = map[string]string{
	"cyd":     "CYD (micro-USB, ILI9341)",
	"cyd2usb": "CYD2USB (USB-C, ST7789)",
}

// EncodeFactoryBlob packs URL+token for the cydcfg partition.
func EncodeFactoryBlob(url, token string) ([]byte, error) {
	payload, err := json.Marshal(map[string]string{
		"url":   strings.TrimSpace(url),
		"token": strings.TrimSpace(token),
	})
	if err != nil {
		return nil, err
	}
	if len(payload) > FactorySize-8 {
		return nil, fmt.Errorf("provision payload too large")
	}
	out := make([]byte, FactorySize)
	copy(out[0:4], FactoryMagic)
	out[4] = FactoryVersion
	binary.LittleEndian.PutUint16(out[6:8], uint16(len(payload)))
	copy(out[8:], payload)
	return out, nil
}

// DecodeFactoryBlob is used by tests to round-trip EncodeFactoryBlob.
func DecodeFactoryBlob(buf []byte) (url, token string, ok bool) {
	if len(buf) < 8 || string(buf[0:4]) != FactoryMagic || buf[4] != FactoryVersion {
		return "", "", false
	}
	n := int(binary.LittleEndian.Uint16(buf[6:8]))
	if n <= 0 || 8+n > len(buf) {
		return "", "", false
	}
	var payload struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if json.Unmarshal(buf[8:8+n], &payload) != nil {
		return "", "", false
	}
	return payload.URL, payload.Token, true
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func variantDirComplete(dir string) (missing []string) {
	for _, part := range flashParts {
		if !fileExists(filepath.Join(dir, part.Name)) {
			missing = append(missing, part.Name)
		}
	}
	return missing
}

func readVersion(dir string) string {
	raw, err := os.ReadFile(filepath.Join(dir, "version.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func probeVariant(id, dir string) VariantInfo {
	info := VariantInfo{
		ID:      id,
		Label:   variantLabels[id],
		Version: readVersion(dir),
		Dir:     dir,
		Parts:   append([]FlashPart(nil), flashParts...),
	}
	if info.Label == "" {
		info.Label = id
	}
	info.Missing = variantDirComplete(dir)
	info.Available = len(info.Missing) == 0
	return info
}

func uniqueDirs(dirs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range dirs {
		if d == "" {
			continue
		}
		abs, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
	return out
}

func walkParents(start string, rel string) []string {
	var out []string
	dir := start
	for i := 0; i < 6; i++ {
		out = append(out, filepath.Join(dir, rel))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return out
}

// FirmwareSearchDirs lists folders that may contain cyd/ and cyd2usb/ image packs.
func FirmwareSearchDirs(dataDir string) []string {
	var dirs []string
	if env := strings.TrimSpace(os.Getenv("CYD_FIRMWARE_DIR")); env != "" {
		dirs = append(dirs, env)
	}
	cwd, _ := os.Getwd()
	exe, _ := os.Executable()
	exeDir := ""
	if exe != "" {
		exeDir = filepath.Dir(exe)
	}
	if dataDir != "" {
		dirs = append(dirs, filepath.Join(dataDir, "cyd-firmware"))
	}
	for _, base := range []string{cwd, exeDir} {
		if base == "" {
			continue
		}
		dirs = append(dirs, filepath.Join(base, "cyd-firmware"))
		dirs = append(dirs, walkParents(base, filepath.Join("agocyd", "firmware"))...)
		dirs = append(dirs, walkParents(base, filepath.Join("agocyd", ".pio", "build"))...)
		dirs = append(dirs, walkParents(base, filepath.Join("repo", "agocyd", "firmware"))...)
	}
	return uniqueDirs(dirs)
}

func pickVariant(id string, roots []string) VariantInfo {
	var best VariantInfo
	best.ID = id
	best.Label = variantLabels[id]
	if best.Label == "" {
		best.Label = id
	}
	best.Parts = append([]FlashPart(nil), flashParts...)
	best.Missing = []string{"bootloader.bin", "partitions.bin", "boot_app0.bin", "firmware.bin"}
	for _, root := range roots {
		dir := filepath.Join(root, id)
		info := probeVariant(id, dir)
		if info.Available {
			return info
		}
		if best.Dir == "" || len(info.Missing) < len(best.Missing) {
			best = info
		}
	}
	return best
}

// DiscoverFirmware finds CYD flash images on disk.
func DiscoverFirmware(dataDir string) []VariantInfo {
	roots := FirmwareSearchDirs(dataDir)
	out := []VariantInfo{
		pickVariant("cyd", roots),
		pickVariant("cyd2usb", roots),
	}
	return out
}

// FirmwareFilePath returns the absolute path of a named image, or empty.
func FirmwareFilePath(dataDir, variant, name string) string {
	variant = strings.ToLower(strings.TrimSpace(variant))
	name = filepath.Base(name)
	allowed := false
	for _, part := range flashParts {
		if part.Name == name {
			allowed = true
			break
		}
	}
	if !allowed || (variant != "cyd" && variant != "cyd2usb") {
		return ""
	}
	info := pickVariant(variant, FirmwareSearchDirs(dataDir))
	if info.Dir == "" {
		return ""
	}
	path := filepath.Join(info.Dir, name)
	if !fileExists(path) {
		return ""
	}
	return path
}
