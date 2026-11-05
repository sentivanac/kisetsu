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

// setTrayIconBeforeRun does nothing on Windows: the tray accepts an icon
// only once it runs.
func setTrayIconBeforeRun(log *slog.Logger, iconDir string) {}

// setTrayIconOnReady sets the icon as soon as the tray runs.
func setTrayIconOnReady() {
	systray.SetIcon(trayIconICO)
}

// attachParentProcess is ATTACH_PARENT_PROCESS for AttachConsole.
const attachParentProcess = ^uint32(0)

var attachConsoleProc = windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")

// attachConsole makes command output visible: kisetsu.exe is a Windows
// GUI program, which gets no console of its own, so `kisetsu.exe version`
// in a terminal would print nothing. It attaches to the terminal it was
// started from. Output already redirected (to a file or a pipe) is left
// as it is. The terminal shows its prompt before the output, because it
// does not wait for GUI programs.
func attachConsole() {
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && h != 0 && h != windows.InvalidHandle {
		return
	}
	if r, _, _ := attachConsoleProc.Call(uintptr(attachParentProcess)); r == 0 {
		return // started without a terminal (for example at sign-in)
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = f, f
	}
}
