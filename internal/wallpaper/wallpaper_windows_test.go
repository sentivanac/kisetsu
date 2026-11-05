//go:build windows

package wallpaper

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"unsafe"
)

// fakeSPI keeps the wallpaper in memory, like SystemParametersInfoW would,
// so tests never change the real one.
type fakeSPI struct {
	current string
	ignore  bool  // "succeeds" without changing anything
	fail    error // returned by every call
	winIni  uint32
}

func (f *fakeSPI) call(action, uiParam uint32, pvParam unsafe.Pointer, winIni uint32) error {
	if f.fail != nil {
		return f.fail
	}
	switch action {
	case spiSetDeskWallpaper:
		f.winIni = winIni
		if !f.ignore {
			f.current = syscall.UTF16ToString(unsafe.Slice((*uint16)(pvParam), maxPath))
		}
	case spiGetDeskWallpaper:
		buf := unsafe.Slice((*uint16)(pvParam), uiParam)
		u, _ := syscall.UTF16FromString(f.current)
		copy(buf, u)
	}
	return nil
}

func TestSetsWindowsWallpaper(t *testing.T) {
	fake := &fakeSPI{}
	s := &Setter{Call: fake.call}
	path := `C:\Users\Ana Šarić\AppData\Local\Kisetsu\data\images\wallhaven\abc.jpg`

	method, err := s.Set(context.Background(), path)
	if err != nil || method != Windows {
		t.Fatalf("method %q, err %v", method, err)
	}
	if fake.current != path {
		t.Errorf("wallpaper = %q", fake.current)
	}
	if fake.winIni != spifUpdateIniFile|spifSendChange {
		t.Errorf("winIni = %#x, want saved and announced", fake.winIni)
	}
	if s.VarietyRunning() {
		t.Error("Variety reported on Windows")
	}
}

func TestWindowsWallpaperErrors(t *testing.T) {
	ctx := context.Background()
	path := `C:\pictures\a.jpg`

	if _, err := (&Setter{Call: (&fakeSPI{}).call}).Set(ctx, `relative\a.jpg`); err == nil {
		t.Error("a relative path was accepted")
	}
	if _, err := (&Setter{Call: (&fakeSPI{current: `C:\old.jpg`, ignore: true}).call}).Set(ctx, path); !errors.Is(err, ErrNotApplied) {
		t.Errorf("unchanged wallpaper: err = %v, want ErrNotApplied", err)
	}
	failed := errors.New("access denied")
	if _, err := (&Setter{Call: (&fakeSPI{fail: failed}).call}).Set(ctx, path); !errors.Is(err, failed) {
		t.Errorf("failing call: err = %v", err)
	}
	// Windows compares paths without regard to case.
	if _, err := (&Setter{Call: (&fakeSPI{current: `c:\PICTURES\A.JPG`, ignore: true}).call}).Set(ctx, path); err != nil {
		t.Errorf("same path in other case: %v", err)
	}
}
