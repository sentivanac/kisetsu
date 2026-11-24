package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateState(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, `{"tag_name":"v0.9.1"}`)
	}))
	defer srv.Close()
	app := newTestApp(t, &stubProvider{})
	app.updates.URL, app.updates.Version = srv.URL, "0.9.0"

	if st, err := app.GetUpdateState(); err != nil || !st.Enabled || st.Available != "" || st.CheckedAt != "" || requests != 0 {
		t.Errorf("at first: %+v, %v, %d requests", st, err, requests)
	}
	st, err := app.SetUpdateCheck(true)
	if err != nil || st.Available != "0.9.1" || st.URL != "https://github.com/sentivanac/kisetsu/releases/tag/v0.9.1" || st.CheckedAt == "" {
		t.Errorf("on: %+v, %v", st, err)
	}
	st, err = app.SetUpdateCheck(false)
	if err != nil || st.Enabled || st.Available != "" || st.URL != "" || requests != 1 {
		t.Errorf("off: %+v, %v, %d requests", st, err, requests)
	}
	// Being offline does not prevent turning it on.
	srv.Close()
	app.db.Exec(`DELETE FROM settings WHERE key = 'updates.result'`)
	if st, err := app.SetUpdateCheck(true); err != nil || !st.Enabled {
		t.Errorf("offline: %+v, %v", st, err)
	}
}
