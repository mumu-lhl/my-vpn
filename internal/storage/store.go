package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/mumu-lhl/looma/internal/config"
)

const encPrefix = "enc:v1:"

type Store struct {
	mu        sync.RWMutex
	path      string
	key       [32]byte
	legacyKey [32]byte
	snapshot  *config.AppConfig
}

func NewStore(dir string, secretSeed string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{
		path:      filepath.Join(dir, "config.json"),
		key:       sha256.Sum256([]byte("looma-store:" + secretSeed)),
		legacyKey: sha256.Sum256([]byte("my-vpn-store:" + secretSeed)),
	}
	cfg, err := s.loadFromDisk()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = config.DefaultConfig()
		} else {
			return nil, err
		}
	}
	s.snapshot = cfg
	return s, nil
}

func (s *Store) Get() *config.AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.Marshal(s.snapshot)
	var cp config.AppConfig
	_ = json.Unmarshal(b, &cp)
	return &cp
}

func (s *Store) Save(cfg *config.AppConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	diskCopy := cloneConfig(cfg)
	var err error
	if diskCopy.Geph.Secret != "" {
		if diskCopy.Geph.Secret, err = s.encrypt(diskCopy.Geph.Secret); err != nil {
			return err
		}
	}
	if diskCopy.Warp.PrivateKey != "" {
		if diskCopy.Warp.PrivateKey, err = s.encrypt(diskCopy.Warp.PrivateKey); err != nil {
			return err
		}
	}
	if diskCopy.Warp.LicenseKey != "" {
		if diskCopy.Warp.LicenseKey, err = s.encrypt(diskCopy.Warp.LicenseKey); err != nil {
			return err
		}
	}

	raw, err := json.MarshalIndent(diskCopy, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	s.snapshot = cloneConfig(cfg)
	return nil
}

func (s *Store) loadFromDisk() (*config.AppConfig, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var cfg config.AppConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if cfg.Geph.Secret, err = s.decryptIfNeeded(cfg.Geph.Secret); err != nil {
		return nil, fmt.Errorf("decrypt geph secret: %w", err)
	}
	if cfg.Warp.PrivateKey, err = s.decryptIfNeeded(cfg.Warp.PrivateKey); err != nil {
		return nil, fmt.Errorf("decrypt warp private_key: %w", err)
	}
	if cfg.Warp.LicenseKey, err = s.decryptIfNeeded(cfg.Warp.LicenseKey); err != nil {
		return nil, fmt.Errorf("decrypt warp license_key: %w", err)
	}
	return &cfg, nil
}

func (s *Store) encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(s.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ct), nil
}

func decryptWithKey(key [32]byte, data []byte) (string, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func (s *Store) decryptIfNeeded(val string) (string, error) {
	if len(val) < len(encPrefix) || val[:len(encPrefix)] != encPrefix {
		return val, nil
	}
	data, err := base64.StdEncoding.DecodeString(val[len(encPrefix):])
	if err != nil {
		return "", err
	}
	if pt, err := decryptWithKey(s.key, data); err == nil {
		return pt, nil
	}
	return decryptWithKey(s.legacyKey, data)
}

func cloneConfig(cfg *config.AppConfig) *config.AppConfig {
	b, _ := json.Marshal(cfg)
	var cp config.AppConfig
	_ = json.Unmarshal(b, &cp)
	return &cp
}
