package main

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sentivanac/kisetsu/internal/appinfo"
)

const (
	// aboutArg opens the window with About shown; the tray uses it.
	aboutArg = "--about"
	// aboutEvent tells an open window to show About.
	aboutEvent = "about:show"
)

// About is what the About window shows.
type About struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Author     string `json:"author"`
	AuthorURL  string `json:"authorUrl"`
	ProjectURL string `json:"projectUrl"`
	IssuesURL  string `json:"issuesUrl"`
	License    string `json:"license"`
	LicenseURL string `json:"licenseUrl"`
	Platform   string `json:"platform"`
	GoVersion  string `json:"goVersion"`
	// HasThirdPartyLicenses: the packages installed the list of licenses
	// of the code built into Kisetsu (not in a development build).
	HasThirdPartyLicenses bool `json:"hasThirdPartyLicenses"`
}

// GetAbout returns what the About window shows.
func (a *App) GetAbout() (about About, err error) {
	defer a.recoverPanic("GetAbout", &err)
	info := appinfo.Get()
	return About{
		Name:                  info.Name,
		Version:               info.Version,
		Author:                appinfo.Author,
		AuthorURL:             appinfo.AuthorURL,
		ProjectURL:            appinfo.ProjectURL,
		IssuesURL:             appinfo.ProjectURL + "/issues",
		License:               appinfo.License,
		LicenseURL:            appinfo.ProjectURL + "/blob/main/LICENSE",
		Platform:              info.Platform,
		GoVersion:             info.GoVersion,
		HasThirdPartyLicenses: thirdPartyLicenses(a.executable) != "",
	}, nil
}

// TakeAboutRequest reports, once, whether the window was opened to show
// About.
func (a *App) TakeAboutRequest() (show bool, err error) {
	defer a.recoverPanic("TakeAboutRequest", &err)
	return a.aboutRequested.Swap(false), nil
}

// maxLicensesSize bounds the list read for the About window; it is about
// 160 KB.
const maxLicensesSize = 4 << 20

// GetThirdPartyLicenses returns the licenses of the code built into
// Kisetsu, as the packages installed them.
func (a *App) GetThirdPartyLicenses() (text string, err error) {
	defer a.recoverPanic("GetThirdPartyLicenses", &err)
	text, err = readThirdPartyLicenses(thirdPartyLicenses(a.executable))
	if err != nil {
		a.log.Warn("third-party licenses not read", "err", err)
		return "", errors.New("the list of third-party licenses could not be read")
	}
	return text, nil
}

// readThirdPartyLicenses reads the list, unpacking it when it is gzipped
// (the .deb package).
func readThirdPartyLicenses(path string) (string, error) {
	if path == "" {
		return "", errors.New("not installed")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		z, err := gzip.NewReader(f)
		if err != nil {
			return "", err
		}
		defer z.Close()
		r = z
	}
	b, err := io.ReadAll(io.LimitReader(r, maxLicensesSize+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxLicensesSize {
		return "", fmt.Errorf("%s is larger than %d bytes", path, maxLicensesSize)
	}
	return string(b), nil
}

// thirdPartyLicenses returns where the packages install the list of
// third-party licenses: next to the program on Windows, under
// /usr/share/doc in the .deb package. Empty when neither exists.
func thirdPartyLicenses(executable string) string {
	var candidates []string
	if executable != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "THIRD_PARTY_LICENSES.txt"))
	}
	candidates = append(candidates, "/usr/share/doc/kisetsu/THIRD_PARTY_LICENSES.gz")
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return p
		}
	}
	return ""
}
