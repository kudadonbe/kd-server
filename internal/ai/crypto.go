package ai

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// masterKeySize is the AES-256 key length in bytes.
const masterKeySize = 32

// parseMasterKey decodes the base64 AI_ENCRYPTION_KEY into a 32-byte AES key.
func parseMasterKey(encoded string) ([]byte, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, errors.New("ai: AI_ENCRYPTION_KEY is not set")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("ai: AI_ENCRYPTION_KEY is not valid base64: %w", err)
	}
	if len(key) != masterKeySize {
		return nil, fmt.Errorf("ai: AI_ENCRYPTION_KEY must decode to %d bytes, got %d", masterKeySize, len(key))
	}
	return key, nil
}

// encrypt seals plaintext with AES-256-GCM and prepends the random nonce.
func encrypt(key, plaintext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("ai: generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt reverses encrypt: it splits the prepended nonce and opens the sealed
// ciphertext. A tampered or truncated ciphertext returns an error.
func decrypt(key, sealed []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(sealed) < nonceSize {
		return nil, errors.New("ai: ciphertext too short")
	}
	nonce, ciphertext := sealed[:nonceSize], sealed[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("ai: decrypt credential: %w", err)
	}
	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("ai: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("ai: new gcm: %w", err)
	}
	return gcm, nil
}
