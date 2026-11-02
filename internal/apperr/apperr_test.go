package apperr

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestToPublicKeepsCodeAndMessage(t *testing.T) {
	err := New(Unavailable, "Database is not available.", errors.New("disk I/O error at /secret/path"))

	got := ToPublic(err)
	want := PublicError{Code: Unavailable, Message: "Database is not available."}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestToPublicRoundsRetryAfterUp(t *testing.T) {
	err := &Error{Code: RateLimited, Message: "Too many searches.", RetryAfter: 1500 * time.Millisecond}

	if got := ToPublic(err).RetryAfterSeconds; got != 2 {
		t.Errorf("RetryAfterSeconds = %d, want 2", got)
	}
	if got := ToPublic(New(RateLimited, "Too many searches.", nil)).RetryAfterSeconds; got != 0 {
		t.Errorf("RetryAfterSeconds without RetryAfter = %d, want 0", got)
	}
}

func TestToPublicFindsWrappedError(t *testing.T) {
	err := fmt.Errorf("load status: %w", New(Unavailable, "Database is not available.", nil))

	if got := ToPublic(err).Code; got != Unavailable {
		t.Errorf("Code = %q, want %q", got, Unavailable)
	}
}

func TestToPublicHidesUnknownErrors(t *testing.T) {
	got := ToPublic(errors.New("sql: no such table: images"))

	if got.Code != Internal {
		t.Errorf("Code = %q, want %q", got.Code, Internal)
	}
	if strings.Contains(got.Message, "sql") {
		t.Errorf("internal details leaked into message: %q", got.Message)
	}
}

func TestErrorKeepsCauseForLogs(t *testing.T) {
	cause := errors.New("disk full")
	err := InternalError(cause)

	if !errors.Is(err, cause) {
		t.Error("cause is not reachable with errors.Is")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("Error() = %q, want it to include the cause", err.Error())
	}
}
