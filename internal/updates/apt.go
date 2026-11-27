package updates

import (
	"os"
	"path/filepath"
	"strings"
)

// AptDir is where the system lists its package sources.
const AptDir = "/etc/apt"

// aptRepository is the address of Kisetsu's APT repository (ADR 0057).
const aptRepository = "sentivanac.github.io/kisetsu/apt"

// FromAPT reports whether Kisetsu's APT repository is among the package
// sources in dir (normally AptDir), so new versions arrive with the
// system's updates. Entries that are commented out or disabled do not
// count. On Windows there is no such directory, and the answer is false.
func FromAPT(dir string) bool {
	if lineListed(filepath.Join(dir, "sources.list")) {
		return true
	}
	lists, _ := filepath.Glob(filepath.Join(dir, "sources.list.d", "*.list"))
	for _, f := range lists {
		if lineListed(f) {
			return true
		}
	}
	sources, _ := filepath.Glob(filepath.Join(dir, "sources.list.d", "*.sources"))
	for _, f := range sources {
		if stanzaListed(f) {
			return true
		}
	}
	return false
}

// lineListed reads a one-line-per-source file ("deb [...] URI suite ...").
func lineListed(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") && strings.Contains(line, aptRepository) {
			return true
		}
	}
	return false
}

// stanzaListed reads a deb822 file: stanzas separated by blank lines, one
// of which names the repository in URIs and is not "Enabled: no".
func stanzaListed(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for stanza := range strings.SplitSeq(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n\n") {
		var uris, enabled = false, true
		for line := range strings.SplitSeq(stanza, "\n") {
			key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
			if !ok || strings.HasPrefix(key, "#") {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "uris":
				uris = strings.Contains(value, aptRepository)
			case "enabled":
				enabled = strings.TrimSpace(strings.ToLower(value)) != "no"
			}
		}
		if uris && enabled {
			return true
		}
	}
	return false
}
