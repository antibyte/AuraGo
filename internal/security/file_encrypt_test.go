package security

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var fileCryptTestKey = strings.Repeat("0123456789abcdef", 4)

const fileCryptTestLabel = "aurago/easydrag/purge-backup/v1"

func fileCryptFixture(t *testing.T, size int) (dir, src, enc string, plain []byte) {
	t.Helper()
	dir = t.TempDir()
	plain = make([]byte, size)
	if _, err := rand.Read(plain); err != nil {
		t.Fatal(err)
	}
	src = filepath.Join(dir, "plain.bin")
	enc = filepath.Join(dir, "backup.enc")
	if err := os.WriteFile(src, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EncryptFileWithMasterKey(src, enc, fileCryptTestLabel, fileCryptTestKey); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return dir, src, enc, plain
}

func TestFileEncryptionRoundTrip(t *testing.T) {
	for _, size := range []int{0, 1, fileCryptChunkSize - 1, fileCryptChunkSize, fileCryptChunkSize + 1, 3*fileCryptChunkSize + 5} {
		dir, _, enc, plain := fileCryptFixture(t, size)
		data, err := os.ReadFile(enc)
		if err != nil {
			t.Fatal(err)
		}
		if size >= 32 && bytes.Contains(data, plain[:32]) {
			t.Fatalf("size %d: the encrypted file holds plaintext", size)
		}
		out := filepath.Join(dir, "out.bin")
		if err := DecryptFileWithMasterKey(enc, out, fileCryptTestLabel, fileCryptTestKey); err != nil {
			t.Fatalf("size %d: decrypt: %v", size, err)
		}
		got, err := os.ReadFile(out)
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatalf("size %d: round trip differs (%v)", size, err)
		}
	}
}

func TestFileEncryptionRefusesWrongInputs(t *testing.T) {
	dir, _, enc, _ := fileCryptFixture(t, 3*fileCryptChunkSize+5)
	data, err := os.ReadFile(enc)
	if err != nil {
		t.Fatal(err)
	}
	sealedChunk := fileCryptChunkSize + fileCryptTagSize
	flip := func(at int) []byte {
		c := append([]byte(nil), data...)
		c[at] ^= 0x01
		return c
	}
	cases := map[string][]byte{
		"cut at a chunk boundary":     data[:fileCryptHeaderSize+sealedChunk],
		"cut one byte short":          data[:len(data)-1],
		"header only":                 data[:fileCryptHeaderSize],
		"flipped bit in a chunk":      flip(fileCryptHeaderSize + 100),
		"flipped bit in the header":   flip(fileCryptHeaderSize - 3),
		"flipped bit in the last tag": flip(len(data) - 1),
	}
	for name, content := range cases {
		bad := filepath.Join(dir, "bad.enc")
		if err := os.WriteFile(bad, content, 0o600); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, "out-"+strings.ReplaceAll(name, " ", "-"))
		if err := DecryptFileWithMasterKey(bad, out, fileCryptTestLabel, fileCryptTestKey); err == nil {
			t.Fatalf("%s: decrypt succeeded", name)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("%s: a failed decrypt left %s", name, out)
		}
	}
	if err := DecryptFileWithMasterKey(enc, filepath.Join(dir, "x1"), "another/label", fileCryptTestKey); err == nil {
		t.Fatal("a wrong label decrypted")
	}
	if err := DecryptFileWithMasterKey(enc, filepath.Join(dir, "x2"), fileCryptTestLabel, strings.Repeat("ab", 32)); err == nil {
		t.Fatal("a wrong key decrypted")
	}
	if err := EncryptFileWithMasterKey(enc, filepath.Join(dir, "x3"), fileCryptTestLabel, "not hex"); err == nil {
		t.Fatal("a malformed master key encrypted")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".enc-*.part")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}
