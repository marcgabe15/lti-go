# Deep Linking

Deep Linking lets an instructor pick content from your tool (an
assignment, a video, a quiz) from inside the platform's own content
picker UI, instead of you having to know a fixed `target_link_uri` ahead
of time.

## When it applies

Only for launches where `claims.MessageType ==
lti.MessageTypeDeepLinkingRequest`. Check `claims.DeepLinking != nil`
before building a response -- it's `nil` for a normal resource-link
launch.

## Building a response

```go
import "github.com/marcgabe15/lti-go/deeplink"

resp, err := deeplink.NewResponseForLaunch(claims, tool.KeyManager())
if err != nil {
	// lti.ErrDeepLinkingNotAvailable if this wasn't a deep linking launch
}

err = resp.AddItem(deeplink.LTIResourceLink{
	Title: "Week 3 Quiz",
	URL:   "https://tool.example.com/quizzes/3",
	LineItem: &deeplink.LineItem{ // optional: auto-creates an AGS line item
		ScoreMaximum: 10,
		Label:        "Week 3 Quiz",
	},
})
```

`AddItem` checks the item against what the launch declared it will
accept (`accept_types`, `accept_multiple`) and returns an error rather
than silently building a response the platform will reject:

```go
err := resp.AddItem(deeplink.Link{URL: "https://example.com"})
// error: content item type "link" is not accepted by this launch (accepted: [ltiResourceLink])
```

### Content item types

`deeplink.LTIResourceLink` (by far the most common -- launches back into
your tool), `deeplink.Link` (a plain hyperlink), `deeplink.HTML` (an
embedded HTML fragment), `deeplink.Image`, and `deeplink.File`.

## Returning the response to the platform

The response has to be POSTed back to the platform as an auto-submitting
HTML form -- write it directly to the response:

```go
mux.Handle("/lti/launch", tool.LaunchHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	claims, _ := lti.ClaimsFromContext(r.Context())
	if claims.MessageType != lti.MessageTypeDeepLinkingRequest {
		// handle a normal resource-link launch instead
		return
	}

	resp, _ := deeplink.NewResponseForLaunch(claims, tool.KeyManager())
	resp.AddItem(deeplink.LTIResourceLink{Title: "...", URL: "..."})

	if err := resp.WriteAutoSubmitForm(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
})))
```

`WriteAutoSubmitForm` signs the response and writes the form in one
call, targeting the `deep_link_return_url` the launch declared. If you
need the raw signed JWT instead (to embed in your own page), call
`resp.Sign(ctx)` and `deeplink.RenderAutoSubmitForm(w, jwt, returnURL)`
separately.

## Reporting an error instead

If content selection failed on your end, set `ErrorMessage` instead of
adding items -- the platform shows it to the user in place of a success
message:

```go
resp.ErrorMessage = "Could not load the assignment list. Please try again."
resp.WriteAutoSubmitForm(r.Context(), w)
```

`Message`/`ErrorMessage` are user-facing; `Log`/`ErrorLog` are recorded
by the platform for diagnostics but never shown to the user.
