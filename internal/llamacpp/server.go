package llamacpp

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// FindServer locates the llama-server executable below a runtime root.
func FindServer(root string) (string, error) {
	expected := "llama-server"
	if filepath.Separator == '\\' {
		expected += ".exe"
	}
	var result string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), expected) {
			result = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan llama.cpp runtime: %w", err)
	}
	if result == "" {
		return "", fmt.Errorf("%s not found below %s", expected, root)
	}
	return result, nil
}

// GPUVerified reports whether server logs show the backend in use with layers offloaded.
func GPUVerified(backend, logs string) bool {
	lower := strings.ToLower(logs)
	backendEvidence := strings.Contains(lower, strings.ToLower(backend))
	if backend == "metal" {
		backendEvidence = strings.Contains(lower, "metal")
	}
	offloadEvidence := strings.Contains(lower, "offloaded") ||
		strings.Contains(lower, "using device") ||
		strings.Contains(lower, "gpu layers")
	return backendEvidence && offloadEvidence
}

// LoopbackPort reserves a free 127.0.0.1 TCP port.
func LoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("reserve llama-server loopback port: %w", err)
	}
	defer listener.Close()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("loopback listener did not return a TCP address")
	}
	return address.Port, nil
}

// RandomAPIKey returns a fresh URL-safe key for one llama-server process.
func RandomAPIKey() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate llama-server API key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
