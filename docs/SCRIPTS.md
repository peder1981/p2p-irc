# Documentação dos Scripts do Projeto p2p-irc

Este documento descreve o propósito, uso e integração dos scripts `.sh` presentes no repositório p2p-irc, considerando as evoluções recentes (modularização, testes multi-peer, plugins, comandos dinâmicos).

---

## build.sh

Script principal para build e testes dos binários do p2p-irc.

### Funcionalidades
- Resolve dependências Go (`go mod tidy` e `go mod download`)
- Compila binários GUI (`p2p-irc`) e TUI (`p2p-irc-tui`) para o host
- Faz build cruzado da versão TUI para Linux, macOS e Windows (amd64/arm64)
- **Executa todos os testes automatizados** (incluindo testes multi-peer/canal)
- Exibe mensagens de sucesso/falha coloridas

### Uso
```bash
./build.sh
```
Os binários gerados ficam em `./bin/`.

### Observações
- Após o build, todos os testes são executados automaticamente. Falhas nos testes abortam o processo.
- Para detalhes sobre comandos IRC dinâmicos/plugins, consulte `docs/EXTENSAO_COMANDOS_IRC.md`.

---

## bitchat/setup.sh

Script de configuração inicial do módulo Bitchat (Swift/iOS/macOS).

### Funcionalidades
- Gera projeto Xcode via XcodeGen (se instalado)
- Exibe instruções para abrir o projeto via Xcode ou Swift Package Manager
- Explica a estrutura do projeto Bitchat
- **Recomenda testar Bluetooth Mesh em dispositivos físicos**

### Integração com p2p-irc
- Para integração mesh/IRC, consulte a documentação de arquitetura em `docs/ARQUITETURA.md` e exemplos de integração em `docs/EXEMPLOS_IRCROUTER_COMMANDHISTORY.md`.
- Se for usar mesh com Go, verifique dependências e compatibilidade entre os módulos.

### Uso
```bash
cd bitchat
./setup.sh
```

---

## git-author-rewrite.sh

Script utilitário para reescrever autor/commit do histórico Git.

### Uso
```bash
./git-author-rewrite.sh
```
- Edite as variáveis `OLD_EMAIL`, `CORRECT_NAME`, `CORRECT_EMAIL` conforme necessário antes de rodar.
- Não interfere no build ou execução do sistema.

---

## Fluxo recomendado para desenvolvedores

1. **Compilar e testar tudo:**
   ```bash
   ./build.sh
   ```
   - Todos os testes automatizados (incluindo integração multi-peer/canal com MockPeerDiscovery) são executados e devem passar.
2. **Rodar testes determinísticos (mock) e multi-peer/canal manualmente:**
   ```bash
   go test ./tests -v
   ```
   - Os testes em `tests/mesh_integration_test.go` e outros cobrem toda a lógica crítica do sistema de forma determinística, sem dependência de rede real.
   - O plano de cobertura e próximos passos está documentado em `docs/PLANO_DE_TESTES_E_EVOLUCAO.md`.
3. **Adicionar comandos/plugins dinâmicos:**
   - Siga os exemplos de `docs/EXTENSAO_COMANDOS_IRC.md`.
4. **Integrar Bitchat:**
   - Veja instruções em `bitchat/setup.sh` e docs de arquitetura.

---

## Referências
- [EXTENSAO_COMANDOS_IRC.md](./EXTENSAO_COMANDOS_IRC.md): Como criar e registrar comandos IRC em tempo real.
- [EXEMPLOS_IRCROUTER_COMMANDHISTORY.md](./EXEMPLOS_IRCROUTER_COMMANDHISTORY.md): Exemplos práticos de roteamento e histórico de comandos.
- [ARQUITETURA.md](./ARQUITETURA.md): Visão geral da arquitetura e pontos de integração mesh/IRC.
