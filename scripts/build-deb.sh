#!/usr/bin/env bash
# Builds a .deb package from build/bin/kisetsu (run `make build` first;
# `make deb` does both). Nothing is installed: the package is only written
# to build/deb/.
#
# Usage: scripts/build-deb.sh VERSION APP_ID
set -euo pipefail

version=${1:?version}
app_id=${2:?application ID}
arch=$(dpkg --print-architecture)
root=$(cd "$(dirname "$0")/.." && pwd)
binary=$root/build/bin/kisetsu
out=$root/build/deb
pkg=$out/kisetsu_${version}_${arch}

[[ -x $binary ]] || { echo "missing $binary; run make build" >&2; exit 1; }
# A binary built by `wails dev` or without the Makefile says "dev".
built=$("$binary" version)
[[ $built == "Kisetsu $version" ]] || { echo "binary reports '$built', expected 'Kisetsu $version'; run make build" >&2; exit 1; }

rm -rf "$pkg"
umask 022
install -Dm755 "$binary" "$pkg/usr/bin/kisetsu"
install -Dm644 "$root/build/linux/$app_id.desktop" "$pkg/usr/share/applications/$app_id.desktop"
install -Dm644 "$root/build/linux/$app_id.svg" "$pkg/usr/share/icons/hicolor/scalable/apps/$app_id.svg"
for png in "$root"/build/linux/icons/*.png; do
	size=$(basename "$png" .png)
	install -Dm644 "$png" "$pkg/usr/share/icons/hicolor/${size}x${size}/apps/$app_id.png"
done

# Debian's copyright file: the project's MIT license.
doc=$pkg/usr/share/doc/kisetsu
install -d "$doc"
{
	echo "Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/"
	echo "Upstream-Name: Kisetsu"
	echo "Source: https://github.com/sentivanac/kisetsu"
	echo
	echo "Files: *"
	echo "Copyright: 2026 Miroslav Sentivanac"
	echo "License: Expat"
	sed -e 's/^$/./' -e 's/^/ /' "$root/LICENSE"
} > "$doc/copyright"
chmod 644 "$doc/copyright"

# Debian changelog (required in every package), one entry per release.
{
	echo "kisetsu ($version) unstable; urgency=medium"
	echo
	echo "  * Release $version. See https://github.com/sentivanac/kisetsu for changes."
	echo
	echo " -- Miroslav Sentivanac <miroslav.sentivanac@gmail.com>  $(date -R)"
} | gzip -9n > "$doc/changelog.gz"
chmod 644 "$doc/changelog.gz"

# Library dependencies with the versions the binary was linked against,
# worked out by dpkg-shlibdeps from the installed library packages.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/debian"
printf 'Source: kisetsu\n\nPackage: kisetsu\nArchitecture: any\n' > "$tmp/debian/control"
depends=$(cd "$tmp" && dpkg-shlibdeps -O "$pkg/usr/bin/kisetsu" 2>/dev/null | sed -n 's/^shlibs:Depends=//p')
[[ -n $depends ]] || { echo "dpkg-shlibdeps found no dependencies" >&2; exit 1; }

install -d "$pkg/DEBIAN"
cat > "$pkg/DEBIAN/control" <<CONTROL
Package: kisetsu
Version: $version
Architecture: $arch
Maintainer: Miroslav Sentivanac <miroslav.sentivanac@gmail.com>
Installed-Size: $(du -sk --exclude=DEBIAN "$pkg" | cut -f1)
Depends: $depends
Recommends: libglib2.0-bin, systemd
Section: graphics
Priority: optional
Homepage: https://github.com/sentivanac/kisetsu
Description: wallpaper discovery and management
 Kisetsu searches several image sources at once (Wallhaven, and
 optionally Wikimedia Commons), keeps a local library with collections
 and duplicate detection, and sets or rotates the desktop wallpaper.
 Author, source and license of every image are kept.
 .
 Setting the wallpaper is supported on GNOME and MATE, directly or
 through Variety. Rotation uses a systemd user timer.
CONTROL

dpkg-deb --build --root-owner-group -Zxz "$pkg" "$pkg.deb" >/dev/null
rm -rf "$pkg"
echo "$pkg.deb"
