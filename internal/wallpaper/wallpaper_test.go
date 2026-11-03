package wallpaper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeRunner records commands and acts like a gsettings store. It never
// runs anything, so tests cannot change the real wallpaper.
type fakeRunner struct {
	calls    [][]string
	settings map[string]string // "schema key" -> printed value
	missing  map[string]bool   // commands LookPath does not find
	fail     string            // command whose Run fails
	ignore   bool              // gsettings set "succeeds" without changing anything
	// session is what loginctl reports: the graphical session's Desktop
	// and Type, e.g. "ubuntu x11"; empty means no graphical session.
	// loginctl only reads, so its calls are not recorded.
	session string
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if f.missing[name] {
		return "", errors.New("not found")
	}
	return "/usr/bin/" + name, nil
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name == "loginctl" {
		switch {
		case f.session == "":
			return nil, nil
		case args[0] == "show-user":
			return []byte("c2\n"), nil
		default:
			desktop, kind, _ := strings.Cut(f.session, " ")
			return []byte("Type=" + kind + "\nDesktop=" + desktop + "\n"), nil // not in the order asked for
		}
	}
	f.calls = append(f.calls, append([]string{name}, args...))
	if name == f.fail {
		return nil, errors.New(name + " failed")
	}
	if name == "gsettings" && len(args) >= 3 {
		key := args[1] + " " + args[2]
		switch args[0] {
		case "set":
			if !f.ignore {
				f.settings[key] = args[3]
			}
		case "get":
			return []byte(f.settings[key] + "\n"), nil
		}
	}
	return nil, nil
}

// fakeProc creates a /proc-like directory with the given process names.
func fakeProc(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for i, name := range names {
		pid := filepath.Join(dir, string(rune('1'+i))+"00")
		os.MkdirAll(pid, 0o755)
		os.WriteFile(filepath.Join(pid, "comm"), []byte(name+"\n"), 0o644)
	}
	os.MkdirAll(filepath.Join(dir, "self"), 0o755) // non-numeric entries are skipped
	return dir
}

func newSetter(t *testing.T, desktop string, procs ...string) (*Setter, *fakeRunner) {
	t.Helper()
	run := &fakeRunner{settings: map[string]string{}, missing: map[string]bool{}}
	env := map[string]string{"XDG_CURRENT_DESKTOP": desktop}
	return &Setter{
		Run:     run,
		Getenv:  func(k string) string { return env[k] },
		ProcDir: fakeProc(t, procs...),
		UID:     os.Getuid(), // the fake /proc entries belong to the test user
	}, run
}

const image = "/home/user/.local/share/kisetsu/images/wallhaven/abc.jpg"

func TestUsesVarietyWhenRunning(t *testing.T) {
	s, run := newSetter(t, "MATE", "bash", "variety")

	method, err := s.Set(context.Background(), image)
	if err != nil {
		t.Fatal(err)
	}
	if method != Variety {
		t.Errorf("method = %q", method)
	}
	if want := [][]string{{"variety", "--set", image}}; !slices.EqualFunc(run.calls, want, slices.Equal) {
		t.Errorf("calls = %q, want %q", run.calls, want)
	}
}

func TestIgnoresOtherUsersVariety(t *testing.T) {
	s, run := newSetter(t, "MATE", "variety")
	s.UID = os.Getuid() + 1 // the fake process belongs to someone else

	method, err := s.Set(context.Background(), image)
	if err != nil || method != MATE {
		t.Fatalf("method %q, err %v", method, err)
	}
	if run.calls[0][0] != "gsettings" {
		t.Errorf("calls = %q", run.calls)
	}
}

func TestSetsMATEBackground(t *testing.T) {
	s, run := newSetter(t, "MATE", "bash")

	method, err := s.Set(context.Background(), image)
	if err != nil || method != MATE {
		t.Fatalf("method %q, err %v", method, err)
	}
	want := [][]string{
		{"gsettings", "set", "org.mate.background", "picture-filename", "'" + image + "'"},
		{"gsettings", "get", "org.mate.background", "picture-filename"},
	}
	if !slices.EqualFunc(run.calls, want, slices.Equal) {
		t.Errorf("calls = %q\nwant    %q", run.calls, want)
	}
}

