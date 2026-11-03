# Kisetsu 季節

Find beautiful wallpapers, keep the ones you like, and let your desktop
change with them.

Kisetsu (Japanese for "season") searches several image sources at once,
keeps a local library with the author and license of every image, and
sets or rotates your desktop wallpaper. It is free and open source (MIT).

> **Status:** version 0.1.0, the first release. Linux only; Windows
> support is planned after the Linux release.

## Features

- **Search** Wallhaven and, if you turn it on, Wikimedia Commons at the
  same time. Filter by minimum size (or "my screen size") and
  orientation; exclude words with `-word`, e.g. `mountains -snow`.
- **Save searches** and run them again with one click.
- **Library:** download images to keep them. They work offline, can be
  sorted into collections, and duplicates are detected — also re-encoded
  copies of the same picture.
- **Credit where it is due:** author, source page and license are kept
  for every image and shown with it.
- **Set as wallpaper**, or **rotate** automatically — every 15 minutes up
  to once a day, in random or added order, from the whole library, a
  collection, or a saved search.
- **Fresh images from a saved search:** when rotating from a saved
  search, Kisetsu downloads a few new images when it runs low. They live
  in a size-limited cache (200 MB, reduced to your screen size); press
  **Keep** to move one into your library.

## Install

Kisetsu is packaged for **Ubuntu 22.04 or newer, Linux Mint 21 or newer
and Debian 12 or newer** (64-bit).

Download `kisetsu_0.1.0_amd64.deb` and install it with:

```sh
sudo apt install ./kisetsu_0.1.0_amd64.deb
```

`apt` also installs what Kisetsu needs (GTK 3 and WebKitGTK 4.1). Then
start **Kisetsu** from the applications menu, or run `kisetsu`.

To build it yourself, see [docs/development.md](docs/development.md).

## Supported desktops

| Desktop | Set wallpaper | Rotation |
|---|---|---|
| GNOME (X11 and Wayland) | ✓ | ✓ |
| MATE | ✓ | ✓ |
| Variety running (any desktop) | ✓ through Variety | — see below |
| KDE Plasma, XFCE, Cinnamon, others | not yet | not yet |

On other desktops, search and the library work fully; only setting the
wallpaper says it is not supported yet.

**Variety:** if [Variety](https://peterlevi.com/variety/) is running,
"Set as wallpaper" hands the image to Variety, so Variety keeps working
as before. Kisetsu's own rotation sets the wallpaper directly, and the
Wallpaper tab warns that Variety would change it too: turn off Variety's
automatic change, or quit Variety, if Kisetsu should rotate.

**Rotation** runs as a systemd user timer, so it continues while the
window is closed and stops when you log out. The timer is created only
when you turn rotation on, and removed when you turn it off.

## Images and licenses

Kisetsu shows where every image comes from and under which license, but
**it does not make images free to use**. Check the license on the source
page before publishing or sharing an image.

- **Wallhaven:** only images rated safe for work are requested. Licenses
  are often not stated; Kisetsu then says "License unknown".
- **Wikimedia Commons** (off by default): limited to its curated quality
  collections, with unsuitable results filtered out. Commons has no
  safe-for-work rating, so the filter cannot be perfect. Most images are
  under Creative Commons licenses that require crediting the author.

Kisetsu uses the official APIs of both sources, respects their rate
limits, and identifies itself to them as Kisetsu with its version.

## Privacy

Kisetsu has no account, no telemetry and no server of its own.

- Your searches go to the image sources you selected (Wallhaven,
  Wikimedia Commons) — that is how they are searched.
- Thumbnails, previews and downloads are loaded from those sources'
  servers.
- Everything else stays on your computer: library, collections, saved
  searches, settings and logs. Search text is not stored unless you save
  the search (or turn on debug logging, which writes it to the log).

## Where Kisetsu keeps its files

| What | Where |
|---|---|
| Library images, thumbnails, database | `~/.local/share/kisetsu` |
| Fresh images from saved searches (max 200 MB) | `~/.cache/kisetsu` |
| Log file (a new one is started above 5 MB; the previous is kept as `.1`) | `~/.local/state/kisetsu/kisetsu.log` |
| Rotation timer (only while rotation is on) | `~/.config/systemd/user/kisetsu-rotate.{service,timer}` |

Kisetsu never deletes images from your library on its own; only the
cache of fresh images is cleaned up automatically.

## Uninstall

1. In Kisetsu, open **Wallpaper** and turn rotation off. This removes
   the timer.
2. Remove the program:

   ```sh
   sudo apt remove kisetsu
   ```

3. Your library stays. To delete it as well:

   ```sh
   rm -r ~/.local/share/kisetsu ~/.cache/kisetsu ~/.local/state/kisetsu ~/.config/kisetsu
   ```

If you removed the program while rotation was on, the timer stays but
does nothing. To remove it as well:

```sh
systemctl --user disable --now kisetsu-rotate.timer
rm ~/.config/systemd/user/kisetsu-rotate.service ~/.config/systemd/user/kisetsu-rotate.timer
systemctl --user daemon-reload
```

## Command line

```sh
kisetsu                              # open the window
kisetsu rotate                       # change to the next wallpaper now
kisetsu fetch "Mountains" --count 5  # new images for the saved search "Mountains"
kisetsu version
kisetsu help
```

## Troubleshooting

- **The window stays blank or flickers:** start it without WebView
  hardware acceleration: `KISETSU_WEBVIEW_GPU=never kisetsu`
  (`ondemand` is the default, `always` is also accepted).
- **More detail in the log:** `KISETSU_LOG_LEVEL=debug kisetsu`, then
  look at `~/.local/state/kisetsu/kisetsu.log`.
- **The wallpaper does not change:** the Wallpaper tab shows whether the
  timer runs and when the next change is due. Check whether Variety is
  running, and whether your desktop is in the table above.

## Contributing

Bug reports and contributions are welcome; see
[CONTRIBUTING.md](CONTRIBUTING.md). Changes between versions are listed
in [CHANGELOG.md](CHANGELOG.md).

## License

Kisetsu is released under the [MIT License](LICENSE). The licenses of
the libraries it is built with are installed with it, in
`/usr/share/doc/kisetsu/THIRD_PARTY_LICENSES.gz`. Images found with
Kisetsu belong to their authors and keep their own licenses.
