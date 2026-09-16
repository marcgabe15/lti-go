package lti

import (
	"errors"
	"fmt"
)

// ErrorCode discriminates the kinds of errors this package returns. Use
// CodeOf or errors.Is (against one of the Err* sentinels) to inspect it.
type ErrorCode string

const (
	// Login / platform resolution

	CodeUnregisteredPlatform ErrorCode = "unregistered_platform" // no Platform matches the (issuer, client_id) in the request
	CodeInactivePlatform     ErrorCode = "inactive_platform"     // the Platform is registered but Active is false
	CodeInvalidLoginRequest  ErrorCode = "invalid_login_request" // the OIDC login-initiation request is missing required parameters

	// State (returned by the platform alongside the id_token)

	CodeInvalidState ErrorCode = "invalid_state" // the state JWT failed signature/issuer verification
	CodeStateExpired ErrorCode = "state_expired" // the state JWT verified but its TTL has passed

	// Nonce (OIDC replay protection)

	CodeInvalidNonce ErrorCode = "invalid_nonce" // the id_token's nonce claim is missing or malformed
	CodeNonceReused  ErrorCode = "nonce_reused"  // the nonce was already consumed, never issued, or has expired

	// id_token validation

	CodeInvalidIDToken     ErrorCode = "invalid_id_token"     // the id_token failed structural or signature validation
	CodeTokenExpired       ErrorCode = "token_expired"        // the id_token's exp claim has passed
	CodeTokenTooOld        ErrorCode = "token_too_old"        // the id_token's iat claim is older than the configured max age
	CodeInvalidAudience    ErrorCode = "invalid_audience"     // the id_token's aud does not include the platform's client_id
	CodeAzpMismatch        ErrorCode = "azp_mismatch"         // aud has multiple values and azp does not equal the client_id
	CodeInvalidAlgorithm   ErrorCode = "invalid_algorithm"    // the token was not signed with RS256
	CodeInvalidMessageType ErrorCode = "invalid_message_type" // the message_type claim is missing or not one this package supports
	CodeMissingClaim       ErrorCode = "missing_claim"        // a claim required by the LTI 1.3 core spec is absent
	CodeUnknownSigningKey  ErrorCode = "unknown_signing_key"  // no key could be resolved to verify the id_token's signature

	// Deployment enforcement

	CodeUnknownDeployment ErrorCode = "unknown_deployment" // deployment_id is not registered for this platform, and no hook accepted it

	// Session resumption (ltik)

	CodeInvalidLTIK    ErrorCode = "invalid_ltik"     // the ltik failed verification or has expired
	CodeLaunchNotFound ErrorCode = "launch_not_found" // the ltik verified but its LaunchRecord no longer exists (e.g. evicted, TTL passed)

	// Per-service availability (returned by ags/nrps/deeplink, not VerifyLaunch)

	CodeAGSNotAvailable         ErrorCode = "ags_not_available"          // this launch did not grant Assignment and Grade Services
	CodeNRPSNotAvailable        ErrorCode = "nrps_not_available"         // this launch did not grant Names and Role Provisioning Service
	CodeDeepLinkingNotAvailable ErrorCode = "deep_linking_not_available" // this launch is not an LtiDeepLinkingRequest

	// Platform/deployment registration

	CodePlatformAlreadyExists   ErrorCode = "platform_already_exists"   // a Platform with this (issuer, client_id) is already registered
	CodeDeploymentAlreadyExists ErrorCode = "deployment_already_exists" // this deployment_id is already registered for the platform

	// Generic

	CodeNotFound      ErrorCode = "not_found"      // the requested Store record does not exist
	CodeInvalidConfig ErrorCode = "invalid_config" // Config passed to New is missing a required field
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
