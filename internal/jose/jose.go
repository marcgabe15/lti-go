// Package jose provides small helpers over go-jose for signing and
// verifying the RS256 JWTs used throughout LTI 1.3 (state, ltik, id_token,
// and in later phases client assertions and deep linking responses).
package jose

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	josepkg "github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
)

// ParseRSAPrivateKeyPEM parses a PEM-encoded PKCS#1 or PKCS#8 RSA private key.
func ParseRSAPrivateKeyPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("jose: no PEM block found")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jose: parse private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("jose: private key is not RSA")
	}
	return rsaKey, nil
}

// ParseRSAPublicKeyPEM parses a PEM-encoded PKIX RSA public key.
func ParseRSAPublicKeyPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("jose: no PEM block found")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jose: parse public key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("jose: public key is not RSA")
	}
	return rsaKey, nil
}

// SignClaims signs claims (typically a map[string]any or a struct
// embedding jwt.Claims) as a compact RS256 JWT using priv, with the given
// kid in the header.
func SignClaims(priv *rsa.PrivateKey, kid string, claims any) (string, error) {
	signer, err := josepkg.NewSigner(josepkg.SigningKey{
		Algorithm: josepkg.RS256,
		Key:       priv,
	}, (&josepkg.SignerOptions{}).WithHeader("kid", kid).WithType("JWT"))
	if err != nil {
		return "", fmt.Errorf("jose: new signer: %w", err)
	}
	tok, err := jwt.Signed(signer).Claims(claims).CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("jose: sign: %w", err)
	}
	return tok, nil
}

// ParseSigned parses a compact JWT without verifying its signature.
func ParseSigned(token string) (*jwt.JSONWebToken, error) {
	tok, err := jwt.ParseSigned(token)
	if err != nil {
		return nil, fmt.Errorf("jose: parse token: %w", err)
	}
	return tok, nil
}

// KeyID returns the "kid" header of a parsed token, if present.
func KeyID(tok *jwt.JSONWebToken) string {
	for _, h := range tok.Headers {
		if h.KeyID != "" {
			return h.KeyID
		}
	}
	return ""
}

// Algorithms returns the "alg" header values of a parsed token.
func Algorithms(tok *jwt.JSONWebToken) []string {
	algs := make([]string, 0, len(tok.Headers))
	for _, h := range tok.Headers {
		algs = append(algs, h.Algorithm)
	}
	return algs
}

// VerifyWithKey verifies tok's signature with key and unmarshals its
// claims into dest.
func VerifyWithKey(tok *jwt.JSONWebToken, key any, dest ...any) error {
	if err := tok.Claims(key, dest...); err != nil {
		return fmt.Errorf("jose: verify: %w", err)
	}
	return nil
}
