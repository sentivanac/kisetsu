# ADR 0057: An APT repository on GitHub Pages

- **Status:** accepted
- **Date:** 2026-11-26

## Context

A `.deb` installed from a file is never updated by `apt`. Since 0.9.0
Kisetsu tells about new versions (ADR 0056), but the user still downloads
and installs each one by hand. Programs such as Chrome, VS Code and Signal
have their own APT repository: added once, they are then updated with the
rest of the system.

## Decision

A signed APT repository at `https://sentivanac.github.io/kisetsu/apt`,
added by hand with three commands from the README (key, `.sources` file in
deb822 format with `Signed-By`, so the key is trusted for this repository
only).

- **GitHub Pages**, deployed by `.github/workflows/apt.yml` when a release
  is published (and by hand, to rebuild). Free, no server, no third party
  between the project and its users, and built in public like the release
  itself (ADR 0036). The draft step of releasing stays: nothing reaches
  the repository before **Publish release**.
- **Only the latest release** is in it. `apt` installs the newest version
  anyway, so the site is rebuilt from scratch each time and keeps no
  state. Going back means downloading an older `.deb` from Releases.
- **Layout:** `dists/stable/main/binary-amd64`, `pool/main/k/kisetsu`,
  made by `apt-ftparchive` in `scripts/apt-repo.sh`, signed into
  `InRelease` and `Release.gpg` (SHA-512). The script then checks both
  signatures with `gpgv` against the public key alone, as `apt` will.
- **The key.** Ed25519, used only for this repository, without an expiry
  date (a date would make every user download the key again). The private
  key is the secret `APT_SIGNING_KEY` (with `APT_SIGNING_PASSPHRASE` if it
  has one); the maintainer keeps a backup and the revocation certificate
  offline. The public key is in the repository as
  `packaging/apt/kisetsu.asc`, and the script signs only with the key of
  that fingerprint: a wrong secret fails the workflow instead of signing.

## Consequences

- A broken or unreachable repository shows as an error in every user's
  `apt update`. The workflow therefore only replaces the site after the
  repository was built and its signatures checked.
- A lost private key means a new key that every user must add by hand.
  A leaked one would let someone sign a false package: it must be revoked
  and replaced at once.
- Windows is not affected.
- Kisetsu still tells about new versions in an installation from the
  repository, though the update arrives through the system anyway
  (left for later; see ADR 0056).

## Alternatives considered

- **A Launchpad PPA.** Ubuntu and Mint only, not Debian; packages are
  built there from source without network access, but the build needs Go
  modules and npm packages.
- **Cloudsmith, packagecloud and similar services.** Free for open
  source, but a company between the project and its users.
- **Adding the repository from the `.deb` itself**, as Chrome does. More
  convenient, but the package would change `/etc/apt` on its own.
  Possible later, once the repository has worked for a while.

## Verification

Locally, with a throwaway key: the script built a repository from two
packages; `apt-get update` (run without root, with its own directories)
accepted it, listed both versions and downloaded a package from the pool;
a changed `InRelease` was refused with BADSIG. The script refuses a wrong
passphrase and a secret key that does not match the public one.
