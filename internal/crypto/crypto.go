package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const keyFileName = ".encryption_key"

var (
	globalKey  []byte
	globalOnce sync.Once
	globalErr  error
)

// InitKey loads or generates an AES-256 key stored in the data directory.
func InitKey(dataDir string) error {
	globalOnce.Do(func() {
		keyPath := filepath.Join(dataDir, keyFileName)
		data, err := os.ReadFile(keyPath)
		if err == nil && len(data) == 32 {
			globalKey = data
			return
		}
		key := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			globalErr = fmt.Errorf("generate encryption key: %w", err)
			return
		}
		if err := os.WriteFile(keyPath, key, 0o600); err != nil {
			globalErr = fmt.Errorf("write encryption key: %w", err)
			return
		}
		globalKey = key
	})
	return globalErr
}

// InitKeyWithBytes sets the encryption key directly. Used for testing.
func InitKeyWithBytes(key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	globalOnce = sync.Once{}
	globalKey = key
	globalErr = nil
	return nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64 string.
// Empty plaintext is returned as-is.
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if globalKey == nil {
		return "", fmt.Errorf("encryption key not initialized")
	}

	block, err := aes.NewCipher(globalKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext.
// Empty ciphertext is returned as-is.
// If the value is not valid base64, it is returned as plaintext (legacy migration).
// All other decryption failures return an error.
func Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	if globalKey == nil {
		return "", fmt.Errorf("encryption key not initialized")
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Not valid base64 — treat as unencrypted plaintext (migration path).
		return encoded, nil
	}

	block, err := aes.NewCipher(globalKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("decrypt: ciphertext too short (%d bytes, need %d)", len(data), nonceSize)
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// ResetForTest resets global state. Only for testing.
func ResetForTest() {
	globalOnce = sync.Once{}
	globalKey = nil
	globalErr = nil
}
