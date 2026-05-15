package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const KeyEnv = "WANGUI_MASTER_KEY"

// Crypto wraps AES-256-GCM for token encryption at rest.
type Crypto struct {
	aead cipher.AEAD
}

// NewCryptoFromKey builds a Crypto from a 32-byte key.
func NewCryptoFromKey(key []byte) (*Crypto, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Crypto{aead: aead}, nil
}

// LoadCrypto resolves the master key in order:
//  1. env var WANGUI_MASTER_KEY (hex-encoded 32 bytes)
//  2. file at devKeyPath (raw 32 bytes)
//  3. generate fresh and persist to devKeyPath (dev-mode convenience)
//
// Production deployments MUST set the env var; the dev file is gitignored.
func LoadCrypto(devKeyPath string) (*Crypto, error) {
	if v := os.Getenv(KeyEnv); v != "" {
		key, err := hex.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("invalid %s hex: %w", KeyEnv, err)
		}
		return NewCryptoFromKey(key)
	}
	if b, err := os.ReadFile(devKeyPath); err == nil {
		return NewCryptoFromKey(b)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(devKeyPath), 0o700); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(devKeyPath, key, 0o600); err != nil {
		return nil, err
	}
	return NewCryptoFromKey(key)
}

func (c *Crypto) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (c *Crypto) Decrypt(ciphertext []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("ciphertext too short")
	}
	return c.aead.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
}
