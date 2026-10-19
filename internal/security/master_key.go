package security

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

var processMasterKey struct {
	sync.RWMutex
	value string
}

// RetainProcessMasterKey is reserved for trusted startup/update code. Runtime
// interpreters receive neither this value nor the ambient master-key variable.
func RetainProcessMasterKey(value string) {
	RegisterSensitive(value)
	processMasterKey.Lock()
	processMasterKey.value = value
	processMasterKey.Unlock()
}

func ProcessMasterKey() string {
	processMasterKey.RLock()
	defer processMasterKey.RUnlock()
	return processMasterKey.value
}

// ReadMasterKeyFile accepts the same existing 32-byte hexadecimal key as the
// environment setting; it never creates, changes or rotates a key file.
func ReadMasterKeyFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open master-key file: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 257))
	if err != nil {
		return "", fmt.Errorf("read master-key file: %w", err)
	}
	value := strings.TrimSpace(string(data))
	decoded, err := hex.DecodeString(value)
	if len(data) > 256 || err != nil || len(decoded) != 32 {
		return "", fmt.Errorf("master-key file must contain exactly 64 hexadecimal characters")
	}
	RegisterSensitive(value)
	return value, nil
}
