package vault

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

type ItemType string

const (
	TypePassword ItemType = "password"
	TypeKey      ItemType = "key"
	TypeSecret   ItemType = "secret"
)

type VaultItem struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    ItemType `json:"type"`
	Content string   `json:"content"` // Encrypted or plaintext before encryption
}

type Vault struct {
	Items []VaultItem `json:"items"`
}

func deriveKey(password []byte, salt []byte) []byte {
	return argon2.IDKey(password, salt, 1, 64*1024, 4, 32)
}

func Encrypt(data []byte, password string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	key := deriveKey([]byte(password), salt)
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// salt + nonce + ciphertext
	ciphertext := aead.Seal(nil, nonce, data, nil)
	result := append(salt, nonce...)
	result = append(result, ciphertext...)

	return result, nil
}

func Decrypt(data []byte, password string) ([]byte, error) {
	if len(data) < 16+24 { // salt(16) + nonce(24 for XChaCha or 12 for ChaCha20Poly1305)
		return nil, errors.New("ciphertext too short")
	}

	salt := data[:16]
	key := deriveKey([]byte(password), salt)

	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	nonceSize := aead.NonceSize()
	nonce := data[16 : 16+nonceSize]
	ciphertext := data[16+nonceSize:]

	return aead.Open(nil, nonce, ciphertext, nil)
}

func SaveVault(v *Vault, path string, password string) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	encrypted, err := Encrypt(data, password)
	if err != nil {
		return err
	}

	return os.WriteFile(path, encrypted, 0600)
}

func LoadVault(path string, password string) (*Vault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	decrypted, err := Decrypt(data, password)
	if err != nil {
		return nil, err
	}

	var v Vault
	err = json.Unmarshal(decrypted, &v)
	return &v, err
}
