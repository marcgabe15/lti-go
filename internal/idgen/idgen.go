// Package idgen generates random identifiers used for nonces, state ids,
// key ids, and launch record ids.
package idgen

import (
	"crypto/rand"
	"encoding/base64"
)

// New returns a random URL-safe, base64-encoded string derived from n
// bytes of crypto/rand output.
func New(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
