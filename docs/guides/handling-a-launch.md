# Handling a Launch

## Two ways to verify a launch

**Mounted handler** (the common case): `Tool.LaunchHandler` verifies the
launch, issues an `ltik` session token, and calls your `next` handler
with `*lti.Claims` in the request context.

```go
mux.Handle("/lti/launch", tool.LaunchHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	claims, _ := lti.ClaimsFromContext(r.Context())
	// ...
})))
```

**Your own middleware**: if you're integrating with an existing
session/cookie/auth layer, call the primitive directly instead:

```go
claims, err := tool.VerifyLaunch(r)
```

`VerifyLaunch` does not issue an `ltik` or persist a launch record --
that's specifically what `LaunchHandler` adds on top.

## What's in `Claims`

```go
type Claims struct {
	MessageType   MessageType // "LtiResourceLinkRequest", "LtiDeepLinkingRequest", or "LtiSubmissionReviewRequest"
	Issuer        string      // the platform's issuer
	Subject       string      // the platform-assigned user id
	Audience      []string
	ClientID      string
	DeploymentID  string
	TargetLinkURI string
	IssuedAt, ExpiresAt time.Time
	Roles         []string

	Context            *ContextClaim            // course/section this launch occurred in
	ResourceLink       *ResourceLinkClaim        // the specific link that was launched
	LaunchPresentation *LaunchPresentationClaim  // iframe vs new window, dimensions, locale
	AGSEndpoint        *AGSEndpointClaim         // non-nil only if AGS was granted -- see grades-and-roster.md
	NRPSEndpoint       *NRPSEndpointClaim        // non-nil only if NRPS was granted
	DeepLinking        *DeepLinkingSettingsClaim // non-nil only for LtiDeepLinkingRequest -- see deep-linking.md

	Custom map[string]string // custom launch parameters you configured
	LTIK   string            // the session-resumption token issued for this launch
}

func (c *Claims) Platform() *Platform // the resolved Platform this launch came from
```

Check `MessageType` to know what kind of launch you're handling, and
check whether `AGSEndpoint`/`NRPSEndpoint`/`DeepLinking` are `nil` before
using them -- each service is granted per-launch, not globally.

## Roles

`Roles` is the raw list of role URIs the platform sent (for example
`http://purl.imsglobal.org/vocab/lis/v2/membership#Instructor`). There's
no built-in role-checking helper -- check with `strings.Contains` or
`strings.HasSuffix` against the URIs you care about, since the exact set
an LMS sends varies:

```go
func hasInstructorRole(claims *lti.Claims) bool {
	for _, r := range claims.Roles {
		if strings.HasSuffix(r, "#Instructor") {
			return true
		}
	}
	return false
}
```

## Resuming a launch later

`ltik` is a signed, opaque token pointing at the verified launch. Pass it
back to `Tool.VerifyLTIK` from an unrelated later request (a background
job, an AJAX call from the same page, etc.) to get the original launch
back:

```go
record, err := tool.VerifyLTIK(ctx, ltik)
// record.Claims is the same *lti.Claims from the original launch
```

It expires after 24 hours by default (`lti.WithLTIKTTL` to change that).

## Custom error handling

By default, failed logins/launches get a plain-text `http.Error` with a
status code chosen from the error's `lti.ErrorCode` (see
[Errors](./errors.md)). Override this with `lti.WithErrorHandler` at
`New` if you want branded error pages or structured JSON errors instead:

```go
tool, err := lti.New(cfg, lti.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
	code, _ := lti.CodeOf(err)
	// render your own page based on code
}))
```
