package lti

// Message types and version string defined by the LTI 1.3 core
// specification. See
// https://www.imsglobal.org/spec/lti/v1p3#lti-message-general-claims.
const (
	ltiVersion1p3 = "1.3.0"

	messageTypeResourceLink     = "LtiResourceLinkRequest"
	messageTypeDeepLinking      = "LtiDeepLinkingRequest"
	messageTypeSubmissionReview = "LtiSubmissionReviewRequest"
)
