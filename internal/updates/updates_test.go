package updates

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/settings"
)

type server struct {
	status   int
	body     string
	requests int
	agent    string
}

func newChecker(t *testing.T) (*Checker, *server, *time.Time) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := &server{status: http.StatusOK, body: `{"tag_name":"v0.9.1","html_url":"https://evil.example"}`}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests++
		s.agent = r.UserAgent()
		w.WriteHeader(s.status)
		io.WriteString(w, s.body)
	}))
	t.Cleanup(srv.Close)
	now := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	c := &Checker{
		Client: srv.Client(), UserAgent: "Kisetsu/test", Version: "0.9.0", ReleaseURL: "https://example.org/kisetsu",
		URL: srv.URL, Settings: settings.NewStore(db), Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return now },
	}
	return c, s, &now
}

func check(t *testing.T, c *Checker) State {
	t.Helper()
	if err := c.CheckIfDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, err := c.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestFindsANewerVersionOnceADay(t *testing.T) {
	c, s, now := newChecker(t)
	if st, err := c.State(context.Background()); err != nil || !st.Enabled || st.Available != "" || s.requests != 0 {
		t.Errorf("before: %+v, %v, %d requests", st, err, s.requests)
	}
	if st := check(t, c); st.Available != "0.9.1" || !st.CheckedAt.Equal(*now) || s.agent != "Kisetsu/test" {
		t.Errorf("state = %+v, agent %q", st, s.agent)
	}
	if got := c.PageURL("0.9.1"); got != "https://example.org/kisetsu/releases/tag/v0.9.1" {
		t.Errorf("page = %q", got)
	}
	*now = now.Add(23 * time.Hour)
	check(t, c)
	if s.requests != 1 {
		t.Errorf("%d requests within a day", s.requests)
	}
	*now = now.Add(time.Hour)
	check(t, c)
	if s.requests != 2 {
		t.Errorf("%d requests after a day", s.requests)
	}
	// The clock went back: not a reason to wait until it catches up.
	*now = now.Add(-40 * 24 * time.Hour)
	check(t, c)
	if s.requests != 3 {
		t.Errorf("%d requests with the clock set back", s.requests)
	}
	// After updating, there is nothing to tell.
	c.Version = "0.9.1"
	if st := check(t, c); st.Available != "" {
		t.Errorf("available after updating: %+v", st)
	}
}

func TestOffMeansNoRequests(t *testing.T) {
	c, s, now := newChecker(t)
	check(t, c)
	if err := c.SetEnabled(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(48 * time.Hour)
	if st := check(t, c); st.Enabled || st.Available != "" || s.requests != 1 {
		t.Errorf("off: %+v, %d requests", st, s.requests)
	}
	// An unreadable choice counts as off.
	if err := c.Settings.Save(context.Background(), prefsKey, "x"); err != nil {
		t.Fatal(err)
	}
	if err := c.CheckIfDue(context.Background()); err == nil || s.requests != 1 {
		t.Errorf("unreadable: %v, %d requests", err, s.requests)
	}
	if err := c.SetEnabled(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if st := check(t, c); st.Available != "0.9.1" || s.requests != 2 {
		t.Errorf("on again: %+v, %d requests", st, s.requests)
	}
}

func TestDevelopmentBuildsWithoutAVersionDoNotAsk(t *testing.T) {
	c, s, _ := newChecker(t)
	c.Version = "dev"
	if st := check(t, c); st.Available != "" || s.requests != 0 {
		t.Errorf("%+v, %d requests", st, s.requests)
	}
}

// A failed check keeps the last answer and is tried again.
func TestFailedChecks(t *testing.T) {
	for name, s2 := range map[string]server{
		"no release":  {status: 404, body: `{"message":"Not Found"}`},
		"limit":       {status: 403},
		"not JSON":    {status: 200, body: "<html>"},
		"no tag":      {status: 200, body: `{}`},
		"bad tag":     {status: 200, body: `{"tag_name":"v1.0.0/../../evil"}`},
		"without v":   {status: 200, body: `{"tag_name":"1.0.0"}`},
		"development": {status: 200, body: `{"tag_name":"v1.0.0~dev.1"}`},
	} {
		t.Run(name, func(t *testing.T) {
			c, s, now := newChecker(t)
			check(t, c)
			*now = now.Add(25 * time.Hour)
			s.status, s.body = s2.status, s2.body
			for range 2 {
				if err := c.CheckIfDue(context.Background()); err == nil {
					t.Error("no error")
				}
			}
			if st, _ := c.State(context.Background()); st.Available != "0.9.1" || s.requests != 3 {
				t.Errorf("%+v, %d requests", st, s.requests)
			}
		})
	}
}

func TestRunStops(t *testing.T) {
	c, _, _ := newChecker(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Run(ctx, 0, cancel)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("did not stop")
	}
	if st, _ := c.State(context.Background()); st.Available != "0.9.1" {
		t.Errorf("state = %+v", st)
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.9.1", "0.9.0", true}, {"0.10.0", "0.9.9", true}, {"1.0.0", "0.99.99", true},
		{"0.9.0", "0.9.0", false}, {"0.8.1", "0.9.0", false},
		{"0.9.0", "0.9.0~dev.116.gabc.dirty.2026", true}, {"0.8.1", "0.9.0~dev.1", false},
		{"0.9.0~dev.2", "0.9.0~dev.1", false},
		{"", "0.9.0", false}, {"0.9.1", "dev", false}, {"0.9", "0.8.0", false}, {"99999999.0.0", "0.9.0", false},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}
