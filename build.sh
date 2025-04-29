#!/bin/bash

# Script de compilação e configuração para o P2P-IRC
# Este script compila o projeto e configura para que o usuário não precise especificar portas manualmente

# Cores para saída
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Iniciando compilação do P2P-IRC...${NC}"

# Verifica se Go está instalado
if ! command -v go &> /dev/null; then
    echo -e "${RED}Erro: Go não está instalado. Por favor, instale o Go antes de continuar.${NC}"
    exit 1
fi

# Cria diretórios necessários se não existirem
mkdir -p history
mkdir -p configs

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

[ui]
# Configurações da interface do usuário
debugMode = false
maxLogLines = 100
historyDir = "history"
EOF

echo -e "${GREEN}Configuração gerada com porta aleatória: $RANDOM_PORT${NC}"

# Compila o projeto
echo -e "${YELLOW}Compilando o projeto...${NC}"
if go build -o p2p-irc ./cmd/p2p-irc/main.go; then
    echo -e "${GREEN}Compilação concluída com sucesso!${NC}"
    
    # Torna o executável executável
    chmod +x p2p-irc
    
    echo -e "${YELLOW}Instruções de uso:${NC}"
    echo -e "  ${GREEN}./p2p-irc${NC} - Executa o cliente com configurações padrão"
    echo -e "  ${GREEN}./p2p-irc --debug${NC} - Executa com modo de depuração ativado"
    echo -e "  ${GREEN}./p2p-irc --peers ip1:porta1,ip2:porta2${NC} - Conecta a peers específicos"
    echo -e "\nO cliente está configurado para usar a porta $RANDOM_PORT automaticamente."
    echo -e "Você não precisa especificar a porta manualmente."
else
    echo -e "${RED}Erro durante a compilação.${NC}"
    exit 1
fi
