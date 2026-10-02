package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// MinSecretKeyLen é o tamanho mínimo de APP_SECRET_KEY.
const MinSecretKeyLen = 16

var errBadCiphertext = errors.New("segredo salvo ilegível (APP_SECRET_KEY mudou?)")

// Cipher cifra os segredos com AES-256-GCM. A chave vem de APP_SECRET_KEY (só no ambiente), nunca do banco.
type Cipher struct{ aead cipher.AEAD }

// NewCipher devolve nil (sem erro) quando a chave está vazia: segredos ficam desligados.
func NewCipher(secretKey string) (*Cipher, error) {
	if secretKey == "" {
		return nil, nil
	}
	if len(secretKey) < MinSecretKeyLen {
		return nil, errors.New("APP_SECRET_KEY muito curta: use ao menos 16 caracteres")
	}
	sum := sha256.Sum256([]byte("finassist-settings-v1:" + secretKey))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt devolve base64(nonce || texto cifrado). A chave da configuração entra como dado autenticado,
// então um segredo não pode ser copiado para outra chave.
func (c *Cipher) Encrypt(key, plain string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := c.aead.Seal(nonce, nonce, []byte(plain), []byte(key))
	return base64.StdEncoding.EncodeToString(out), nil
}

func (c *Cipher) Decrypt(key, encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) < c.aead.NonceSize() {
		return "", errBadCiphertext
	}
	n := c.aead.NonceSize()
	plain, err := c.aead.Open(nil, raw[:n], raw[n:], []byte(key))
	if err != nil {
		return "", errBadCiphertext
	}
	return string(plain), nil
}
