package cyd

import (
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

const (
	FactoryMagic   = "AGCY"
	FactoryVersion = 1
	FactorySize    = 4096
	FactoryOffset  = 0x1F0000
	// BundledDir is the repo path of the packed CYD images.
	BundledDir = "internal/cyd/firmware"
)

//go:embed firmware/cyd
var bundled embed.FS

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
	Parts     []FlashPart `json:"parts"`
}

var variantLabels = map[string]string{
	"cyd":     "CYD (micro-USB, ILI9341)",
	"cyd2usb": "CYD2USB (USB-C, ST7789)",
}

func allowedPart(name string) bool {
	for _, part := range flashParts {
		if part.Name == name {
			return true
		}
	}
	return false
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

func bundledVariant(id string) VariantInfo {
	info := VariantInfo{
		ID:    id,
		Label: variantLabels[id],
		Parts: append([]FlashPart(nil), flashParts...),
	}
	if info.Label == "" {
		info.Label = id
	}
	dir := path.Join("firmware", id)
	for _, part := range flashParts {
		if _, err := bundled.ReadFile(path.Join(dir, part.Name)); err != nil {
			info.Missing = append(info.Missing, part.Name)
		}
	}
	if raw, err := bundled.ReadFile(path.Join(dir, "version.txt")); err == nil {
		info.Version = strings.TrimSpace(string(raw))
	}
	info.Available = len(info.Missing) == 0
	return info
}

// DiscoverFirmware returns the packed CYD images shipped with AuraGo.
func DiscoverFirmware() []VariantInfo {
	return []VariantInfo{bundledVariant("cyd")}
}

// FirmwareBytes returns a bundled flash image.
func FirmwareBytes(variant, name string) ([]byte, error) {
	variant = strings.ToLower(strings.TrimSpace(variant))
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	if variant != "cyd" || !allowedPart(name) {
		return nil, fmt.Errorf("unknown firmware image")
	}
	data, err := bundled.ReadFile(path.Join("firmware", variant, name))
	if err != nil {
		return nil, fmt.Errorf("unknown firmware image")
	}
	return data, nil
}
