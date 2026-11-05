# Releasing Kisetsu

How a version goes from this repository to a downloadable `.deb` and
Windows installer. Both are built by GitHub from the tagged commit
([ADR 0036](adr/0036-ci-and-release-workflows.md)); publishing it is
always a manual decision.

The examples use version 0.1.0.

## 1. Prepare

1. `VERSION` in the `Makefile` is the version to release, and
   `productVersion` in `wails.json` is the same (`make check` fails
   otherwise):

   ```sh
   make version
   ```

2. In `CHANGELOG.md`, replace "Unreleased" in the version's heading with
   today's date, for example `## [0.1.0] - 2026-11-10`. Check what the
   release notes will say:

   ```sh
   scripts/release-notes.sh 0.1.0
   ```

3. Check and build locally once more:

   ```sh
   nvm use
   make check
   make deb
   lintian --fail-on warning build/deb/kisetsu_*_amd64.deb
   make windows
   ```

   This package is a development build (for example
   `kisetsu_0.1.0~dev.142.g4d105fd_amd64.deb`): only a build from the
   tag is named plain `0.1.0`, and that one is made by GitHub (step 4).

4. Commit:

   ```sh
   git add CHANGELOG.md
   git commit -m "Release 0.1.0"
   ```

## 2. Tag

An annotated tag marks the exact commit of the release:

```sh
git tag -a v0.1.0 -m "Kisetsu 0.1.0"
```

## 3. The GitHub repository (first release only)

1. On <https://github.com/new>, create a repository named `kisetsu` in the
   `sentivanac` account. Do **not** add a README, license or `.gitignore`
   there; the repository already has them.
   - **Private** first is fine: Actions run there too, and the draft
     release can be checked without anyone else seeing it. A private
     repository's releases cannot be downloaded by others.
2. Connect it (use the SSH or HTTPS address GitHub shows):

   ```sh
   git remote add origin git@github.com:sentivanac/kisetsu.git
   ```

The name must stay `sentivanac/kisetsu`: the address is in the program
(User-Agent sent to image providers), the package (`Homepage`), the
manual page and the documentation.

## 4. Push

```sh
git push -u origin main
git push origin v0.1.0
```

The first push starts **CI**; the tag starts **Release**. Both are under
the repository's **Actions** tab. Release takes a few minutes.

If Release fails, the reason is in its log. Common ones: the tag does
not match `VERSION`, or `CHANGELOG.md` still says "Unreleased". Fix it,
then move the tag (allowed only while nothing is published):

```sh
git tag -d v0.1.0
git push origin :refs/tags/v0.1.0
# fix, commit, then:
git tag -a v0.1.0 -m "Kisetsu 0.1.0"
git push origin main v0.1.0
```

## 5. Check and publish

1. Open **Releases**: there is a draft "Kisetsu 0.1.0" with the notes
   from `CHANGELOG.md`, `kisetsu_0.1.0_amd64.deb` and
   `kisetsu-0.1.0-windows-amd64-setup.exe`.
2. Download the `.deb` and install it on a test machine:

   ```sh
   sudo apt install ./kisetsu_0.1.0_amd64.deb
   kisetsu version
   ```

   Install the Windows installer in the Windows VM, from its clean
   snapshot, and check the version in Settings → Apps.

3. If all is well, edit the draft and **Publish release**.
4. When the repository should be public: **Settings → General → Danger
   Zone → Change visibility → Public**. From then on, the latest version
   is always at
   <https://github.com/sentivanac/kisetsu/releases/latest>.

A published tag is never moved or reused. A mistake found later is
fixed in a new version (0.1.1).

## 6. After the release

Start the next version:

1. `VERSION := 0.1.1` (or `0.2.0`) in the `Makefile`, and the same
   `productVersion` in `wails.json`.
2. A new section at the top of `CHANGELOG.md`:

   ```markdown
   ## [0.2.0] - Unreleased
   ```

3. Commit.

## Version numbers

`VERSION` in the Makefile is the version of the next release. A build
calls itself that only when made from its tag without changes; every
other build is a development build, named after the commit
(`scripts/build-version.sh`):

```sh
make build-version   # e.g. 0.1.0~dev.142.g4d105fd
kisetsu version      # what an installed build says
```

The `~` makes `apt` treat a development build as older than the release,
so installing the release replaces it.


[Semantic Versioning](https://semver.org/): `0.1.1` for fixes only,
`0.2.0` for new features. Before 1.0, a minor version may also change
behavior; the changelog says so.
