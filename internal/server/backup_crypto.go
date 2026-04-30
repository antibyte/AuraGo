package server

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

func encryptBackupPasswordBlob(magic string, plain []byte, password string) ([]byte, error) {
	salt := make([]byte, agoSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := deriveBackupKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(magic)
	buf.WriteByte(agoKDFArgon2ID)
	buf.Write(salt)
	buf.Write(nonce)
	buf.Write(gcm.Seal(nil, nonce, plain, nil))
	return buf.Bytes(), nil
}

func decryptBackupPasswordBlob(magic string, encData []byte, password string) ([]byte, error) {
	if !bytes.HasPrefix(encData, []byte(magic)) {
		return nil, fmt.Errorf("encrypted blob has invalid magic")
	}
	body := encData[len(magic):]
	if len(body) < 1+agoSaltSize+12 {
		return nil, fmt.Errorf("encrypted blob too short")
	}
	if body[0] != agoKDFArgon2ID {
		return nil, fmt.Errorf("encrypted blob has unsupported KDF version")
	}
	salt := body[1 : 1+agoSaltSize]
	plain, err := decryptBackupGCM(body[1+agoSaltSize:], deriveBackupKey(password, salt))
	if err != nil {
		return nil, fmt.Errorf("encrypted blob decryption failed (wrong password?): %w", err)
	}
	return plain, nil
}
