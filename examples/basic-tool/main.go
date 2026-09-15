// Command basic-tool is a minimal, runnable LTI 1.3 tool provider. It
// registers a placeholder platform (edit the values below to point at a
// real platform, or use ltitest.FakePlatform / storetest to drive it
// programmatically) and prints the verified launch claims.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/memstore"
)

func main() {
	store := memstore.New()

	tool, err := lti.New(lti.Config{
		Issuer: "http://localhost:8080",
		Store:  store,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	if _, err := tool.Platforms().Register(ctx, &lti.Platform{
		Issuer:                 "https://platform.example.com",
		ClientID:               "example-client-id",
		Name:                   "Example Platform",
		AuthenticationEndpoint: "https://platform.example.com/auth",
		AccessTokenEndpoint:    "https://platform.example.com/token",
		Active:                 true,
		KeyConfig: lti.KeyConfig{
			Method:  lti.KeyConfigMethodJWKSet,
			JWKSURI: "https://platform.example.com/jwks",
		},
	}); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/lti/login", tool.LoginHandler())
	mux.Handle("/lti/launch", tool.LaunchHandler(http.HandlerFunc(handleLaunch)))
	mux.Handle("/lti/jwks", tool.JWKSHandler())

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func handleLaunch(w http.ResponseWriter, r *http.Request) {
	claims, ok := lti.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "no launch claims in context", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Hello, %s!\nMessage type: %s\nRoles: %v\nLTIK: %s\n",
		claims.Subject, claims.MessageType, claims.Roles, claims.LTIK)
}
