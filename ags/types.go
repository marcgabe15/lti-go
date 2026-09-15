// Package ags implements the LTI Advantage Assignment and Grade Services
// (AGS): line item management and score/result submission against a
// platform's gradebook for one completed launch.
package ags

import "time"

// OAuth2 scopes defined by the AGS specification.
const (
	ScopeLineItem         = "https://purl.imsglobal.org/spec/lti-ags/scope/lineitem"
	ScopeLineItemReadonly = "https://purl.imsglobal.org/spec/lti-ags/scope/lineitem.readonly"
	ScopeScore            = "https://purl.imsglobal.org/spec/lti-ags/scope/score"
	ScopeResultReadonly   = "https://purl.imsglobal.org/spec/lti-ags/scope/result.readonly"
)

const (
	mediaTypeLineItem          = "application/vnd.ims.lis.v2.lineitem+json"
	mediaTypeLineItemContainer = "application/vnd.ims.lis.v2.lineitemcontainer+json"
	mediaTypeScore             = "application/vnd.ims.lis.v1.score+json"
	mediaTypeResultContainer   = "application/vnd.ims.lis.v2.resultcontainer+json"
)

// LineItem is an AGS line item (a gradebook column). ID is the line
// item's own URL, assigned by the platform; leave it empty when creating
// one.
type LineItem struct {
	ID             string     `json:"id,omitempty"`
	ScoreMaximum   float64    `json:"scoreMaximum"`
	Label          string     `json:"label"`
	ResourceID     string     `json:"resourceId,omitempty"`
	ResourceLinkID string     `json:"resourceLinkId,omitempty"`
	Tag            string     `json:"tag,omitempty"`
	StartDateTime  *time.Time `json:"startDateTime,omitempty"`
	EndDateTime    *time.Time `json:"endDateTime,omitempty"`
}

// ActivityProgress is the learner's progress through the associated
// activity, as reported in a Score.
type ActivityProgress string

// Values defined by the AGS specification for ActivityProgress.
const (
	ActivityProgressInitialized ActivityProgress = "Initialized"
	ActivityProgressStarted     ActivityProgress = "Started"
	ActivityProgressInProgress  ActivityProgress = "InProgress"
	ActivityProgressSubmitted   ActivityProgress = "Submitted"
	ActivityProgressCompleted   ActivityProgress = "Completed"
)

// GradingProgress is the grading status of a submitted Score.
type GradingProgress string

// Values defined by the AGS specification for GradingProgress.
const (
	GradingProgressFullyGraded   GradingProgress = "FullyGraded"
	GradingProgressPending       GradingProgress = "Pending"
	GradingProgressPendingManual GradingProgress = "PendingManual"
	GradingProgressFailed        GradingProgress = "Failed"
	GradingProgressNotReady      GradingProgress = "NotReady"
)

// Score is a score submission for one user against one line item.
type Score struct {
	UserID           string           `json:"userId"`
	ScoreGiven       *float64         `json:"scoreGiven,omitempty"`
	ScoreMaximum     *float64         `json:"scoreMaximum,omitempty"`
	Comment          string           `json:"comment,omitempty"`
	ActivityProgress ActivityProgress `json:"activityProgress"`
	GradingProgress  GradingProgress  `json:"gradingProgress"`
	Timestamp        time.Time        `json:"timestamp"`
}

// Result is one user's recorded result for a line item.
type Result struct {
	ID            string   `json:"id"`
	ScoreOf       string   `json:"scoreOf"`
	UserID        string   `json:"userId"`
	ResultScore   *float64 `json:"resultScore,omitempty"`
	ResultMaximum *float64 `json:"resultMaximum,omitempty"`
	Comment       string   `json:"comment,omitempty"`
}
