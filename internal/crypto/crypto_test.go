package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestKey(t *testing.T) {
	t.Helper()
	ResetForTest()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	if err := InitKeyWithBytes(key); err != nil {
		t.Fatalf("InitKeyWithBytes: %v", err)
	}
	t.Cleanup(func() { ResetForTest() })
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	setupTestKey(t)

	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple password", "mysecretpassword"},
		{"empty string", ""},
		{"unicode", "p@$$w0rd-日本語"},
		{"long string", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"special chars", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := Encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}

			if tc.plaintext != "" && encrypted == tc.plaintext {
				t.Error("encrypted text should differ from plaintext")
			}

			decrypted, err := Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}

			if decrypted != tc.plaintext {
				t.Errorf("round-trip failed: got %q, want %q", decrypted, tc.plaintext)
			}
		})
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	setupTestKey(t)

	enc1, err := Encrypt("same-input")
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	enc2, err := Encrypt("same-input")
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}

	if enc1 == enc2 {
		t.Error("two encryptions of same input should produce different ciphertexts (random nonce)")
	}
}

func TestDecryptPlaintextFallback(t *testing.T) {
	setupTestKey(t)

	result, err := Decrypt("not-base64-!@#")
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if result != "not-base64-!@#" {
		t.Errorf("expected fallback to original, got %q", result)
	}
}

func TestDecryptEmptyString(t *testing.T) {
	setupTestKey(t)

	result, err := Decrypt("")
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestEncryptWithoutKeyFails(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	_, err := Encrypt("test")
	if err == nil {
		t.Fatal("expected error when key not initialized")
	}
}

func TestInitKeyCreatesFile(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	dir := t.TempDir()
	if err := InitKey(dir); err != nil {
		t.Fatalf("InitKey: %v", err)
	}

	keyPath := filepath.Join(dir, keyFileName)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key file: %v", err)
	}
	if len(data) != 32 {
		t.Errorf("key length: got %d, want 32", len(data))
	}
}

func TestInitKeyReusesExistingFile(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	dir := t.TempDir()
	if err := InitKey(dir); err != nil {
		t.Fatalf("InitKey first: %v", err)
	}

	keyPath := filepath.Join(dir, keyFileName)
	firstKey, _ := os.ReadFile(keyPath)

	ResetForTest()
	if err := InitKey(dir); err != nil {
		t.Fatalf("InitKey second: %v", err)
	}

	secondKey, _ := os.ReadFile(keyPath)

	if string(firstKey) != string(secondKey) {
		t.Error("key should be reused from existing file")
	}
}

func TestInitKeyWithBytesWrongLength(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	err := InitKeyWithBytes([]byte("too-short"))
	if err == nil {
		t.Fatal("expected error for wrong key length")
	}
}

func TestRawSQLDoesNotRevealPlaintext(t *testing.T) {
	setupTestKey(t)

	original := "super_secret_password"
	encrypted, err := Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if encrypted == original {
		t.Error("encrypted value should not equal original")
	}

	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decrypted != original {
		t.Errorf("decrypted: got %q, want %q", decrypted, original)
	}
}
