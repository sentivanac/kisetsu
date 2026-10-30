package config

import (
	"os"
	"path/filepath"
	"testing"
)

func envFrom(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestResolvePathsDefaults(t *testing.T) {
	got, err := ResolvePaths(envFrom(nil), "/home/user")
	if err != nil {
		t.Fatal(err)
	}

	want := Paths{
		Config: "/home/user/.config/kisetsu",
		Data:   "/home/user/.local/share/kisetsu",
		Cache:  "/home/user/.cache/kisetsu",
		State:  "/home/user/.local/state/kisetsu",
	}
	if got != want {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestResolvePathsXDGOverrides(t *testing.T) {
	env := envFrom(map[string]string{
		"XDG_CONFIG_HOME": "/xdg/config",
		"XDG_DATA_HOME":   "/xdg/data",
		"XDG_CACHE_HOME":  "/xdg/cache",
		"XDG_STATE_HOME":  "/xdg/state",
	})

	got, err := ResolvePaths(env, "/home/user")
	if err != nil {
		t.Fatal(err)
	}

	want := Paths{
		Config: "/xdg/config/kisetsu",
		Data:   "/xdg/data/kisetsu",
		Cache:  "/xdg/cache/kisetsu",
		State:  "/xdg/state/kisetsu",
	}
	if got != want {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestResolvePathsIgnoresRelativeXDG(t *testing.T) {
	got, err := ResolvePaths(envFrom(map[string]string{"XDG_CONFIG_HOME": "relative/dir"}), "/home/user")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/home/user/.config/kisetsu"; got.Config != want {
		t.Errorf("Config = %q, want %q", got.Config, want)
	}
}

func TestDefaultPathsReadsEnvironment(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/env/config")

	got, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	if want := "/env/config/kisetsu"; got.Config != want {
		t.Errorf("Config = %q, want %q", got.Config, want)
	}
}

func TestResolvePathsRejectsRelativeHome(t *testing.T) {
	if _, err := ResolvePaths(envFrom(nil), "home/user"); err == nil {
		t.Error("expected an error for a relative home directory")
	}
}

func TestEnsureCreatesPrivateDirs(t *testing.T) {
	paths, err := ResolvePaths(envFrom(nil), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{paths.Config, paths.Data, paths.Cache, paths.State} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if perm := info.Mode().Perm(); perm != dirPerm {
			t.Errorf("%s has permissions %o, want %o", dir, perm, dirPerm)
		}
	}
}

func TestEnsureKeepsExistingFiles(t *testing.T) {
	paths, err := ResolvePaths(envFrom(nil), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(paths.Data, "user-file")
	if err := os.WriteFile(file, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "keep me" {
		t.Errorf("file content changed to %q", content)
	}
}
