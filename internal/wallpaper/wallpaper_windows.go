//go:build windows

package wallpaper

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	spiGetDeskWallpaper = 0x0073
	spiSetDeskWallpaper = 0x0014
	spifUpdateIniFile   = 0x01 // keep it after sign-out
	spifSendChange      = 0x02 // tell open programs (Explorer)

	// maxPath is the buffer for reading the wallpaper back (long paths
	// included).
	maxPath = 32768
)

var systemParametersInfo = windows.NewLazySystemDLL("user32.dll").NewProc("SystemParametersInfoW")

// SPICall is SystemParametersInfoW; tests replace it, so they never change
// the real wallpaper.
type SPICall func(action, uiParam uint32, pvParam unsafe.Pointer, winIni uint32) error

func callSystemParametersInfo(action, uiParam uint32, pvParam unsafe.Pointer, winIni uint32) error {
	r, _, err := systemParametersInfo.Call(uintptr(action), uintptr(uiParam), uintptr(pvParam), uintptr(winIni))
	if r == 0 {
		return fmt.Errorf("SystemParametersInfo(%#x): %w", action, err)
	}
	return nil
}

// Setter sets wallpapers on Windows, through the Windows API. The picture
// position (Fill, Fit, Span, ...) stays as the user chose it in Settings.
type Setter struct {
	// Direct is accepted for the same calls as on Linux; there is no
	// Variety on Windows.
	Direct bool
	// Call is SystemParametersInfoW (nil means the real one).
	Call SPICall
}

// New returns a Setter.
func New() *Setter {
	return &Setter{}
}

// Set makes the image at path the wallpaper and reads it back to confirm.
// path must be absolute.
func (s *Setter) Set(ctx context.Context, path string) (Method, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("wallpaper path %q is not absolute", path)
	}
	call := s.Call
	if call == nil {
		call = callSystemParametersInfo
	}
	path = filepath.Clean(path)
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return Windows, err
	}
	if err := call(spiSetDeskWallpaper, 0, unsafe.Pointer(p), spifUpdateIniFile|spifSendChange); err != nil {
		return Windows, err
	}

	buf := make([]uint16, maxPath)
	if err := call(spiGetDeskWallpaper, uint32(len(buf)), unsafe.Pointer(&buf[0]), 0); err != nil {
		return Windows, err
	}
	// Windows compares paths without regard to case.
	if got := syscall.UTF16ToString(buf); !strings.EqualFold(got, path) {
		return Windows, fmt.Errorf("%w: the wallpaper is %q", ErrNotApplied, got)
	}
	return Windows, nil
}

// VarietyRunning is always false: Variety does not run on Windows.
func (s *Setter) VarietyRunning() bool { return false }
