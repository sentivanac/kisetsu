package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/rotation"
)

// testCore opens a core in temporary directories (XDG, %LocalAppData%), with a fake wallpaper
// setter, so tests never touch the real data or desktop.
func testCore(t *testing.T, imageCount int) (*core, *rotation.Rotator, *fakeWallpaper) {
	t.Helper()
	dir := t.TempDir()
	for _, v := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "LOCALAPPDATA"} {
		t.Setenv(v, filepath.Join(dir, v))
	}
	c, err := openCore(context.Background(), "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)

	for n := range imageCount {
		rel := fmt.Sprintf("images/test/%d.jpg", n)
		if err := os.MkdirAll(filepath.Join(c.paths.Data, "images", "test"), 0o700); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(c.paths.Data, rel), []byte("image"), 0o600)
		_, err := c.lib.Add(context.Background(), library.Entry{
			Image: images.Image{
				Provider: "test", ProviderID: fmt.Sprint(n), PageURL: "https://x", DownloadURL: "https://x",
				Width: 1, Height: 1, MIMEType: "image/jpeg", FileSize: 5,
				License: images.License{Status: images.LicenseUnknown},
			},
			FilePath: rel, SHA256: fmt.Sprintf("%064d", n),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	r := c.rotator(nil)
	fake := &fakeWallpaper{}
	r.Setter = fake
	return c, r, fake
}

func TestRotateChangesTheWallpaper(t *testing.T) {
	c, r, fake := testCore(t, 2)
	var out bytes.Buffer

	if code := rotate(context.Background(), c, r, false, &out); code != 0 {
		t.Fatalf("exit %d, output %q", code, out.String())
	}
	if !strings.HasPrefix(fake.path, filepath.Join(c.paths.Data, "images")) {
		t.Errorf("setter got %q", fake.path)
	}
	if !strings.Contains(out.String(), "Wallpaper changed") {
		t.Errorf("output %q", out.String())
	}
	logData, _ := os.ReadFile(filepath.Join(c.paths.State, "kisetsu.log"))
	if !strings.Contains(string(logData), "wallpaper rotated") || !strings.Contains(string(logData), "component=test") {
		t.Errorf("log does not record the rotation:\n%s", logData)
	}
}

func TestScheduledRotateRespectsTheSetting(t *testing.T) {
	c, r, fake := testCore(t, 2)
	ctx := context.Background()
	var out bytes.Buffer

	if code := rotate(ctx, c, r, true, &out); code != 0 || fake.path != "" {
		t.Fatalf("disabled rotation: exit %d, setter %q", code, fake.path)
	}
	if !strings.Contains(out.String(), "turned off") {
		t.Errorf("output %q", out.String())
	}

	rotation.SaveSettings(ctx, r.Settings, rotation.Settings{Enabled: true, Interval: rotation.Hourly, Order: rotation.Random})
	if code := rotate(ctx, c, r, true, &out); code != 0 || fake.path == "" {
		t.Errorf("enabled rotation: exit %d, setter %q", code, fake.path)
	}
}

func TestRotateSkipsWhileAnotherRuns(t *testing.T) {
	c, r, fake := testCore(t, 2)
	unlock, err := rotation.Lock(c.rotationLockPath())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	var out bytes.Buffer

	if code := rotate(context.Background(), c, r, false, &out); code != 0 || fake.path != "" {
		t.Errorf("exit %d, setter %q; want 0 and no change", code, fake.path)
	}
}

func TestRotateWithEmptyLibrary(t *testing.T) {
	c, r, fake := testCore(t, 0)
	var out bytes.Buffer

	if code := rotate(context.Background(), c, r, false, &out); code != 0 || fake.path != "" {
		t.Errorf("exit %d, setter %q", code, fake.path)
	}
	if !strings.Contains(out.String(), "no images") {
		t.Errorf("output %q", out.String())
	}
}

func TestRotateReportsFailure(t *testing.T) {
	c, r, fake := testCore(t, 1)
	fake.err = fmt.Errorf("gsettings: exit status 1")

	if code := rotate(context.Background(), c, r, false, &bytes.Buffer{}); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
}

func TestRunCommandUsage(t *testing.T) {
	if code := runCommand("help", nil); code != 0 {
		t.Errorf("help: exit %d", code)
	}
	if code := runCommand("version", nil); code != 0 {
		t.Errorf("version: exit %d", code)
	}
	if code := runCommand("frobnicate", nil); code != 2 {
		t.Errorf("unknown command: exit %d, want 2", code)
	}
	if code := runCommand("rotate", []string{"--nope"}); code != 2 {
		t.Errorf("unknown flag: exit %d, want 2", code)
	}
	if code := runCommand("rotate", []string{"extra"}); code != 2 {
		t.Errorf("extra argument: exit %d, want 2", code)
	}
}

func TestFetchCommandArguments(t *testing.T) {
	for _, args := range [][]string{
		nil,                        // no name
		{"Lakes", "--count", "0"},  // count too small
		{"Lakes", "--count", "99"}, // count too large
		{"Lakes", "extra"},         // extra argument
	} {
		if code := runCommand("fetch", args); code != 2 {
			t.Errorf("fetch %q: exit %d, want 2", args, code)
		}
	}
}

// Every command in the help text is described in the manual page.
func TestManualPageListsEveryCommand(t *testing.T) {
	page, err := os.ReadFile("docs/man/kisetsu.1")
	if err != nil {
		t.Fatal(err)
	}
	commands := 0
	for _, line := range strings.Split(usage, "\n") {
		fields := strings.Fields(line)
		if !strings.HasPrefix(line, "  ") || len(fields) == 0 || fields[0] != strings.ToLower(fields[0]) || strings.HasPrefix(line, "   ") {
			continue
		}
		commands++
		if !strings.Contains(string(page), ".B kisetsu "+fields[0]) {
			t.Errorf("manual page does not list %q", fields[0])
		}
	}
	if commands < 4 {
		t.Fatalf("found only %d commands in the help text; the test no longer reads it correctly", commands)
	}
}
