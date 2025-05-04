#!/usr/bin/env bash
set -euo pipefail

# Cross-compile p2p-irc binaries for multiple platforms
echo "Resolving Go module dependencies..."
go mod tidy
go mod download

# Gera um número de porta aleatório entre 8000 e 9000 para evitar conflitos
RANDOM_PORT=$((8000 + RANDOM % 1000))

# Cria ou atualiza o arquivo de configuração padrão
cat > configs/config.toml << EOF

# Configuração do P2P-IRC
[network]
# Porta para o serviço de descoberta (gerada aleatoriamente)
# Você pode alterar esta porta se necessário
port = $RANDOM_PORT

# Servidores STUN para NAT traversal
stunServers = ["stun:stun.l.google.com:19302", "stun:stun1.l.google.com:19302"]
EOF

BIN_DIR="./bin"

echo "Cleaning ${BIN_DIR}..."
rm -rf "${BIN_DIR}"
mkdir -p "${BIN_DIR}"

# Build GUI version for host OS
echo "Building host GUI version: p2p-irc"
go build -o "${BIN_DIR}/p2p-irc" ./cmd/p2p-irc

# Build TUI version for host OS
echo "Building host TUI version: p2p-irc-tui"
go build -o "${BIN_DIR}/p2p-irc-tui" ./cmd/p2p-irc-tui

## WebAssembly build currently not supported due to os/user package limitations
# To enable wasm builds, ensure code avoids os/user dependencies and imports the appropriate Fyne JS driver.
# echo "Building WebAssembly version: p2p-irc-js-wasm"
# GOOS=js GOARCH=wasm go build -o "${BIN_DIR}/p2p-irc-js-wasm" ./cmd/p2p-irc

# Cross-compile TUI for various platforms
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
  if [[ "${GOOS}" == "windows" ]]; then ext=".exe"; fi
  out="${BIN_DIR}/p2p-irc-tui-${GOOS}-${GOARCH}${ext}"
  echo "Building ${out}..."
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" go build -o "${out}" ./cmd/p2p-irc-tui
done

echo "All binaries are placed in ${BIN_DIR}"
