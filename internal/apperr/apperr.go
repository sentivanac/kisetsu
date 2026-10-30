// Package apperr defines errors that are safe to show to users.
//
// An Error carries a stable code the frontend can act on, a message
// written for users, and the underlying error, which is only logged.
package apperr

import "errors"

// Code identifies a kind of failure. Codes are part of the contract with
// the frontend: add new ones when needed, but do not rename existing ones.
type Code string

const (
	// Internal is an unexpected failure, usually a bug.
	Internal Code = "internal"
	// Unavailable means a required local resource, such as the database,
	// cannot be used right now.
	Unavailable Code = "unavailable"
)

const internalMessage = "Something went wrong. Details were written to the log file."

// Error is an error with a user-facing message.
type Error struct {
	Code    Code
	Message string
	Err     error
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
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

// ToPublic converts any error into its frontend form. Errors that are not
// an *Error anywhere in their chain become a generic internal error, so
// unexpected details never reach the UI.
func ToPublic(err error) Public {
	if e, ok := errors.AsType[*Error](err); ok {
		return Public{Code: e.Code, Message: e.Message}
	}
	return Public{Code: Internal, Message: internalMessage}
}

// InternalError wraps err as an unexpected failure.
func InternalError(err error) *Error {
	return New(Internal, internalMessage, err)
}