func TestSetsGNOMELightAndDarkPictures(t *testing.T) {
	s, run := newSetter(t, "ubuntu:GNOME")
	path := "/home/user/My Pictures/it's/abc.jpg"

	method, err := s.Set(context.Background(), path)
	if err != nil || method != GNOME {
		t.Fatalf("method %q, err %v", method, err)
	}
	uri := `'file:///home/user/My%20Pictures/it%27s/abc.jpg'`
	for _, key := range []string{"picture-uri", "picture-uri-dark"} {
		if got := run.settings["org.gnome.desktop.background "+key]; got != uri {
			t.Errorf("%s = %s, want %s", key, got, uri)
		}
	}
}

func TestSetsCinnamonPicture(t *testing.T) {
	s, run := newSetter(t, "X-Cinnamon")

	method, err := s.Set(context.Background(), "/home/user/a b.jpg")
	if err != nil || method != Cinnamon {
		t.Fatalf("method %q, err %v", method, err)
	}
	if got := run.settings["org.cinnamon.desktop.background picture-uri"]; got != `'file:///home/user/a%20b.jpg'` {
		t.Errorf("picture-uri = %s", got)
	}
	for key := range run.settings {
		if strings.HasPrefix(key, "org.gnome") {
			t.Errorf("GNOME setting changed on Cinnamon: %s", key)
		}
	}
}

