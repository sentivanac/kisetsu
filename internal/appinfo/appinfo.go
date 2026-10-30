// Package appinfo describes the running Kisetsu build.
package appinfo

import "runtime"

// Name is the user-facing application name.
const Name = "Kisetsu"

// Version is set at build time, for example:
//
//	-ldflags "-X github.com/sentivanac/kisetsu/internal/appinfo.Version=0.1.0"
var Version = "dev"

// Info is the build information exposed to the frontend.
type Info struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// Get returns information about the current build and platform.
func Get() Info {
	return Info{
		Name:      Name,
		Version:   Version,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}
