#!/usr/bin/env bash
# Prints the version a build calls itself, given the release version from
# the Makefile. Run it from the repository.
#
#   0.1.0                                   HEAD is tagged v0.1.0, no changes
#   0.1.0~dev.142.g4d105fd                  any other commit
#   0.1.0~dev.142.g4d105fd.dirty.202611031012
#                                           uncommitted changes (UTC time)
#   0.1.0~dev                               no Git history
#
# "~" makes Debian sort a development build before its release, so the
# release package replaces it. Untracked files do not count as changes.
#
# Usage: scripts/build-version.sh VERSION
set -euo pipefail

version=${1:?version}

if ! git rev-parse --verify --quiet HEAD >/dev/null 2>&1; then
	echo "$version~dev"
	exit
fi

dirty=
if [[ -n $(git status --porcelain --untracked-files=no) ]]; then
	dirty=.dirty.$(date -u +%Y%m%d%H%M)
fi

if [[ -z $dirty ]] && git tag --points-at HEAD | grep -qx "v$version"; then
	echo "$version"
	exit
fi

echo "$version~dev.$(git rev-list --count HEAD).g$(git rev-parse --short=7 HEAD)$dirty"
