#!/usr/bin/env bash
# Builds the signed APT repository from .deb packages (ADR 0057):
#
#   OUT/kisetsu.asc                            public key, for users
#   OUT/pool/main/k/kisetsu/*.deb
#   OUT/dists/stable/main/binary-amd64/Packages(.gz)
#   OUT/dists/stable/Release, InRelease, Release.gpg
#
# The signing key must be in the GnuPG keyring and match PUBLIC_KEY (the
# public key kept in the repository), so a wrong secret cannot sign. If
# the key has a passphrase, it is read from APT_SIGNING_PASSPHRASE.
#
# Usage: scripts/apt-repo.sh OUT PUBLIC_KEY DEB...
set -euo pipefail

out=${1:?output directory}
public_key=${2:?public key file}
shift 2
(($# > 0)) || {
	echo "no .deb given" >&2
	exit 1
}

fingerprint=$(gpg --show-keys --with-colons "$public_key" | awk -F: '$1 == "fpr" { print $10; exit }')
[[ -n $fingerprint ]] || {
	echo "$public_key holds no key" >&2
	exit 1
}

if [[ -e $out ]]; then
	echo "$out exists; give a new directory" >&2
	exit 1
fi
pool=$out/pool/main/k/kisetsu
dist=$out/dists/stable
mkdir -p "$pool" "$dist/main/binary-amd64"

for deb in "$@"; do
	name=$(dpkg-deb --field "$deb" Package)
	arch=$(dpkg-deb --field "$deb" Architecture)
	if [[ $name != kisetsu || $arch != amd64 ]]; then
		echo "$deb is $name for $arch, not kisetsu for amd64" >&2
		exit 1
	fi
	cp "$deb" "$pool/"
done
cp "$public_key" "$out/kisetsu.asc"

# Paths in Packages are relative to the repository's root.
(cd "$out" && apt-ftparchive packages pool) >"$dist/main/binary-amd64/Packages"
gzip -9 --keep --no-name "$dist/main/binary-amd64/Packages"

release=$(mktemp)
trap 'rm -f "$release"' EXIT
apt-ftparchive \
	-o APT::FTPArchive::Release::Origin=Kisetsu \
	-o APT::FTPArchive::Release::Label=Kisetsu \
	-o APT::FTPArchive::Release::Suite=stable \
	-o APT::FTPArchive::Release::Codename=stable \
	-o APT::FTPArchive::Release::Architectures=amd64 \
	-o APT::FTPArchive::Release::Components=main \
	-o "APT::FTPArchive::Release::Description=Kisetsu, a wallpaper finder and manager" \
	release "$dist" >"$release"
mv "$release" "$dist/Release"

sign=(gpg --batch --yes --local-user "$fingerprint!" --digest-algo SHA512)
if [[ -n ${APT_SIGNING_PASSPHRASE:-} ]]; then
	sign+=(--pinentry-mode loopback --passphrase-fd 0)
fi
"${sign[@]}" --clearsign --output "$dist/InRelease" "$dist/Release" <<<"${APT_SIGNING_PASSPHRASE:-}"
"${sign[@]}" --armor --detach-sign --output "$dist/Release.gpg" "$dist/Release" <<<"${APT_SIGNING_PASSPHRASE:-}"

# What apt will do: check both signatures against the public key alone.
keyring=$(mktemp)
trap 'rm -f "$release" "$keyring"' EXIT
gpg --dearmor <"$public_key" >"$keyring"
gpgv --quiet --keyring "$keyring" "$dist/InRelease" 2>/dev/null
gpgv --quiet --keyring "$keyring" "$dist/Release.gpg" "$dist/Release" 2>/dev/null
echo "APT repository in $out, signed by $fingerprint"
