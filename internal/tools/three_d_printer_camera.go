package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
)

// Serialize explicit state changes; cache reads remain independent of network I/O.
var elegooCameraChange = make(chan struct{}, 1)
var elegooCameraCache = struct {
	sync.Mutex
	urls map[[32]byte]string
}{urls: make(map[[32]byte]string)}

func elegooCameraKey(p ElegooCentauriCarbonPrinter) [32]byte {
	return sha256.Sum256([]byte(p.ID + "\x00" + p.URL + "\x00" + p.MainboardID))
}

func cachedElegooCameraURL(p ElegooCentauriCarbonPrinter) (string, error) {
	elegooCameraCache.Lock()
	defer elegooCameraCache.Unlock()
	if value := elegooCameraCache.urls[elegooCameraKey(p)]; value != "" {
		return value, nil
	}
	return "", fmt.Errorf("camera activation required: use enable_camera with write permission before reading the Elegoo camera")
}

func setElegooCamera(ctx context.Context, p ElegooCentauriCarbonPrinter, enabled bool) (string, error) {
	select {
	case elegooCameraChange <- struct{}{}:
		defer func() { <-elegooCameraChange }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	key := elegooCameraKey(p)
	elegooCameraCache.Lock()
	delete(elegooCameraCache.urls, key)
	elegooCameraCache.Unlock()
	response, err := elegooCentauriCarbonCommand(ctx, p, sdcpCmdCameraURL, map[string]interface{}{"Enable": boolAsInt(enabled)})
	if err != nil {
		return "", err
	}
	if !enabled {
		return "", nil
	}
	value := findElegooCameraURL(response)
	if value == "" {
		return "", fmt.Errorf("camera activation response contains no stream URL")
	}
	value, err = resolvePrinterHTTPURL(p.URL, value)
	if err != nil {
		return "", err
	}
	if err := ValidateThreeDPrinterStreamURL(p.URL, value); err != nil {
		return "", err
	}
	elegooCameraCache.Lock()
	defer elegooCameraCache.Unlock()
	if len(elegooCameraCache.urls) >= 128 {
		clear(elegooCameraCache.urls)
	}
	elegooCameraCache.urls[key] = value
	return value, nil
}
