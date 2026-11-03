package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/providers"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/search"
	"github.com/sentivanac/kisetsu/internal/searches"
)

// useTempXDG points every XDG directory at a temporary one, so commands
// started through runCommand never touch the user's data.
func useTempXDG(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, v := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		t.Setenv(v, filepath.Join(dir, v))
	}
}

func TestRunFetchUsage(t *testing.T) {
	for name, args := range map[string][]string{
		"no name":        nil,
		"count zero":     {"Forest", "--count", "0"},
		"count too high": {"Forest", "--count", "25"},
		"two names":      {"Forest", "Sea"},
		"unknown flag":   {"Forest", "--quick"},
	} {
		if code := runCommand("fetch", args); code != 2 {
			t.Errorf("%s: exit %d, want 2", name, code)
		}
	}
}

// The name is accepted before and after the flags; an unknown saved
// search is a usage error.
func TestRunFetchUnknownSearch(t *testing.T) {
	useTempXDG(t)
	for _, args := range [][]string{{"Nowhere"}, {"--count", "3", "Nowhere"}, {"Nowhere", "--count", "3"}} {
		if code := runCommand("fetch", args); code != 2 {
			t.Errorf("%q: exit %d, want 2", args, code)
		}
	}
}

// The whole rotate command, as the timer starts it, with an empty
// library: nothing to set, so the desktop is never touched.
func TestRunRotateWithEmptyLibrary(t *testing.T) {
	useTempXDG(t)
	for _, args := range [][]string{nil, {"--scheduled"}} {
		if code := runCommand("rotate", args); code != 0 {
			t.Errorf("%q: exit %d, want 0", args, code)
		}
	}
}

type fakeSearcher struct {
	err   error
	calls int
}

func (f *fakeSearcher) SearchMany(ctx context.Context, sources []search.SourcePage, q providers.Query) (search.MultiResult, error) {
	f.calls++
	return search.MultiResult{}, f.err
}

func (f *fakeSearcher) Providers() []search.ProviderInfo {
	return []search.ProviderInfo{{ID: "fake", Name: "Fake"}}
}

func newFetchTest(t *testing.T, err error) (*core, *fakeSearcher, func(string) (int, string)) {
	t.Helper()
	c, _, _ := testCore(t, 0)
	if _, err := searches.NewStore(c.db).Create(context.Background(), "Forest", searches.Query{ProviderIDs: []string{"fake"}}); err != nil {
		t.Fatal(err)
	}
	searcher := &fakeSearcher{err: err}
	f := c.fetcher(nil, nil)
	f.Search = searcher
	run := func(name string) (int, string) {
		var out bytes.Buffer
		code := fetch(context.Background(), c, f, name, 5, &out)
		return code, out.String()
	}
	return c, searcher, run
}

func TestFetchCommand(t *testing.T) {
	_, searcher, run := newFetchTest(t, nil)

	code, out := run("Forest")
	if code != 0 || !strings.Contains(out, `Fetched 0 new image(s) for "Forest"`) {
		t.Errorf("exit %d, output %q", code, out)
	}
	if searcher.calls == 0 {
		t.Error("the saved search was not run")
	}
	if code, _ := run("Desert"); code != 2 {
		t.Errorf("unknown search: exit %d, want 2", code)
	}
}

func TestFetchCommandReportsSearchFailure(t *testing.T) {
	_, _, run := newFetchTest(t, errors.New("provider down"))

	if code, _ := run("Forest"); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
}

// Fetching prunes old images, so it waits for a running rotation instead
// of racing with it.
func TestFetchCommandWaitsForRotation(t *testing.T) {
	c, searcher, run := newFetchTest(t, nil)
	unlock, err := rotation.Lock(c.rotationLockPath())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	code, out := run("Forest")
	if code != 0 || !strings.Contains(out, "in progress") || searcher.calls != 0 {
		t.Errorf("exit %d, output %q, searches %d", code, out, searcher.calls)
	}
}
