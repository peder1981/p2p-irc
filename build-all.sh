#!/usr/bin/env bash
set -euo pipefail

# Script to cross-compile p2p-irc binaries for multiple platforms
BIN_DIR=./bin
rm -rf "$BIN_DIR"
mkdir -p "$BIN_DIR"

# Define target platforms (GOOS GOARCH)
platforms=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
  "windows arm64"
)

for plat in "${platforms[@]}"; do
  read -r GOOS GOARCH <<< "$plat"
  ext=""
  if [ "$GOOS" = "windows" ]; then
    ext=".exe"
  fi
  out="$BIN_DIR/p2p-irc-${GOOS}-${GOARCH}${ext}"
  echo "Building $out"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -mod=mod -o "$out" ./cmd/p2p-irc
  out2="$BIN_DIR/p2p-irc-tui-${GOOS}-${GOARCH}${ext}"
  echo "Building $out2"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -mod=mod -o "$out2" ./cmd/p2p-irc-tui
done
