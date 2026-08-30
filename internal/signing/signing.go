package signing

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// DefaultKeyBits is the RSA key size generated for simulator workspaces.
const DefaultKeyBits = 2048

// GenerateKey creates a new simulator RSA private key.
func GenerateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, DefaultKeyBits)
}

// SignatureInput constructs Kick's period-delimited webhook signing input.
func SignatureInput(messageID, timestamp string, body []byte) []byte {
	input := make([]byte, 0, len(messageID)+len(timestamp)+len(body)+2)
	input = append(input, messageID...)
	input = append(input, '.')
	input = append(input, timestamp...)
	input = append(input, '.')
	input = append(input, body...)
	return input
}

// Sign returns a base64-encoded RSA PKCS #1 v1.5 SHA-256 webhook signature.
func Sign(privateKey *rsa.PrivateKey, messageID, timestamp string, body []byte) (string, error) {
	if privateKey == nil {
		return "", errors.New("simulator private key is required")
	}
	digest := sha256.Sum256(SignatureInput(messageID, timestamp, body))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign webhook: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// Verify checks a base64-encoded webhook signature against its request metadata and body.
func Verify(publicKey *rsa.PublicKey, messageID, timestamp string, body []byte, encodedSignature string) error {
	if publicKey == nil {
		return errors.New("simulator public key is required")
	}
	signature, err := base64.StdEncoding.DecodeString(encodedSignature)
	if err != nil {
		return fmt.Errorf("decode webhook signature: %w", err)
	}

	digest := sha256.Sum256(SignatureInput(messageID, timestamp, body))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return fmt.Errorf("verify webhook signature: %w", err)
	}
	return nil
}

// MarshalPrivateKey encodes an RSA private key as PKCS #8 PEM.
func MarshalPrivateKey(privateKey *rsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// MarshalPublicKey encodes an RSA public key as PKIX PEM.
func MarshalPublicKey(publicKey *rsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

// ReadPrivateKey reads and parses an RSA private key from path.
func ReadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePrivateKey(data)
}

// ReadPublicKey reads and parses an RSA public key from path.
func ReadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePublicKey(data)
}

// ParsePrivateKey accepts PKCS #8 and legacy PKCS #1 PEM-encoded RSA keys.
func ParsePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("private key must be PEM encoded")
	}
	if block.Type == "RSA PRIVATE KEY" {
		privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#1 private key: %w", err)
		}
		return privateKey, nil
	}
	if block.Type != "PRIVATE KEY" {
		return nil, errors.New("private key must be PEM-encoded PRIVATE KEY")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8 private key: %w", err)
	}
	privateKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return privateKey, nil
}

// Fingerprint returns the SHA-256 digest of a public key's PKIX encoding.
func Fingerprint(publicKey *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("marshal public key for fingerprint: %w", err)
	}
	digest := sha256.Sum256(der)
	return "SHA256:" + hex.EncodeToString(digest[:]), nil
}

// ParsePublicKey parses a PKIX PEM-encoded RSA public key.
func ParsePublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("public key must be PEM-encoded PUBLIC KEY")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not RSA")
	}
	return publicKey, nil
}
