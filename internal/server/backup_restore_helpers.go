package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"aurago/internal/security"
)

// agoMaxBackupEntrySize is the per-file limit for regular backup entries.
// SQLite snapshots may use the total cap instead; it is a variable so tests can
// lower it. The import side has no per-entry limit (only total size and ratio).
var agoMaxBackupEntrySize int64 = 100 << 20

// agoEntryTooLargeError names the offending file and its size.
type agoEntryTooLargeError struct {
	path  string
	size  int64
	limit int64
}

func (e *agoEntryTooLargeError) Error() string {
	return fmt.Sprintf("file exceeds backup entry size limit: %s (%d bytes, limit %d bytes)", e.path, e.size, e.limit)
}

// agoBackupFailMsg appends the file name and size to a generic failure text
// when the failure was a size limit.
func agoBackupFailMsg(msg string, err error) string {
	var tooLarge *agoEntryTooLargeError
	if errors.As(err, &tooLarge) {
		return fmt.Sprintf("%s: Datei zu groß: %s (%d MiB, Limit %d MiB)", msg, tooLarge.path, tooLarge.size>>20, tooLarge.limit>>20)
	}
	return msg
}

const agoVersionUnknown = -2

// agoCompareAppVersions compares dotted numeric versions (leading "v" and any
// "-suffix"/"+build" are ignored). It returns 1 if a is newer than b, 0 if
// equal, -1 if older and agoVersionUnknown if either side is not parseable.
func agoCompareAppVersions(a, b string) int {
	pa, okA := agoParseAppVersion(a)
	pb, okB := agoParseAppVersion(b)
	if !okA || !okB {
		return agoVersionUnknown
	}
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x > y {
			return 1
		}
		if x < y {
			return -1
		}
	}
	return 0
}

func agoParseAppVersion(v string) ([]int, bool) {
	v = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "v"))
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// stageVaultSecrets decrypts the backup's vault blob with the backup password
// and writes the secrets, encrypted with the local vault key, to dest. The live
// vault is not touched.
func stageVaultSecrets(vault *security.Vault, encData []byte, password, dest string) (int, error) {
	if vault == nil {
		return 0, nil
	}
	plain, err := decryptVaultSecretsBlob(encData, password)
	if err != nil {
		return 0, err
	}
	var data map[string]string
	if err := json.Unmarshal(plain, &data); err != nil {
		return 0, fmt.Errorf("vault secrets parse: %w", err)
	}
	for key := range data {
		if vaultSkipKeys[key] {
			delete(data, key)
		}
	}
	count := len(data)
	staged, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	enc, err := vault.EncryptBytes(staged)
	if err != nil {
		return 0, fmt.Errorf("vault secrets encrypt: %w", err)
	}
	if err := os.WriteFile(dest, enc, 0o600); err != nil {
		return 0, fmt.Errorf("vault secrets stage: %w", err)
	}
	return count, nil
}
