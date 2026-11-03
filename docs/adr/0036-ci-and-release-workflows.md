# ADR 0036: CI and release workflows

- **Status:** accepted
- **Date:** 2026-11-03
- Phase 7. Prepared before the repository is public; nothing runs until
  it is pushed to GitHub.

## Decision

### `.github/workflows/ci.yml`

On every push to `main` and every pull request: install GTK 3 and
WebKitGTK 4.1, Go from `go.mod`, Node from `.nvmrc`, `npm ci`, then
`make check` — the same command developers run.

### `.github/workflows/release.yml`

On a tag `v*`:

1. the tag must equal `v` + `VERSION` from the Makefile (`make version`);
2. the release notes are the version's section of `CHANGELOG.md`
   (`scripts/release-notes.sh`), which must have a date, not
   "Unreleased";
3. `make check`, `make deb` (Wails CLI v2.17.0), `lintian --fail-on
   warning`;
4. a **draft** GitHub release with the `.deb` attached. Publishing it is
   a manual decision.

The package is built from exactly the tagged commit, by GitHub, not on a
developer's machine — anyone can see how it was made.

Both run on **Ubuntu 22.04**, the oldest supported system: a package
built there declares library versions that 22.04 and everything newer
satisfy.

Actions are pinned to major versions (`actions/checkout@v4`,
`actions/setup-go@v5`, `actions/setup-node@v4`); the release job alone
may write to the repository (`contents: write`).

### Fixes found while preparing this

- **A fresh clone did not compile.** `//go:embed all:frontend/dist`
  needs at least one file, but `frontend/dist` is not in Git and Vite
  empties it on every build (so the old `gitkeep` was gone). `make test`,
  `make vet` and `make test-live` now create an empty placeholder when the
  directory is empty; Go tests do not need the real frontend.
- **`npm ls` masked paths.** See ADR 0033: the bundled npm packages are
  now read from `package-lock.json`.

## Verification

Not run on GitHub yet. Simulated locally in a fresh clone (with the
uncommitted changes copied in): `npm ci`, `make check` (all tests pass,
placeholder created), `make deb`, `lintian --fail-on warning` (no
findings), `scripts/release-notes.sh` (refuses "Unreleased", prints the
section with `--draft`). Both workflow files parse as YAML; `actionlint`
was not available.
