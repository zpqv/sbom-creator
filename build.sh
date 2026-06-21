#!/usr/bin/env bash
# Cross-compile SBOM Creator for every target. Pure Go + stdlib only, so
# CGO_ENABLED=0 lets us build all platforms from one machine with no toolchain.
set -euo pipefail
cd "$(dirname "$0")"

VERSION="${1:-dev}"
LDFLAGS="-s -w -X main.version=${VERSION}" # strip symbols/debug; stamp version
export CGO_ENABLED=0

mkdir -p dist
build() { # os arch ext
  local os="$1" arch="$2" ext="${3:-}"
  local bin="sbom-creator-${os}-${arch}${ext}"
  echo "  building ${bin}"
  GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "$LDFLAGS" -o "dist/${bin}" .

  # Package an archive containing the binary under the uniform name
  # "sbom-creator". Extracting a .tar.gz with the `tar` command does NOT attach
  # macOS's com.apple.quarantine flag, so the extracted binary runs without a
  # Gatekeeper prompt (unlike a bare downloaded binary).
  local staged="sbom-creator${ext}"
  cp "dist/${bin}" "dist/${staged}"
  if [ "$os" = "windows" ]; then
    ( cd dist && zip -q "sbom-creator-${os}-${arch}.zip" "${staged}" )
  else
    ( cd dist && tar -czf "sbom-creator-${os}-${arch}.tar.gz" "${staged}" )
  fi
  rm -f "dist/${staged}"
}

echo "SBOM Creator ${VERSION} — cross-compiling…"
build darwin  arm64
build darwin  amd64
build linux   amd64
build linux   arm64
build windows amd64 .exe

echo "done:"
ls -lh dist/sbom-creator-*.tar.gz dist/sbom-creator-*.zip 2>/dev/null | awk '{print "  "$5"\t"$9}'
