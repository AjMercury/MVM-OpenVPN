#!/usr/bin/env bash
set -Eeuo pipefail
version=${1:?Usage: package-linux.sh VERSION amd64|arm64}
arch=${2:?Architecture required}
[[ $version =~ ^[0-9][a-zA-Z0-9._-]{0,63}$ && $arch =~ ^(amd64|arm64)$ ]] || exit 1
root=$(cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
stage=$(mktemp -d)
trap 'rm -rf -- "$stage"' EXIT
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -tags nomsgpack -trimpath -ldflags="-s -w -X main.version=$version" -o "$stage/server" ./cmd/server
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -tags nomsgpack -trimpath -ldflags="-s -w -X main.version=$version" -o "$stage/mvmctl" ./cmd/mvmctl
mkdir -p "$stage/editions/linux" artifacts/editions
cp -R editions/linux/installer editions/linux/systemd "$stage/editions/linux/"
cp editions/linux/README.md "$stage/editions/linux/README.md"
cp install.sh README.md DELIVERY.md "$stage/"
cp -R docs "$stage/docs"
maturity=DevelopmentPreview
if [[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then maturity=Stable; fi
printf 'VERSION=%s\nARCH=%s\nSCHEMA=1\nMATURITY=%s\n' "$version" "$arch" "$maturity" > "$stage/metadata.env"
chmod 755 "$stage/server" "$stage/mvmctl"
# Normalize CRLF from Windows working copies.
find "$stage/editions" -type f -exec sed -i 's/\r$//' {} +
archive="artifacts/editions/mvm-linux-$arch-$version.tar.gz"
tar -czf "$archive" -C "$stage" .
sha256sum "$archive" > "$archive.sha256"
go version -m "$stage/server" > "$archive.buildinfo.txt"
go run scripts/sbom.go "$stage/server" "$version" "$archive.server.spdx.json"
go run scripts/sbom.go "$stage/mvmctl" "$version" "$archive.mvmctl.spdx.json"



