package providers

import (
	"errors"
	"fmt"
	"time"
)

// Kinds of provider failure. Check them with errors.Is.
var (
	// ErrInvalidQuery means the provider rejected the query.
	ErrInvalidQuery = errors.New("invalid query")
	// ErrUnavailable means the provider could not be reached or failed
	// (network error, timeout, server error, unexpected response).
	ErrUnavailable = errors.New("provider unavailable")
	// ErrRateLimited means too many requests were sent; see RetryAfter.
	ErrRateLimited = errors.New("rate limited")
)

// Error is a failure reported by a provider.
type Error struct {
	Provider string // provider ID
	Kind     error  // ErrInvalidQuery, ErrUnavailable or ErrRateLimited
	// RetryAfter is how long to wait before retrying; 0 when unknown.
	RetryAfter time.Duration
	Err        error // underlying cause; may be nil
}

func (e *Error) Error() string {
	msg := e.Provider + ": " + e.Kind.Error()
	if e.RetryAfter > 0 {
		msg += fmt.Sprintf(" (retry after %s)", e.RetryAfter)
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

// Unwrap exposes both the kind and the cause to errors.Is and errors.As.
func (e *Error) Unwrap() []error {
	if e.Err == nil {
		return []error{e.Kind}
	}
	return []error{e.Kind, e.Err}
}
