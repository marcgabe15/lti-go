# Errors

Every error this SDK returns is (or wraps) a single `*lti.Error` type:

```go
type Error struct {
	Code ErrorCode
	Msg  string
	Err  error // the wrapped cause, if any -- e.g. a JSON decode error
}
```

There's no hierarchy of exception subtypes to remember -- one type, one
`ErrorCode` enum discriminating what went wrong.

## Checking a specific failure

Use `errors.Is` against one of the exported sentinels:

```go
if errors.Is(err, lti.ErrUnregisteredPlatform) {
	// this iss/client_id pair has no registered Platform
}
if errors.Is(err, lti.ErrNonceReused) {
	// replay attempt, or the login flow took too long
}
```

`errors.Is` compares by `Code`, not by identity or wrapped cause, so it
works the same way whether the error came straight from this package or
was wrapped somewhere in between.

## Branching on the code directly

Useful when you want to handle a category of errors the same way (for
example, mapping several codes to the same HTTP status), or when logging
the code without a long `switch` of `errors.Is` checks:

```go
if code, ok := lti.CodeOf(err); ok {
	switch code {
	case lti.CodeUnregisteredPlatform, lti.CodeInactivePlatform:
		http.Error(w, "this platform is not configured", http.StatusForbidden)
	case lti.CodeInvalidLTIK, lti.CodeLaunchNotFound:
		http.Error(w, "session expired, please relaunch", http.StatusUnauthorized)
	default:
		http.Error(w, "invalid request", http.StatusBadRequest)
	}
}
```

This is exactly what the default `ErrorHandler` does internally -- see
`lti.WithErrorHandler` in [Handling a Launch](./handling-a-launch.md) to
replace it.

## Getting at the wrapped cause

`errors.As` (or `Unwrap`) gets you the original error, when there is one
-- useful for logging the underlying JSON/HTTP/crypto failure alongside
the classified `ErrorCode`:

```go
var ltiErr *lti.Error
if errors.As(err, &ltiErr) {
	log.Printf("lti error %s: %v (cause: %v)", ltiErr.Code, ltiErr.Msg, ltiErr.Err)
}
```

## Errors from `ags`/`nrps`/`deeplink`

`ags.NewClientForLaunch` and `nrps.NewClientForLaunch` return the same
sentinels (`lti.ErrAGSNotAvailable`, `lti.ErrNRPSNotAvailable`) when a
launch didn't grant that service, so the same `errors.Is` pattern
applies. `deeplink.NewResponseForLaunch` returns
`lti.ErrDeepLinkingNotAvailable` the same way. Everything else those
packages return (a failed HTTP call to the platform, a JSON decode
failure) is a plain wrapped `error`, not an `*lti.Error` -- those
failures are about the platform's API behaving unexpectedly, not about
this SDK's own verification/authorization logic, so they don't carry an
`ErrorCode`.

## Full list of codes

See the `Code*` constants and their paired `Err*` sentinels in
`errors.go` -- they're grouped by area (login/state, nonce, id_token
validation, deployment, session/`ltik`, and per-service availability) and
each has a short doc comment.
