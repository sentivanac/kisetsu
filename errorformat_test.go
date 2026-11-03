package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/apperr"
)

func TestFormatErrorSendsJSONText(t *testing.T) {
	var logBuf bytes.Buffer
	format := formatError(slog.New(slog.NewTextHandler(&logBuf, nil)))

	cause := errors.New("gsettings: exit status 1")
	out, ok := format(&apperr.Error{Code: apperr.RateLimited, Message: "Too many searches.", RetryAfter: 30 * time.Second, Err: cause}).(string)
	if !ok {
		t.Fatalf("formatter returned %T; Wails needs a string to keep it", out)
	}
	var got apperr.PublicError
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %q", out)
	}
	want := apperr.PublicError{Code: apperr.RateLimited, Message: "Too many searches.", RetryAfterSeconds: 30}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if strings.Contains(out, "gsettings") {
		t.Errorf("the cause reached the frontend: %s", out)
	}
	if !strings.Contains(logBuf.String(), "gsettings") {
		t.Errorf("the cause was not logged: %s", logBuf.String())
	}
}

func TestFormatErrorHidesUnexpectedErrors(t *testing.T) {
	format := formatError(slog.New(slog.DiscardHandler))
	out := format(errors.New("open /home/user/secret: permission denied")).(string)

	if strings.Contains(out, "secret") || !strings.Contains(out, `"code":"internal"`) {
		t.Errorf("got %s", out)
	}
}
