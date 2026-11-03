#!/usr/bin/env bash
# Prints the CHANGELOG.md section of a version, without its heading, for
# the GitHub release notes. Fails if the version has no section, or if
# its date is still "Unreleased" (unless --draft is given).
#
# Usage: scripts/release-notes.sh [--draft] VERSION
set -euo pipefail

draft=false
if [[ ${1:-} == --draft ]]; then
	draft=true
	shift
fi
version=${1:?version}
changelog=$(dirname "$0")/../CHANGELOG.md

heading=$(grep -m1 -E "^## \[${version//./\\.}\]" "$changelog") || {
	echo "CHANGELOG.md has no section for $version" >&2
	exit 1
}
if [[ $heading == *Unreleased* ]] && ! $draft; then
	echo "CHANGELOG.md still says '$heading'; set the release date first" >&2
	exit 1
fi
awk -v h="$heading" '
	$0 == h { found = 1; next }
	found && /^## \[/ { exit }
	found { print }
' "$changelog" | sed -e '/./,$!d'