func TestQuoteGVariant(t *testing.T) {
	tests := map[string]string{
		"/a/b.jpg":           `'/a/b.jpg'`,
		"/it's/a.jpg":        `'/it\'s/a.jpg'`,
		`/back\slash/a.jpg`:  `'/back\\slash/a.jpg'`,
		"/dir 'x'/$(rm).jpg": `'/dir \'x\'/$(rm).jpg'`, // no shell is involved
	}
	for in, want := range tests {
		if got := quoteGVariant(in); got != want {
			t.Errorf("quoteGVariant(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestErrors(t *testing.T) {
	ctx := context.Background()

	s, run := newSetter(t, "KDE")
	if _, err := s.Set(ctx, image); !errors.Is(err, ErrUnsupportedDesktop) {
		t.Errorf("KDE: err = %v, want ErrUnsupportedDesktop", err)
	}
	if len(run.calls) != 0 {
		t.Errorf("commands ran on an unsupported desktop: %q", run.calls)
	}

	s, _ = newSetter(t, "MATE")
	if _, err := s.Set(ctx, "relative/abc.jpg"); err == nil {
		t.Error("a relative path was accepted")
	}

	s, run = newSetter(t, "MATE")
	run.missing["gsettings"] = true
	if _, err := s.Set(ctx, image); err == nil || !strings.Contains(err.Error(), "gsettings not found") {
		t.Errorf("missing gsettings: err = %v", err)
	}

	s, run = newSetter(t, "MATE")
	run.fail = "gsettings"
	if _, err := s.Set(ctx, image); err == nil {
		t.Error("a failing gsettings was ignored")
	}

	s, run = newSetter(t, "MATE")
	run.ignore = true
	if _, err := s.Set(ctx, image); !errors.Is(err, ErrNotApplied) {
		t.Errorf("unchanged setting: err = %v, want ErrNotApplied", err)
	}

	s, run = newSetter(t, "MATE", "variety")
	run.fail = "variety"
	if method, err := s.Set(ctx, image); err == nil || method != Variety {
		t.Errorf("failing variety: method %q, err %v", method, err)
	}
}

func TestDirectSkipsVariety(t *testing.T) {
	s, run := newSetter(t, "MATE", "variety")
	s.Direct = true

	method, err := s.Set(context.Background(), image)
	if err != nil || method != MATE {
		t.Fatalf("method %q, err %v", method, err)
	}
	if run.calls[0][0] != "gsettings" {
		t.Errorf("calls = %q, want gsettings only", run.calls)
	}
	if !s.VarietyRunning() {
		t.Error("VarietyRunning = false")
	}
}

// sessionDirs creates xsessions/ and wayland-sessions/ like the user's
// system (Ubuntu 22.04 with MATE, GNOME and i3).
func sessionDirs(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"xsessions/mate.desktop":                  "[Desktop Entry]\nName=MATE\nDesktopNames=MATE\n",
		"xsessions/ubuntu.desktop":                "[Desktop Entry]\nName=Ubuntu\nDesktopNames=ubuntu:GNOME\n",
		"xsessions/i3.desktop":                    "[Desktop Entry]\nName=i3\nDesktopNames=i3\n",
		"xsessions/i3-with-shmlog.desktop":        "[Desktop Entry]\nName=i3 (with debug log)\n",
		"wayland-sessions/ubuntu-wayland.desktop": "[Desktop Entry]\nName=Ubuntu on Wayland\nDesktopNames=ubuntu;GNOME\n",
	}
	for name, content := range files {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}
	return []string{dir}
}

// The rotation timer's environment can name another desktop than the one
// the user is logged into; the active session decides.
func TestActiveSessionDecidesTheDesktop(t *testing.T) {
	tests := []struct {
		session, env string
		want         Method
		wantErr      bool
	}{
		{"ubuntu x11", "MATE", GNOME, false},         // stale MATE value, logged into GNOME
		{"ubuntu-wayland wayland", "", GNOME, false}, // Wayland session file, ";" separated
		{"mate x11", "ubuntu:GNOME", MATE, false},    // stale GNOME value, logged into MATE
		{"i3 x11", "ubuntu:GNOME", "", true},         // i3 is not supported, whatever the env says
		{"i3-with-shmlog x11", "", "", true},         // no DesktopNames: the session name
		{"cinnamon x11", "", Cinnamon, false},        // Linux Mint: no session file, loginctl name
		{"cinnamon2d x11", "", Cinnamon, false},      // Mint's session without 3D
		{"", "ubuntu:GNOME", GNOME, false},           // no graphical session found: the env
	}
	for _, tt := range tests {
		s, run := newSetter(t, tt.env)
		s.DataDirs = sessionDirs(t)
		run.session = tt.session

		method, err := s.Set(context.Background(), image)
		if tt.wantErr {
			if !errors.Is(err, ErrUnsupportedDesktop) {
				t.Errorf("%q / %q: err = %v, want ErrUnsupportedDesktop", tt.session, tt.env, err)
			}
			continue
		}
		if err != nil || method != tt.want {
			t.Errorf("%q / %q: method %q, err %v; want %q", tt.session, tt.env, method, err, tt.want)
		}
	}
}

func TestFallbackDesktopFromTheTimer(t *testing.T) {
	s, _ := newSetter(t, "")
	s.Getenv = func(k string) string { return map[string]string{FallbackEnv: "MATE"}[k] }

	if method, err := s.Set(context.Background(), image); err != nil || method != MATE {
		t.Errorf("method %q, err %v; want the fallback MATE", method, err)
	}
}

func TestExecRunner(t *testing.T) {
	if _, err := (ExecRunner{}).LookPath("sh"); err != nil {
		t.Fatal(err)
	}
	if _, err := (ExecRunner{}).LookPath("kisetsu-no-such-command"); err == nil {
		t.Error("a missing command was found")
	}
	out, err := ExecRunner{}.Run(context.Background(), "sh", "-c", "echo picture")
	if err != nil || strings.TrimSpace(string(out)) != "picture" {
		t.Errorf("output %q, err %v", out, err)
	}
	_, err = ExecRunner{}.Run(context.Background(), "sh", "-c", "echo 'No such schema' >&2; exit 1")
	if err == nil || !strings.Contains(err.Error(), "No such schema") {
		t.Errorf("error does not include stderr: %v", err)
	}
}
