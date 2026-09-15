package lti

import (
	"encoding/json"
	"net/http"

	josepkg "github.com/go-jose/go-jose/v3"

	internaljose "github.com/marcgabe15/lti-go/internal/jose"
)

// JWKSHandler serves this tool's JSON Web Key Set: one RSA public key per
// registered platform, keyed by that platform's key id. In later phases,
// platforms fetch this to verify client-assertion JWTs this tool sends to
// their token endpoint.
func (t *Tool) JWKSHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platforms, err := t.store.ListPlatforms(r.Context(), nil)
		if err != nil {
			t.writeError(w, r, err)
			return
		}

		set := josepkg.JSONWebKeySet{}
		for _, p := range platforms {
			kp, err := t.keyManager.KeyPair(r.Context(), p.ID)
			if err != nil {
				continue
			}
			pub, err := internaljose.ParseRSAPublicKeyPEM(kp.PublicKey)
			if err != nil {
				continue
			}
			set.Keys = append(set.Keys, josepkg.JSONWebKey{
				Key:       pub,
				KeyID:     kp.KeyID,
				Algorithm: string(josepkg.RS256),
				Use:       "sig",
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	})
}
