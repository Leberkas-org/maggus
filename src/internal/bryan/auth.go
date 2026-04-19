package bryan

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

type KeyPair struct {
	Private *rsa.PrivateKey
	Public  *rsa.PublicKey
}

func LoadOrGenerateKeys(keysDir string) (*KeyPair, error) {
	privPath := filepath.Join(keysDir, "agent.pem")
	pubPath := filepath.Join(keysDir, "agent.pub")

	if _, err := os.Stat(privPath); err == nil {
		return loadKeys(privPath, pubPath)
	}

	if err := os.MkdirAll(keysDir, 0o700); err != nil {
		return nil, fmt.Errorf("create keys dir: %w", err)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA key: %w", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return nil, fmt.Errorf("write private key: %w", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		return nil, fmt.Errorf("write public key: %w", err)
	}

	return &KeyPair{Private: key, Public: &key.PublicKey}, nil
}

func loadKeys(privPath, _ string) (*KeyPair, error) {
	privPEM, err := os.ReadFile(privPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(privPEM)
	if block == nil {
		return nil, fmt.Errorf("decode private key PEM")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return &KeyPair{Private: priv, Public: &priv.PublicKey}, nil
}

func (kp *KeyPair) SignChallenge(nonce []byte) ([]byte, error) {
	hash := sha256.Sum256(nonce)
	return rsa.SignPKCS1v15(rand.Reader, kp.Private, 0, hash[:])
}

func (kp *KeyPair) PublicKeyPEM() (string, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(kp.Public)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})), nil
}
