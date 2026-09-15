package lti

import (
	"errors"
	"fmt"
)

// ErrorCode discriminates the kinds of errors this package returns. Use
// CodeOf or errors.Is (against one of the Err* sentinels) to inspect it.
type ErrorCode string

const (
	CodeUnregisteredPlatform    ErrorCode = "unregistered_platform"
	CodeInactivePlatform        ErrorCode = "inactive_platform"
	CodeInvalidLoginRequest     ErrorCode = "invalid_login_request"
	CodeInvalidState            ErrorCode = "invalid_state"
	CodeStateExpired            ErrorCode = "state_expired"
	CodeInvalidNonce            ErrorCode = "invalid_nonce"
	CodeNonceReused             ErrorCode = "nonce_reused"
	CodeInvalidIDToken          ErrorCode = "invalid_id_token"
	CodeTokenExpired            ErrorCode = "token_expired"
	CodeTokenTooOld             ErrorCode = "token_too_old"
	CodeInvalidAudience         ErrorCode = "invalid_audience"
	CodeAzpMismatch             ErrorCode = "azp_mismatch"
	CodeInvalidAlgorithm        ErrorCode = "invalid_algorithm"
	CodeInvalidMessageType      ErrorCode = "invalid_message_type"
	CodeMissingClaim            ErrorCode = "missing_claim"
	CodeUnknownDeployment       ErrorCode = "unknown_deployment"
	CodeUnknownSigningKey       ErrorCode = "unknown_signing_key"
	CodeInvalidLTIK             ErrorCode = "invalid_ltik"
	CodeLaunchNotFound          ErrorCode = "launch_not_found"
	CodeAGSNotAvailable         ErrorCode = "ags_not_available"
	CodeNRPSNotAvailable        ErrorCode = "nrps_not_available"
	CodeDeepLinkingNotAvailable ErrorCode = "deep_linking_not_available"
	CodePlatformAlreadyExists   ErrorCode = "platform_already_exists"
	CodeDeploymentAlreadyExists ErrorCode = "deployment_already_exists"
	CodeNotFound                ErrorCode = "not_found"
	CodeInvalidConfig           ErrorCode = "invalid_config"
)

// Error is the error type returned by this package. Compare against a
// sentinel with errors.Is (matching is by Code, not identity), or use
// errors.As to access the wrapped cause via Err.
type Error struct {
	Code ErrorCode
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

// Unwrap returns the wrapped cause, if any.
func (e *Error) Unwrap() error { return e.Err }

// Is reports whether target is an *Error with the same Code, so callers
// can use errors.Is(err, lti.ErrUnregisteredPlatform) regardless of the
// wrapped cause.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return t.Code == e.Code
}

// CodeOf returns the ErrorCode carried by err, if err (or something it
// wraps) is an *Error.
func CodeOf(err error) (ErrorCode, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.Code, true
	}
	return "", false
}

// wrap returns a copy of sentinel with cause attached as the wrapped
// error.
func wrap(sentinel *Error, cause error) *Error {
	return &Error{Code: sentinel.Code, Msg: sentinel.Msg, Err: cause}
}

// errorf builds an *Error with a formatted message and no wrapped cause.
func errorf(code ErrorCode, format string, args ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}

var (
	ErrUnregisteredPlatform    = &Error{Code: CodeUnregisteredPlatform, Msg: "platform is not registered"}
	ErrInactivePlatform        = &Error{Code: CodeInactivePlatform, Msg: "platform is registered but not active"}
	ErrInvalidLoginRequest     = &Error{Code: CodeInvalidLoginRequest, Msg: "invalid OIDC login initiation request"}
	ErrInvalidState            = &Error{Code: CodeInvalidState, Msg: "invalid or unverifiable state"}
	ErrStateExpired            = &Error{Code: CodeStateExpired, Msg: "state has expired"}
	ErrInvalidNonce            = &Error{Code: CodeInvalidNonce, Msg: "nonce is invalid"}
	ErrNonceReused             = &Error{Code: CodeNonceReused, Msg: "nonce has already been used, was never issued, or has expired"}
	ErrInvalidIDToken          = &Error{Code: CodeInvalidIDToken, Msg: "invalid id_token"}
	ErrTokenExpired            = &Error{Code: CodeTokenExpired, Msg: "id_token has expired"}
	ErrTokenTooOld             = &Error{Code: CodeTokenTooOld, Msg: "id_token iat is too old"}
	ErrInvalidAudience         = &Error{Code: CodeInvalidAudience, Msg: "id_token audience is invalid"}
	ErrAzpMismatch             = &Error{Code: CodeAzpMismatch, Msg: "azp does not match client_id"}
	ErrInvalidAlgorithm        = &Error{Code: CodeInvalidAlgorithm, Msg: "id_token uses an unsupported signing algorithm"}
	ErrInvalidMessageType      = &Error{Code: CodeInvalidMessageType, Msg: "unknown or unsupported LTI message type"}
	ErrMissingClaim            = &Error{Code: CodeMissingClaim, Msg: "a required claim is missing"}
	ErrUnknownDeployment       = &Error{Code: CodeUnknownDeployment, Msg: "deployment_id is not recognized for this platform"}
	ErrUnknownSigningKey       = &Error{Code: CodeUnknownSigningKey, Msg: "could not resolve a signing key for id_token verification"}
	ErrInvalidLTIK             = &Error{Code: CodeInvalidLTIK, Msg: "invalid or expired ltik"}
	ErrLaunchNotFound          = &Error{Code: CodeLaunchNotFound, Msg: "launch record not found"}
	ErrAGSNotAvailable         = &Error{Code: CodeAGSNotAvailable, Msg: "Assignment and Grade Services is not available for this launch"}
	ErrNRPSNotAvailable        = &Error{Code: CodeNRPSNotAvailable, Msg: "Names and Role Provisioning Service is not available for this launch"}
	ErrDeepLinkingNotAvailable = &Error{Code: CodeDeepLinkingNotAvailable, Msg: "deep linking is not available for this launch"}
	ErrPlatformAlreadyExists   = &Error{Code: CodePlatformAlreadyExists, Msg: "a platform with this issuer and client_id is already registered"}
	ErrDeploymentAlreadyExists = &Error{Code: CodeDeploymentAlreadyExists, Msg: "this deployment is already registered"}
	ErrNotFound                = &Error{Code: CodeNotFound, Msg: "resource not found"}
	ErrInvalidConfig           = &Error{Code: CodeInvalidConfig, Msg: "invalid configuration"}
)
