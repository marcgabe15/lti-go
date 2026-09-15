// Package nrps implements the LTI Advantage Names and Role Provisioning
// Service (NRPS): reading the course roster granted to one completed
// launch.
package nrps

// ScopeContextMembershipReadonly is the OAuth2 scope defined by the NRPS
// specification.
const ScopeContextMembershipReadonly = "https://purl.imsglobal.org/spec/lti-nrps/scope/contextmembership.readonly"

const mediaTypeMembershipContainer = "application/vnd.ims.lti-nrps.v2.membershipcontainer+json"

// MessageContext carries per-resource-link claims about a member, when
// the platform includes them (rare; most platforms omit this).
type MessageContext struct {
	MessageType string            `json:"message_type,omitempty"`
	Custom      map[string]string `json:"https://purl.imsglobal.org/spec/lti/claim/custom,omitempty"`
}

// Member is one entry in a course roster.
type Member struct {
	UserID     string           `json:"user_id"`
	Roles      []string         `json:"roles"`
	Status     string           `json:"status,omitempty"`
	Name       string           `json:"name,omitempty"`
	GivenName  string           `json:"given_name,omitempty"`
	FamilyName string           `json:"family_name,omitempty"`
	Email      string           `json:"email,omitempty"`
	Picture    string           `json:"picture,omitempty"`
	Message    []MessageContext `json:"message,omitempty"`
}

type membershipContainer struct {
	ID      string    `json:"id"`
	Members []*Member `json:"members"`
}
