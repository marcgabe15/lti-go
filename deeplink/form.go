package deeplink

import (
	"context"
	"fmt"
	"html/template"
	"io"
)

var formTemplate = template.Must(template.New("deeplink-form").Parse(`<!DOCTYPE html>
<html>
<head><title>Deep Linking Response</title></head>
<body onload="document.forms[0].submit()">
<form action="{{.ReturnURL}}" method="POST">
<input type="hidden" name="JWT" value="{{.JWT}}">
<noscript><button type="submit">Continue</button></noscript>
</form>
</body>
</html>
`))

// RenderAutoSubmitForm writes an HTML page that auto-submits jwt to
// returnURL via a POST form, as required by the Deep Linking
// specification's response flow. jwt and returnURL are HTML-escaped.
func RenderAutoSubmitForm(w io.Writer, jwt, returnURL string) error {
	return formTemplate.Execute(w, struct{ JWT, ReturnURL string }{JWT: jwt, ReturnURL: returnURL})
}

// WriteAutoSubmitForm signs r and writes the resulting auto-submitting
// HTML form to w, targeting the deep_link_return_url the launch
// declared. This is the one-call convenience most handlers want; use
// Sign directly if you need the raw JWT instead.
func (r *Response) WriteAutoSubmitForm(ctx context.Context, w io.Writer) error {
	jwt, err := r.Sign(ctx)
	if err != nil {
		return err
	}
	returnURL := r.claims.DeepLinking.DeepLinkReturnURL
	if returnURL == "" {
		return fmt.Errorf("deeplink: launch did not provide a deep_link_return_url")
	}
	return RenderAutoSubmitForm(w, jwt, returnURL)
}
