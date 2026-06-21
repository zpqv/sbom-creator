#!/usr/bin/env bash
# Cross-compile SBOM Creator for every target. Pure Go + stdlib only, so
# CGO_ENABLED=0 lets us build all platforms from one machine with no toolchain.
set -euo pipefail
cd "$(dirname "$0")"

VERSION="${1:-dev}"
LDFLAGS="-s -w"   # strip symbols/debug for smaller binaries
export CGO_ENABLED=0

mkdir -p dist
build() { # os arch ext
  local os="$1" arch="$2" ext="${3:-}"
  local out="dist/sbom-creator-${os}-${arch}${ext}"
  echo "  building ${out}"
  GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "$LDFLAGS" -o "$out" .
}

echo "SBOM Creator ${VERSION} — cross-compiling…"
build darwin  arm64
build darwin  amd64
build linux   amd64
build linux   arm64
build windows amd64 .exe

echo "done:"
ls -lh dist/sbom-creator-* | awk '{print "  "$5"\t"$9}'
