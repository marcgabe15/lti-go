# Dynamic Registration

Dynamic Registration lets a platform admin register your tool themselves
from inside their LMS, instead of you manually exchanging issuer/
client_id/JWKS URLs over email. The admin clicks "add tool" in their LMS,
enters your registration URL, and the platform redirects their browser
to it with everything your tool needs to complete registration itself.

## Mounting the handler

```go
import "github.com/marcgabe15/lti-go/dynreg"

handler, err := dynreg.NewHandler(dynreg.Config{
	Store:              store, // the same lti.Store your Tool uses
	ToolName:           "My Tool",
	InitiateLoginURI:   "https://tool.example.com/lti/login",   // your LoginHandler's URL
	RedirectURIs:       []string{"https://tool.example.com/lti/launch"}, // your LaunchHandler's URL
	JWKSURI:            "https://tool.example.com/lti/jwks",    // your JWKSHandler's URL
	TargetLinkURI:      "https://tool.example.com/lti/launch",
	SupportDeepLinking: true,
	Scopes: []string{
		ags.ScopeLineItem,
		nrps.ScopeContextMembershipReadonly,
	},
})
if err != nil {
	log.Fatal(err)
}

mux.Handle("/lti/register", handler)
```

Give platform admins this tool's `/lti/register` URL to start a
registration. It only handles the `GET .../register?openid_configuration=...`
request the platform redirects to -- there's no separate initiation step
on your side.

## What happens on registration

1. Fetches the platform's OpenID configuration from the
   `openid_configuration` query parameter.
2. Builds and POSTs a standard OIDC Dynamic Client Registration request
   (extended with the LTI Tool Configuration claim) to the platform's
   `registration_endpoint`, including the `registration_token` query
   parameter as a bearer token if the platform sent one.
3. Persists a new `Platform` from the platform's response --
   **inactive by default**, since any admin at any platform can trigger
   this flow and a human should confirm the relationship first. Activate
   it with `tool.Platforms().Activate(ctx, platform.ID)` once you've
   reviewed it.
4. Persists the `deployment_id` the platform assigned, if any.
5. Writes a completion page that closes the registration popup/iframe.

## Customizing the completion page

By default, a minimal page is written that posts
`{subject: 'org.imsglobal.lti.close'}` to close the registration window,
per the Dynamic Registration spec. Override it with `OnRegistered` if you
want to show your own confirmation UI or redirect somewhere:

```go
dynreg.Config{
	// ...
	OnRegistered: func(w http.ResponseWriter, r *http.Request, platform *lti.Platform) {
		fmt.Fprintf(w, "Registered %s -- pending review.", platform.Issuer)
	},
}
```

## Reviewing and activating

New registrations sit inactive until you approve them:

```go
pending := false
platforms, _ := tool.Platforms().List(ctx, &lti.ListPlatformsParams{Active: &pending})
for _, p := range platforms {
	// show p in an admin UI, then:
	tool.Platforms().Activate(ctx, p.ID)
}
```
