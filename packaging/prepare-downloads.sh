#!/bin/sh
set -eu

VERSION="${1:?usage: prepare-downloads.sh VERSION [DIST] [OUT]}"
DIST="${2:-dist}"
OUT="${3:-downloads-site}"

rm -rf "$OUT"
mkdir -p "$OUT/latest" "$OUT/v$VERSION"

for f in "$DIST"/*.tar.gz "$DIST"/*.zip "$DIST"/*.deb "$DIST"/*.rpm "$DIST"/checksums.txt; do
  cp "$f" "$OUT/v$VERSION/"
done

stable() {
  [ -f "$DIST/$1" ] || { echo "missing artifact: $1" >&2; exit 1; }
  cp "$DIST/$1" "$OUT/latest/$2"
}

for os in linux darwin; do
  for arch in amd64 arm64; do
    stable "peer-agent_${VERSION}_${os}_${arch}.tar.gz" "peer-agent_${os}_${arch}.tar.gz"
  done
done
for arch in amd64 arm64; do
  stable "peer-agent_${VERSION}_windows_${arch}.zip" "peer-agent_windows_${arch}.zip"
  stable "luminaproxy-peer-agent_${VERSION}_linux_${arch}.deb" "luminaproxy-peer-agent_${arch}.deb"
  stable "luminaproxy-peer-agent_${VERSION}_linux_${arch}.rpm" "luminaproxy-peer-agent_${arch}.rpm"
done

(cd "$OUT/latest" && sha256sum -- * > ../checksums.tmp && mv ../checksums.tmp checksums.txt)

cp packaging/install.sh packaging/install.ps1 "$OUT/"
printf '%s\n' "$VERSION" > "$OUT/VERSION"
printf '%s\n' "$VERSION" > "$OUT/latest/VERSION"
