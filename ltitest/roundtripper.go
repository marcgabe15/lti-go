package ltitest

import (
	"bytes"
	"io"
	"net/http"
)

// RoundTripper is a minimal mock http.RoundTripper for tests that need to
// stub an HTTP call (platform JWKS fetches, and in later phases
// AGS/NRPS/token endpoint calls) without a real network request.
type RoundTripper struct {
	// Handler is invoked for every request and returns the response to
	// use.
	Handler func(r *http.Request) (*http.Response, error)
}

// RoundTrip implements http.RoundTripper.
func (rt *RoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.Handler(r)
}

// JSONResponse builds an *http.Response with the given status code and a
// JSON body, for use from a RoundTripper.Handler.
func JSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
