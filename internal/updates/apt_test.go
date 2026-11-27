package updates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFromAPT(t *testing.T) {
	const readme = `Types: deb
URIs: https://sentivanac.github.io/kisetsu/apt
Suites: stable
Components: main
Architectures: amd64
Signed-By: /etc/apt/keyrings/kisetsu.asc
`
	for _, tc := range []struct {
		name, file, content string
		want                bool
	}{
		{"no sources", "", "", false},
		{"as in the README", "sources.list.d/kisetsu.sources", readme, true},
		{"another repository", "sources.list.d/other.sources", "Types: deb\nURIs: https://example.org/apt\nSuites: stable\n", false},
		{"turned off", "sources.list.d/kisetsu.sources", readme + "Enabled: no\n", false},
		{"in a second stanza", "sources.list.d/mixed.sources",
			"Types: deb\nURIs: https://example.org/apt\nSuites: stable\n\n" + readme, true},
		{"one-line format", "sources.list.d/kisetsu.list",
			"deb [signed-by=/etc/apt/keyrings/kisetsu.asc] https://sentivanac.github.io/kisetsu/apt stable main\n", true},
		{"commented out", "sources.list.d/kisetsu.list",
			"# deb https://sentivanac.github.io/kisetsu/apt stable main\n", false},
		{"in sources.list", "sources.list",
			"deb http://archive.ubuntu.com/ubuntu jammy main\ndeb https://sentivanac.github.io/kisetsu/apt stable main\n", true},
	} {
		dir := t.TempDir()
		if tc.file != "" {
			p := filepath.Join(dir, tc.file)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if got := FromAPT(dir); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
	if FromAPT(filepath.Join(t.TempDir(), "missing")) {
		t.Error("a missing directory lists the repository")
	}
}
