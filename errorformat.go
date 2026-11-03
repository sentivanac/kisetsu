package main

import (
	"encoding/json"
	"log/slog"

	"github.com/sentivanac/kisetsu/internal/apperr"
)

// formatError returns the Wails ErrorFormatter. It logs the full error and
// sends only its user-safe part to the frontend, as JSON text.
//
// Text, not an object: the Wails v2 runtime rejects the call's promise
// with new Error(value). A string survives as the Error's message; an
// object becomes "[object Object]" and its code and message are lost.
// frontend/src/lib/backend.ts reads the JSON back (ADR 0004).
func formatError(log *slog.Logger) func(error) any {
	return func(err error) any {
		public := apperr.ToPublic(err)
		if public.Code == apperr.Canceled {
			log.Debug("frontend call canceled", "err", err)
		} else {
			log.Error("frontend call failed", "err", err)
		}
		data, err := json.Marshal(public)
		if err != nil { // cannot happen for this struct; keep the UI working anyway
			return `{"code":"internal","message":"Something went wrong. Details were written to the log file."}`
		}
		return string(data)
	}
}
