// Package apperr defines errors that are safe to show to users.
//
// An Error carries a stable code the frontend can act on, a message
// written for users, and the underlying error, which is only logged.
package apperr

import (
	"errors"
	"time"
)

// Code identifies a kind of failure. Codes are part of the contract with
// the frontend: add new ones when needed, but do not rename existing ones.
type Code string

const (
	// Internal is an unexpected failure, usually a bug.
	Internal Code = "internal"
	// Unavailable means a required local resource, such as the database,
	// cannot be used right now.
	Unavailable Code = "unavailable"
	// InvalidInput means the request from the UI was not valid.
	InvalidInput Code = "invalid_input"
	// RateLimited means an external service received too many requests.
	// Error.RetryAfter says when to try again.
	RateLimited Code = "rate_limited"
	// ProviderUnavailable means an external image provider could not be
	// reached or did not respond correctly.
	ProviderUnavailable Code = "provider_unavailable"
	// Canceled means the operation was stopped on purpose, for example
	// because the user started a new search. The UI does not show it.
	Canceled Code = "canceled"
)

const internalMessage = "Something went wrong. Details were written to the log file."

// Error is an error with a user-facing message.
type Error struct {
	Code       Code
	Message    string
	RetryAfter time.Duration // only for RateLimited; 0 when unknown
	Err        error
}

// New returns an Error. err may be nil.
func New(code Code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

func (e *Error) Error() string {
	if e.Err == nil {
		return string(e.Code) + ": " + e.Message
	}
	return string(e.Code) + ": " + e.Message + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Public is the form of an error sent to the frontend. It never
// contains the underlying error.
type Public struct {
	Code              Code   `json:"code"`
	Message           string `json:"message"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
}

// ToPublic converts any error into its frontend form. Errors that are not
// an *Error anywhere in their chain become a generic internal error, so
// unexpected details never reach the UI.
func ToPublic(err error) Public {
	if e, ok := errors.AsType[*Error](err); ok {
		return Public{
			Code:              e.Code,
			Message:           e.Message,
			RetryAfterSeconds: int((e.RetryAfter + time.Second - 1) / time.Second), // round up
		}
	}
	return Public{Code: Internal, Message: internalMessage}
}

// InternalError wraps err as an unexpected failure.
func InternalError(err error) *Error {
	return New(Internal, internalMessage, err)
}
