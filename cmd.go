package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/fetched"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/searches"
)

const usage = `Usage: kisetsu [command]

Without a command, the Kisetsu window opens.

Commands:
  agent                run in the background: change the wallpaper when
                       rotation is due (started at login)
  rotate               change the wallpaper to the next library image
  fetch NAME [--count N]
                       download up to N (default 5) new images for the
                       saved search NAME into Fetched
  version              show the version
  help                 show this help

Exit status: 0 when the wallpaper changed or there was nothing to do,
1 on failure, 2 for invalid usage.`

// rotateTimeout bounds one rotation run from the command line, including
// fetching new images for a saved search.
const rotateTimeout = 5 * time.Minute

// runCommand runs a command-line command and returns the exit status.
func runCommand(name string, args []string) int {
	switch name {
	case "agent":
		return runAgent(args)
	case "rotate":
		return runRotate(args)
	case "fetch":
		return runFetch(args)
	case "version", "--version":
		fmt.Println(appinfo.Name, appinfo.Version)
		return 0
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

	// A saved search as source may need new images first.
	client := httpClient()
	searcher, err := c.searchService(client)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu rotate:", err)
		return 1
	}
	dl, err := c.downloader(client, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu rotate:", err)
		return 1
	}
	defer dl.Close()

	return rotate(ctx, c, c.rotator(c.fetcher(searcher, dl)), *scheduled, os.Stdout)
}

// rotate changes the wallpaper once and returns the exit status. It is
// separate from runRotate so tests can pass fakes. The agent counts its
// next change from this one.
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

// fetchTimeout bounds one fetch from the command line: a few searches and
// downloads.
const fetchTimeout = 5 * time.Minute

func runFetch(args []string) int {
	flags := flag.NewFlagSet("fetch", flag.ContinueOnError)
	count := flags.Int("count", 5, "how many new images to download (1-24)")
	flags.SetOutput(os.Stderr)
	// Allow the name before or after the flags.
	var name string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if name == "" && flags.NArg() == 1 {
		name = flags.Arg(0)
	} else if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "kisetsu fetch: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if name == "" || *count < 1 || *count > 24 {
		fmt.Fprintln(os.Stderr, "kisetsu fetch: give a saved search name and a count from 1 to 24")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	c, err := openCore(ctx, "fetch", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu fetch:", err)
		return 1
	}
	defer c.Close()

	client := httpClient()
	searcher, err := c.searchService(client)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu fetch:", err)
		return 1
	}
	dl, err := c.downloader(client, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu fetch:", err)
		return 1
	}
	defer dl.Close()
	return fetch(ctx, c, c.fetcher(searcher, dl), name, *count, os.Stdout)
}

// fetch runs one fetch and returns the exit status; tests pass fakes.
func fetch(ctx context.Context, c *core, f *fetched.Fetcher, name string, count int, out io.Writer) int {
	saved, err := f.Searches.FindByName(ctx, name)
	if errors.Is(err, searches.ErrNotFound) {
		fmt.Fprintf(os.Stderr, "kisetsu fetch: no saved search named %q\n", name)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu fetch:", err)
		return 1
	}

	// Pruning must not race with a rotation, and never removes the
	// wallpaper on screen.
	unlock, err := rotation.Lock(c.rotationLockPath())
	if errors.Is(err, rotation.ErrLocked) {
		fmt.Fprintln(out, "A wallpaper change is in progress; try again in a moment.")
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu fetch:", err)
		return 1
	}
	defer unlock()
	var protect []int64
	if st, err := rotation.NewHistory(c.db).State(ctx); err == nil && st.Current != 0 {
		protect = append(protect, st.Current)
	}

	res, err := f.Fetch(ctx, saved.ID, count, protect...)
	if err != nil {
		c.log.Error("fetch failed", "saved_search", saved.ID, "err", err)
		fmt.Fprintln(os.Stderr, "kisetsu fetch:", err)
		return 1
	}
	fmt.Fprintf(out, "Fetched %d new image(s) for %q", res.Fetched, saved.Name)
	if res.Pruned > 0 {
		fmt.Fprintf(out, "; removed %d old fetched image(s) to stay within the limit", res.Pruned)
	}
	fmt.Fprintln(out, ".")
	return 0
}
