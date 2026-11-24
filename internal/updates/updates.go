// Package updates looks for a newer Kisetsu release on GitHub, at most once
// a day, and remembers the answer (ADR 0056). It only tells: nothing is
// downloaded or installed.
package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/sentivanac/kisetsu/internal/settings"
)

const (
	// LatestURL is GitHub's API for the newest published release; drafts
	// and pre-releases are never returned.
	LatestURL = "https://api.github.com/repos/sentivanac/kisetsu/releases/latest"
	// Every is how long an answer is good for.
	Every = 24 * time.Hour
	// Tick is how often Run asks whether a check is due; after a failure
	// (offline), the next tick tries again.
	Tick = time.Hour

	prefsKey  = "updates"
	resultKey = "updates.result"
	timeout   = 20 * time.Second
	maxBody   = 1 << 20
)

// ErrResponse means GitHub's answer was not usable.
var ErrResponse = errors.New("unusable answer about the latest release")

type prefs struct {
	// Disabled, so that checking is on unless it was turned off.
	Disabled bool `json:"disabled"`
}

type result struct {
	CheckedAt time.Time `json:"checkedAt"`
	Latest    string    `json:"latest"` // such as "0.9.1"
}

// State is what is known about updates.
type State struct {
	Enabled   bool
	CheckedAt time.Time // zero: never
	// Available is the newer version, or "".
	Available string
}

// Checker checks for updates.
type Checker struct {
	Client     *http.Client
	UserAgent  string
	Version    string // the running version
	ReleaseURL string // the project's page, which release pages are under
	URL        string // "" means LatestURL
	Settings   *settings.Store
	Log        *slog.Logger
	Now        func() time.Time // nil means time.Now
}

func (c *Checker) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Checker) load(ctx context.Context) (p prefs, r result, err error) {
	if _, err := c.Settings.Load(ctx, prefsKey, &p); err != nil {
		// Unreadable: do not contact GitHub against a choice that may have
		// been made.
		return prefs{Disabled: true}, r, err
	}
	if _, err := c.Settings.Load(ctx, resultKey, &r); err != nil && !errors.Is(err, settings.ErrInvalidValue) {
		return p, result{}, err
	} else if err != nil {
		r = result{}
	}
	return p, r, nil
}

// State returns what the last check found. It does not use the network.
func (c *Checker) State(ctx context.Context) (State, error) {
	p, r, err := c.load(ctx)
	if err != nil {
		return State{}, err
	}
	st := State{Enabled: !p.Disabled, CheckedAt: r.CheckedAt}
	if st.Enabled && Newer(r.Latest, c.Version) {
		st.Available = r.Latest
	}
	return st, nil
}

// PageURL is the page of a release.
func (c *Checker) PageURL(version string) string {
	return c.ReleaseURL + "/releases/tag/v" + version
}

// SetEnabled turns checking on or off.
func (c *Checker) SetEnabled(ctx context.Context, on bool) error {
	return c.Settings.Save(ctx, prefsKey, prefs{Disabled: !on})
}

// CheckIfDue asks GitHub when checking is on and the last answer is older
// than Every. Versions that are not releases ("dev") never ask.
func (c *Checker) CheckIfDue(ctx context.Context) error {
	if _, ok := parse(c.Version); !ok {
		return nil
	}
	p, r, err := c.load(ctx)
	if err != nil || p.Disabled {
		return err
	}
	now := c.now()
	// An answer from the future: the clock was wrong then, or is now.
	if age := now.Sub(r.CheckedAt); age >= 0 && age < Every {
		return nil
	}
	latest, err := c.fetch(ctx)
	if err != nil {
		return err
	}
	return c.Settings.Save(ctx, resultKey, result{CheckedAt: now.UTC(), Latest: latest})
}

func (c *Checker) fetch(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	url := c.URL
	if url == "" {
		url = LatestURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: HTTP %d", ErrResponse, resp.StatusCode)
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&release); err != nil {
		return "", fmt.Errorf("%w: %v", ErrResponse, err)
	}
	// Only plain versions are stored and shown; the page's address is made
	// from them, never taken from the answer.
	v, ok := parse(release.Tag)
	if !ok || v.dev || release.Tag[0] != 'v' {
		return "", fmt.Errorf("%w: tag %.40q", ErrResponse, release.Tag)
	}
	return fmt.Sprintf("%d.%d.%d", v.n[0], v.n[1], v.n[2]), nil
}

// Run checks after delay and then every Tick, until ctx ends. changed, if
// not nil, is called after every check.
func (c *Checker) Run(ctx context.Context, delay time.Duration, changed func()) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if err := c.CheckIfDue(ctx); err != nil && ctx.Err() == nil {
			c.Log.Info("no answer about a newer version", "err", err)
		}
		if changed != nil {
			changed()
		}
		timer.Reset(Tick)
	}
}

type version struct {
	n   [3]int
	dev bool // a development build before the release
}

var versionPattern = regexp.MustCompile(`^v?(\d{1,6})\.(\d{1,6})\.(\d{1,6})(~.+)?$`)

func parse(s string) (v version, ok bool) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return version{}, false
	}
	for i := range v.n {
		v.n[i], _ = strconv.Atoi(m[i+1])
	}
	v.dev = m[4] != ""
	return v, true
}

// Newer reports whether version a is newer than b. A development build
// ("0.9.0~dev.1") is older than its release. Anything that is not a version
// is never newer, nor older.
func Newer(a, b string) bool {
	va, ok := parse(a)
	vb, okb := parse(b)
	if !ok || !okb {
		return false
	}
	for i := range va.n {
		if va.n[i] != vb.n[i] {
			return va.n[i] > vb.n[i]
		}
	}
	return vb.dev && !va.dev
}
