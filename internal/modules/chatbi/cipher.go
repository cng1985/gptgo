package chatbi

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Cipher struct {
	gcm cipher.AEAD
}

func OpenCipher(path string) (*Cipher, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("创建密钥目录失败: %w", err)
	}
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		if err = os.WriteFile(path, key, 0o600); err != nil {
			return nil, fmt.Errorf("写入数据源密钥失败: %w", err)
		}
	} else if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("数据源密钥长度无效")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{gcm: gcm}, nil
}

func (c *Cipher) Encrypt(plain string) (string, error) {
	if c == nil || c.gcm == nil {
		return "", fmt.Errorf("数据源密钥未就绪")
	}
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := c.gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (c *Cipher) Decrypt(encoded string) (string, error) {
	if c == nil || c.gcm == nil {
		return "", fmt.Errorf("数据源密钥未就绪")
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	if len(raw) < c.gcm.NonceSize() {
		return "", fmt.Errorf("密文无效")
	}
	nonce := raw[:c.gcm.NonceSize()]
	plain, err := c.gcm.Open(nil, nonce, raw[c.gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
