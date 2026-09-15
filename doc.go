// Package lti implements the tool-provider side of the IMS Global
// Learning Tools Interoperability (LTI) 1.3 specification:
// https://www.imsglobal.org/spec/lti/v1p3.
//
// A Tool verifies OIDC-based launches sent by platforms (LMSs) and hands
// the resulting Claims to your application code:
//
//	tool, err := lti.New(lti.Config{
//		Issuer: "https://tool.example.com",
//		Store:  myStore, // implement lti.Store, or use memstore.New() for tests
//	})
//	mux.Handle("/lti/login", tool.LoginHandler())
//	mux.Handle("/lti/launch", tool.LaunchHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		claims, _ := lti.ClaimsFromContext(r.Context())
//		// use claims.Subject, claims.Roles, claims.ResourceLink, etc.
//	})))
//	mux.Handle("/lti/jwks", tool.JWKSHandler())
//
// Register the platforms your tool trusts via Tool.Platforms:
//
//	tool.Platforms().Register(ctx, &lti.Platform{
//		Issuer:                 "https://platform.example.com",
//		ClientID:               "client-id-issued-by-platform",
//		AuthenticationEndpoint: "https://platform.example.com/auth",
//		KeyConfig: lti.KeyConfig{
//			Method:  lti.KeyConfigMethodJWKSet,
//			JWKSURI: "https://platform.example.com/jwks",
//		},
//	})
//
// Persistence (platforms, deployments, nonces, launches, cached tokens,
// and signing keys) is pluggable via the Store interface; the memstore
// subpackage provides an in-memory implementation for tests and examples.
package lti
