//go:build windows

package main

import (
	"context"
	_ "embed"
	"log/slog"
	"os"
	"os/exec"
	"syscall"

	"fyne.io/systray"
	"golang.org/x/sys/windows"
)

// trayIconICO is the tray icon on Windows, which needs the .ico format.
//
//go:embed build/windows/icon.ico
var trayIconICO []byte

// detach starts cmd without a console window and outside the window's
// process group, so it keeps running when the window closes.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

// terminate ends the process. Windows has no SIGTERM for programs without
// a console; the agent keeps no state that needs a clean exit, and the
// system releases its lock.
func terminate(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

// watchSession does nothing: Windows ends the agent at sign-out.
func watchSession(ctx context.Context, cancel context.CancelFunc, log *slog.Logger) (stop func()) {
	return func() {}
}

// trayHostPresent is always true: the Windows taskbar shows tray icons.
func trayHostPresent() bool { return true }

// setTrayIcon sets the icon before the tray starts.
func setTrayIcon(log *slog.Logger, iconDir string) {
	systray.SetIcon(trayIconICO)
}
