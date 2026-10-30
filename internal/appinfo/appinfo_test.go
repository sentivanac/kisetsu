package appinfo

import (
	"runtime"
	"testing"
)

func TestGet(t *testing.T) {
	info := Get()

	if info.Name != Name {
		t.Errorf("Name = %q, want %q", info.Name, Name)
	}
	if info.Version != Version {
		t.Errorf("Version = %q, want %q", info.Version, Version)
	}
	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", info.GoVersion, runtime.Version())
	}
	if want := runtime.GOOS + "/" + runtime.GOARCH; info.Platform != want {
		t.Errorf("Platform = %q, want %q", info.Platform, want)
	}
}

func TestUserAgent(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })

	Version = "0.1.0"
	if got, want := UserAgent(), "Kisetsu/0.1.0 (+https://github.com/sentivanac/kisetsu)"; got != want {
		t.Errorf("UserAgent() = %q, want %q", got, want)
	}
}

func TestGetUsesBuildVersion(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })

	Version = "1.2.3"
	if got := Get().Version; got != "1.2.3" {
		t.Errorf("Version = %q, want %q", got, "1.2.3")
	}
}
