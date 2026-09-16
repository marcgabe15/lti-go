// Package deeplink builds and signs LTI Deep Linking response JWTs and
// renders the auto-submitting HTML form used to return them to the
// platform.
package deeplink

// Claim URIs and message type defined by the LTI Deep Linking
// specification. See
// https://www.imsglobal.org/spec/lti-dl/v2p0#deep-linking-response-message.
const (
	claimMessageType  = "https://purl.imsglobal.org/spec/lti/claim/message_type"
	claimVersion      = "https://purl.imsglobal.org/spec/lti/claim/version"
	claimDeploymentID = "https://purl.imsglobal.org/spec/lti/claim/deployment_id"
	claimContentItems = "https://purl.imsglobal.org/spec/lti-dl/claim/content_items"
	claimData         = "https://purl.imsglobal.org/spec/lti-dl/claim/data"
	claimMsg          = "https://purl.imsglobal.org/spec/lti-dl/claim/msg"
	claimErrorMsg     = "https://purl.imsglobal.org/spec/lti-dl/claim/errormsg"
	claimLog          = "https://purl.imsglobal.org/spec/lti-dl/claim/log"
	claimErrorLog     = "https://purl.imsglobal.org/spec/lti-dl/claim/errorlog"

	messageTypeDeepLinkingResponse = "LtiDeepLinkingResponse"
	ltiVersion1p3                  = "1.3.0"
)

// ContentItem is implemented by every deep linking content item type:
// LTIResourceLink, Link, HTML, Image, and File.
type ContentItem interface {
	contentItemType() string
}

// Icon is a small image reference used for a content item's icon or
// thumbnail.
type Icon struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// WindowTarget describes opening a content item in a new browser window.
type WindowTarget struct {
	TargetName     string `json:"targetName,omitempty"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	WindowFeatures string `json:"windowFeatures,omitempty"`
}

// IFrameTarget describes embedding a content item in an iframe.
type IFrameTarget struct {
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Src    string `json:"src,omitempty"`
}

// EmbedTarget describes embedding a Link's content directly via HTML.
type EmbedTarget struct {
	HTML string `json:"html,omitempty"`
}

// LineItem describes an AGS line item the platform should auto-create
// alongside a LTIResourceLink content item.
type LineItem struct {
	ScoreMaximum float64 `json:"scoreMaximum"`
	Label        string  `json:"label,omitempty"`
	ResourceID   string  `json:"resourceId,omitempty"`
	Tag          string  `json:"tag,omitempty"`
}

// LTIResourceLink is a content item that launches back into this tool as
// an LTI resource link. This is the most commonly used content item
// type.
type LTIResourceLink struct {
	Title     string            `json:"title,omitempty"`
	Text      string            `json:"text,omitempty"`
	URL       string            `json:"url,omitempty"`
	Icon      *Icon             `json:"icon,omitempty"`
	Thumbnail *Icon             `json:"thumbnail,omitempty"`
	Window    *WindowTarget     `json:"window,omitempty"`
	IFrame    *IFrameTarget     `json:"iframe,omitempty"`
	Custom    map[string]string `json:"custom,omitempty"`
	LineItem  *LineItem         `json:"lineItem,omitempty"`
}

func (LTIResourceLink) contentItemType() string { return "ltiResourceLink" }

// Link is a content item that is a plain hyperlink.
type Link struct {
	Title     string        `json:"title,omitempty"`
	Text      string        `json:"text,omitempty"`
	URL       string        `json:"url,omitempty"`
	Icon      *Icon         `json:"icon,omitempty"`
	Thumbnail *Icon         `json:"thumbnail,omitempty"`
	Window    *WindowTarget `json:"window,omitempty"`
	Embed     *EmbedTarget  `json:"embed,omitempty"`
}

func (Link) contentItemType() string { return "link" }

// HTML is a content item that embeds a fragment of HTML directly.
type HTML struct {
	HTML  string `json:"html"`
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
}

func (HTML) contentItemType() string { return "html" }

// Image is a content item referencing an image.
type Image struct {
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	Text      string `json:"text,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Icon      *Icon  `json:"icon,omitempty"`
	Thumbnail *Icon  `json:"thumbnail,omitempty"`
}

func (Image) contentItemType() string { return "image" }

// File is a content item referencing a downloadable file.
type File struct {
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	Text      string `json:"text,omitempty"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

func (File) contentItemType() string { return "file" }
