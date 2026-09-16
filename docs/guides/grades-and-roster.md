# Grades and Roster (AGS / NRPS)

**AGS** (Assignment and Grade Services) posts scores into the platform's
gradebook. **NRPS** (Names and Role Provisioning Service) reads the
course roster. Both are optional per-launch -- a platform only grants
them if your tool is configured to request them, so always check the
error from the constructor.

## Shared setup: a token source

Both services authenticate the same way: your tool signs a JWT assertion
with the platform's dedicated key and exchanges it for an access token.
Build one `token.Source` from your `Tool` and reuse it everywhere --
it caches tokens per platform+scopes via your `Store`, so you don't pay
for a new token on every request:

```go
import "github.com/marcgabe15/lti-go/token"

tokens := token.NewCachingSource(tool.Store(), tool.KeyManager())
```

Do this once at startup, not per-request.

## Posting a grade (AGS)

```go
import "github.com/marcgabe15/lti-go/ags"

agsClient, err := ags.NewClientForLaunch(claims, tokens)
if err != nil {
	// lti.ErrAGSNotAvailable -- this launch didn't grant AGS
}

scoreGiven, scoreMax := 8.5, 10.0
err = agsClient.SubmitScore(ctx, claims.AGSEndpoint.LineItem, &ags.Score{
	UserID:           claims.Subject,
	ScoreGiven:       &scoreGiven,
	ScoreMaximum:     &scoreMax,
	ActivityProgress: ags.ActivityProgressCompleted,
	GradingProgress:  ags.GradingProgressFullyGraded,
})
```

`claims.AGSEndpoint.LineItem` is only set when the launch is already tied
to one specific line item (common for a simple resource-link
integration). If your tool manages its own line items instead:

```go
items, err := agsClient.GetLineItems(ctx, &ags.GetLineItemsParams{ResourceLinkID: claims.ResourceLink.ID})
if len(items) == 0 {
	item, err := agsClient.CreateLineItem(ctx, &ags.LineItem{
		ScoreMaximum:   100,
		Label:          "Assignment 1",
		ResourceLinkID: claims.ResourceLink.ID,
	})
}
```

`GetLineItems` and `GetResults` follow the platform's pagination
(`Link` header) automatically and return the full result set.

## Reading the roster (NRPS)

```go
import "github.com/marcgabe15/lti-go/nrps"

nrpsClient, err := nrps.NewClientForLaunch(claims, tokens)
if err != nil {
	// lti.ErrNRPSNotAvailable -- this launch didn't grant NRPS
}

members, err := nrpsClient.GetMembers(ctx, nil)
for _, m := range members {
	fmt.Println(m.UserID, m.Name, m.Roles)
}
```

Filter server-side with `GetMembersParams{Role: "...", ResourceLinkID:
"..."}`, and cap pagination with `MaxPages` if you only need the first
page or two (also a defensive guard against an unexpectedly long roster
or a misbehaving `Link` header):

```go
members, err := nrpsClient.GetMembers(ctx, &nrps.GetMembersParams{MaxPages: 5})
```

## Both together in a launch handler

```go
mux.Handle("/lti/launch", tool.LaunchHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	claims, _ := lti.ClaimsFromContext(r.Context())

	if agsClient, err := ags.NewClientForLaunch(claims, tokens); err == nil {
		// post a grade
	}
	if nrpsClient, err := nrps.NewClientForLaunch(claims, tokens); err == nil {
		// read the roster
	}
})))
```
