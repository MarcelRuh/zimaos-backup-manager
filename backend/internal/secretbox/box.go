package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	prefix   = "enc:v1:"
	keyBytes = 32
)

var ErrInvalidCiphertext = errors.New("invalid ciphertext")

// Box encrypts secrets at rest with AES-GCM using a per-instance key file.
type Box struct {
	gcm cipher.AEAD
}

// OpenOrCreate loads a 32-byte key from path or creates one with mode 0600.
func OpenOrCreate(path string) (*Box, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read encryption key: %w", err)
		}
		key = make([]byte, keyBytes)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, fmt.Errorf("generate encryption key: %w", err)
		}
		if err := os.WriteFile(path, key, 0o600); err != nil {
			return nil, fmt.Errorf("write encryption key: %w", err)
		}
	}
	if len(key) != keyBytes {
		return nil, fmt.Errorf("encryption key must be %d bytes", keyBytes)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{gcm: gcm}, nil
}

// Seal encrypts plaintext. Empty input stays empty.
func (b *Box) Seal(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if b == nil {
		return "", errors.New("secret box not initialized")
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := b.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(out), nil
}

// Open decrypts Seal output. Legacy plaintext (no prefix) is returned unchanged.
func (b *Box) Open(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, prefix) {
		return value, nil
	}
	if b == nil {
		return "", errors.New("secret box not initialized")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	ns := b.gcm.NonceSize()
	if len(raw) < ns {
		return "", ErrInvalidCiphertext
	}
	plain, err := b.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plain), nil
}

// IsSealed reports whether value looks like Seal output.
func IsSealed(value string) bool {
	return strings.HasPrefix(value, prefix)
}
