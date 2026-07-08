package ai

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"io"
	"testing"
)

func testMasterKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, masterKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	t.Parallel()

	key := testMasterKey(t)
	plaintext := []byte("key_abc123.super-secret-value")

	sealed, err := encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(sealed, plaintext) {
		t.Fatal("ciphertext leaks plaintext")
	}

	got, err := decrypt(key, sealed)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch: got %q", got)
	}

	// A fresh encryption of the same plaintext must differ (random nonce).
	sealed2, err := encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt 2: %v", err)
	}
	if bytes.Equal(sealed, sealed2) {
		t.Fatal("two encryptions produced identical ciphertext (nonce reuse)")
	}
}

func TestDecryptRejectsTamper(t *testing.T) {
	t.Parallel()

	key := testMasterKey(t)
	sealed, err := encrypt(key, []byte("secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	sealed[len(sealed)-1] ^= 0xff
	if _, err := decrypt(key, sealed); err == nil {
		t.Fatal("expected error on tampered ciphertext")
	}
}

func TestDecryptRejectsWrongKey(t *testing.T) {
	t.Parallel()

	sealed, err := encrypt(testMasterKey(t), []byte("secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := decrypt(testMasterKey(t), sealed); err == nil {
		t.Fatal("expected error decrypting with a different key")
	}
}

func TestDecryptRejectsShortInput(t *testing.T) {
	t.Parallel()

	if _, err := decrypt(testMasterKey(t), []byte{1, 2, 3}); err == nil {
		t.Fatal("expected error on too-short ciphertext")
	}
}

func TestParseMasterKey(t *testing.T) {
	t.Parallel()

	valid := base64.StdEncoding.EncodeToString(testMasterKey(t))
	key, err := parseMasterKey(valid)
	if err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
	if len(key) != masterKeySize {
		t.Fatalf("unexpected key length %d", len(key))
	}

	for _, bad := range []string{
		"",
		"not-base64!!!",
		base64.StdEncoding.EncodeToString([]byte("too-short")),
	} {
		if _, err := parseMasterKey(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}
