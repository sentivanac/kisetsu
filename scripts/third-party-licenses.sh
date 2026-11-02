#!/usr/bin/env bash
# Prints the licenses of all third-party code built into Kisetsu: the Go
# toolchain's standard library, every Go module recorded in the binary,
# and the npm packages bundled into the frontend. Their licenses (MIT,
# BSD, Apache 2.0) require shipping these notices with the program.
#
# Usage: scripts/third-party-licenses.sh BINARY > THIRD_PARTY_LICENSES
set -euo pipefail

binary=${1:?binary}
root=$(cd "$(dirname "$0")/.." && pwd)

# license_files DIR prints all license and notice files in DIR (some
# modules have several, e.g. modernc.org/sqlite also ships SQLite's).
license_files() {
	find "$1" -maxdepth 1 -type f -regextype posix-extended \
		-iregex '.*/(licen[cs]e|copying|notice)([.-][^/]*)?' | sort | grep .
}

section() { # NAME VERSION FILE...
	local name=$1 version=$2 f
	shift 2
	printf '%s\n%s %s\n' "================================================================" "$name" "$version"
	for f in "$@"; do
		printf '%s\n%s\n\n' "---------------------------------------------------------------- $(basename "$f")" ""
		cat "$f"
		printf '\n'
	done
}

cat <<HEADER
Kisetsu includes the following third-party software. Each is distributed
under its own license, reproduced below. Kisetsu itself is under the MIT
License (see copyright).

HEADER

go_version=$(go version -m "$binary" | awk 'NR == 1 { print $2 }')
section "Go standard library" "$go_version" "$(go env GOROOT)/LICENSE"

missing=0
while read -r path version; do
	dir=$(go mod download -json "$path@$version" | sed -n 's/^\t"Dir": "\(.*\)",$/\1/p')
	if files=$(license_files "$dir"); then
		mapfile -t list <<<"$files"
		section "$path" "$version" "${list[@]}"
	else
		echo "no license file for $path@$version in $dir" >&2
		missing=1
	fi
done < <(go version -m "$binary" | awk '$1 == "dep" { print $2, $3 }')

frontend=$root/frontend
while read -r dir; do
	[[ $dir == "$frontend" ]] && continue
	name=$(node -p "require('$dir/package.json').name")
	version=$(node -p "require('$dir/package.json').version")
	if files=$(license_files "$dir"); then
		mapfile -t list <<<"$files"
		section "$name (npm)" "$version" "${list[@]}"
	else
		echo "no license file for npm package $name" >&2
		missing=1
	fi
done < <(cd "$frontend" && npm ls --omit=dev --all --parseable)

exit $missing
