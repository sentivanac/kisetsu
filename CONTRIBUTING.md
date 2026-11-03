# Contributing to Kisetsu

Thank you for helping. Bug reports, ideas and code are all welcome.

## Reporting a bug

Open an issue and include:

- the output of `kisetsu version`;
- your distribution and desktop (for example Ubuntu 24.04, GNOME on
  Wayland);
- what you did, what you expected and what happened;
- the end of the log, `~/.local/state/kisetsu/kisetsu.log`. For more
  detail, start Kisetsu with `KISETSU_LOG_LEVEL=debug kisetsu` and
  repeat the problem. A debug log contains your search text; remove
  anything you do not want to share.

## Changing the code

[docs/development.md](docs/development.md) explains how to build and run
Kisetsu.

- Keep changes small and focused; one pull request, one purpose.
- New behavior comes with tests. `make check` must pass.
- Important design decisions are recorded in [docs/adr/](docs/adr/).
  If your change makes one, add a short ADR, or open an issue first to
  discuss it.
- User-visible changes get a line in [CHANGELOG.md](CHANGELOG.md).

## Image sources

Kisetsu exists to find images while respecting the people who made them.
A new image provider must:

- use a documented, official API (no scraping);
- respect its rate limits and terms of use, and identify Kisetsu in the
  User-Agent;
- keep the author, source page and license of every image, and say
  "license unknown" rather than guess.

## License

Kisetsu is under the [MIT License](LICENSE). By contributing, you agree
that your contribution is released under the same license.
