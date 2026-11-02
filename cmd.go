package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sentivanac/kisetsu/internal/rotation"
)

const usage = `Usage: kisetsu [command]

Without a command, the Kisetsu window opens.

Commands:
  rotate               change the wallpaper to the next library image
  rotate --scheduled   the same, but only if rotation is enabled
                       (used by the systemd timer)
  help                 show this help

Exit status: 0 when the wallpaper changed or there was nothing to do,
1 on failure, 2 for invalid usage.`

// rotateTimeout bounds one rotation run from the command line.
const rotateTimeout = time.Minute

// runCommand runs a command-line command and returns the exit status.
func runCommand(name string, args []string) int {
	switch name {
	case "rotate":
		return runRotate(args)
	case "help", "-h", "--help":
		fmt.Println(usage)
		return 0
	}
	fmt.Fprintf(os.Stderr, "kisetsu: unknown command %q\n\n%s\n", name, usage)
	return 2
}

func runRotate(args []string) int {
	flags := flag.NewFlagSet("rotate", flag.ContinueOnError)
	scheduled := flags.Bool("scheduled", false, "rotate only if rotation is enabled")
	flags.SetOutput(os.Stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "kisetsu rotate: unexpected argument %q\n", flags.Arg(0))
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), rotateTimeout)
	defer cancel()
	c, err := openCore(ctx, "rotate", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu rotate:", err)
		return 1
	}
	defer c.Close()

	return rotate(ctx, c, c.rotator(), *scheduled, os.Stdout)
}

// rotate changes the wallpaper once and returns the exit status. It is
// separate from runRotate so tests can pass a fake wallpaper setter.
func rotate(ctx context.Context, c *core, r *rotation.Rotator, scheduled bool, out io.Writer) int {
	unlock, err := rotation.Lock(c.rotationLockPath())
	if errors.Is(err, rotation.ErrLocked) {
		c.log.Info("another rotation is running; skipping")
		fmt.Fprintln(out, "Another wallpaper change is in progress; nothing done.")
		return 0
	}
	if err != nil {
		c.log.Error("rotation lock", "err", err)
		fmt.Fprintln(os.Stderr, "kisetsu rotate:", err)
		return 1
	}
	defer unlock()

	if scheduled {
		cfg, err := rotation.LoadSettings(ctx, r.Settings)
		if err != nil {
			c.log.Error("cannot read rotation settings", "err", err)
			return 1
		}
		if !cfg.Enabled {
			// A timer left behind after rotation was turned off does nothing.
			c.log.Info("rotation is turned off; skipping")
			fmt.Fprintln(out, "Rotation is turned off; nothing done.")
			return 0
		}
	}

	res, err := r.Next(ctx)
	switch {
	case errors.Is(err, rotation.ErrNoImages):
		c.log.Warn("no images to rotate", "err", err)
		fmt.Fprintln(out, "There are no images to rotate. Download images into your library first.")
		return 0
	case err != nil:
		c.log.Error("rotation failed", "err", err)
		fmt.Fprintln(os.Stderr, "kisetsu rotate:", err)
		return 1
	}
	fmt.Fprintf(out, "Wallpaper changed to library image %d (%s).\n", res.ImageID, res.Method)
	return 0
}
