#!/usr/bin/env bash
set -euo pipefail

# Cores para saída
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Script para compilar as versões GUI e TUI do p2p-irc

echo -e "${YELLOW}Resolvendo dependências dos módulos Go...${NC}"
go mod tidy
go mod download

BIN_DIR="./bin"

echo -e "${YELLOW}Limpando o diretório de compilação (${BIN_DIR})...${NC}"
rm -rf "${BIN_DIR}"
mkdir -p "${BIN_DIR}"

# Compila a versão GUI para o sistema operacional host
echo -e "${YELLOW}Compilando a versão GUI para o host: p2p-irc...${NC}"
if go build -o "${BIN_DIR}/p2p-irc" ./cmd/p2p-irc; then
    echo -e "${GREEN}Versão GUI compilada com sucesso!${NC}"
else
    echo -e "${RED}Falha ao compilar a versão GUI.${NC}"
    exit 1
fi

# Compila a versão TUI para o sistema operacional host
echo -e "${YELLOW}Compilando a versão TUI para o host: p2p-irc-tui...${NC}"
if go build -o "${BIN_DIR}/p2p-irc-tui" ./cmd/p2p-irc-tui; then
    echo -e "${GREEN}Versão TUI compilada com sucesso!${NC}"
else
    echo -e "${RED}Falha ao compilar a versão TUI.${NC}"
    exit 1
fi

# Compilação cruzada da versão TUI para várias plataformas
echo -e "${YELLOW}Iniciando compilação cruzada da versão TUI...${NC}"
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
  echo -e "Compilando para ${GOOS}/${GOARCH}..."
  if CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" go build -o "${out}" ./cmd/p2p-irc-tui; then
      echo -e "${GREEN}Sucesso:${NC} ${out}"
  else
      echo -e "${RED}Falha:${NC} ${out}"
  fi
done

echo -e "${GREEN}Todos os binários foram criados em ${BIN_DIR}${NC}"

# Executa testes automatizados (inclui testes multi-peer/canal)
echo -e "${YELLOW}Executando testes automatizados...${NC}"
if go test ./... -v; then
    echo -e "${GREEN}Todos os testes passaram com sucesso!${NC}"
else
    echo -e "${RED}Algum teste falhou. Corrija antes de prosseguir.${NC}"
    exit 1
fi

echo -e "\nConsulte a documentação dos scripts e comandos dinâmicos em docs/SCRIPTS.md e docs/EXTENSAO_COMANDOS_IRC.md.\n"
