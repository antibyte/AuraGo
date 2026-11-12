package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// File format: magic "AGFE", version byte, chunk size (uint32, big endian), 16-byte random file
// salt, then AES-256-GCM chunks. A chunk holds up to 64 KiB of plaintext plus a 16-byte tag. The
// key is HKDF-SHA256(master key, salt = file salt, info = label), so every file has its own key
// and a counter nonce is safe. The authenticated data of a chunk is the header plus a flag that
// is 1 for the last chunk: cutting the file after a chunk boundary, or reordering chunks, fails.
const (
	fileCryptMagic      = "AGFE"
	fileCryptVersion    = 1
	fileCryptChunkSize  = 64 * 1024
	fileCryptSaltSize   = 16
	fileCryptTagSize    = 16
	fileCryptHeaderSize = len(fileCryptMagic) + 1 + 4 + fileCryptSaltSize
)

// ErrFileDecrypt means the file cannot be read with this key and label, or it is damaged or cut short.
var ErrFileDecrypt = errors.New("the encrypted file cannot be read: wrong key or label, or the file is damaged or cut short")

func fileCryptAEAD(masterKeyHex, label string, salt []byte) (cipher.AEAD, error) {
	master, err := hex.DecodeString(masterKeyHex)
	if err != nil || len(master) != 32 {
		return nil, errors.New("the master key must be 64 hexadecimal characters")
	}
	defer clear(master)
	key, err := hkdf.Key(sha256.New, master, salt, label, 32)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func fileCryptNonce(index uint64) []byte {
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce[4:], index)
	return nonce
}

func fileCryptAAD(header []byte, final bool) []byte {
	aad := append(append(make([]byte, 0, len(header)+1), header...), 0)
	if final {
		aad[len(aad)-1] = 1
	}
	return aad
}

// PlaintextWorkDirPrefix names the 0700 directories that hold plaintext temporary files.
const PlaintextWorkDirPrefix = ".flows-purge-work-"

// fileCryptOutput creates the temporary file (mode 0600) that is renamed over dst on success. When
// plain is set the content is plaintext: the file then lives in dst's directory only if that is a
// plaintext work directory, otherwise in a fresh 0700 work directory next to dst. cleanup removes
// that work directory.
func fileCryptOutput(dst string, plain bool) (*os.File, func(), error) {
	dir := filepath.Dir(dst)
	cleanup := func() {}
	if plain && !strings.HasPrefix(filepath.Base(dir), PlaintextWorkDirPrefix) {
		work, err := os.MkdirTemp(dir, PlaintextWorkDirPrefix)
		if err != nil {
			return nil, nil, err
		}
		_ = os.Chmod(work, 0o700)
		dir = work
		cleanup = func() { _ = os.RemoveAll(work) }
	}
	f, err := os.CreateTemp(dir, ".enc-*.part")
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return f, cleanup, nil
}

func fileCryptFinish(out *os.File, dst string, err error) error {
	if err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(out.Name(), dst)
	}
	if err != nil {
		_ = os.Remove(out.Name())
	}
	return err
}

// EncryptFileWithMasterKey writes src encrypted to dst (mode 0600). masterKeyHex is the 64-hex-character
// master key; label separates uses of the same key.
func EncryptFileWithMasterKey(src, dst, label, masterKeyHex string) error {
	salt := make([]byte, fileCryptSaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}
	aead, err := fileCryptAEAD(masterKeyHex, label, salt)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open file to encrypt: %w", err)
	}
	defer in.Close()
	out, cleanup, err := fileCryptOutput(dst, false)
	if err != nil {
		return fmt.Errorf("create encrypted file: %w", err)
	}
	defer cleanup()
	header := append([]byte(fileCryptMagic), fileCryptVersion)
	header = binary.BigEndian.AppendUint32(header, fileCryptChunkSize)
	header = append(header, salt...)
	return fileCryptFinish(out, dst, encryptChunks(in, out, aead, header))
}

func encryptChunks(in io.Reader, out io.Writer, aead cipher.AEAD, header []byte) error {
	if _, err := out.Write(header); err != nil {
		return err
	}
	cur, next := make([]byte, fileCryptChunkSize), make([]byte, fileCryptChunkSize)
	n, rerr := io.ReadFull(in, cur)
	for index := uint64(0); ; index++ {
		if rerr != nil && rerr != io.EOF && rerr != io.ErrUnexpectedEOF {
			return rerr
		}
		final := rerr != nil // a short or empty read is the end of the file
		var nn int
		var nerr error
		if !final {
			nn, nerr = io.ReadFull(in, next)
			if nerr == io.EOF {
				final = true
			} else if nerr != nil && nerr != io.ErrUnexpectedEOF {
				return nerr
			}
		}
		if _, err := out.Write(aead.Seal(nil, fileCryptNonce(index), cur[:n], fileCryptAAD(header, final))); err != nil {
			return err
		}
		if final {
			return nil
		}
		cur, next = next, cur
		n, rerr = nn, nerr
	}
}

// DecryptFileWithMasterKey writes the plaintext of src to dst. Any failure (wrong key or label, a
// flipped bit, a truncated file) returns ErrFileDecrypt and leaves no dst behind.
func DecryptFileWithMasterKey(src, dst, label, masterKeyHex string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open encrypted file: %w", err)
	}
	defer in.Close()
	header := make([]byte, fileCryptHeaderSize)
	if _, err := io.ReadFull(in, header); err != nil ||
		string(header[:len(fileCryptMagic)]) != fileCryptMagic || header[len(fileCryptMagic)] != fileCryptVersion ||
		binary.BigEndian.Uint32(header[len(fileCryptMagic)+1:]) != fileCryptChunkSize {
		return ErrFileDecrypt
	}
	aead, err := fileCryptAEAD(masterKeyHex, label, header[len(header)-fileCryptSaltSize:])
	if err != nil {
		return err
	}
	out, cleanup, err := fileCryptOutput(dst, true)
	if err != nil {
		return fmt.Errorf("create decrypted file: %w", err)
	}
	defer cleanup()
	return fileCryptFinish(out, dst, decryptChunks(in, out, aead, header))
}

func decryptChunks(in io.Reader, out io.Writer, aead cipher.AEAD, header []byte) error {
	size := fileCryptChunkSize + fileCryptTagSize
	cur, next := make([]byte, size), make([]byte, size)
	n, rerr := io.ReadFull(in, cur)
	for index := uint64(0); ; index++ {
		if rerr != nil && rerr != io.ErrUnexpectedEOF {
			return ErrFileDecrypt // no chunk at all, or a read error
		}
		final := rerr != nil
		var nn int
		var nerr error
		if !final {
			nn, nerr = io.ReadFull(in, next)
			if nerr == io.EOF {
				final = true
			} else if nerr != nil && nerr != io.ErrUnexpectedEOF {
				return ErrFileDecrypt
			}
		}
		plain, err := aead.Open(nil, fileCryptNonce(index), cur[:n], fileCryptAAD(header, final))
		if err != nil {
			return ErrFileDecrypt
		}
		if _, err := out.Write(plain); err != nil {
			return err
		}
		if final {
			return nil
		}
		cur, next = next, cur
		n, rerr = nn, nerr
	}
}
