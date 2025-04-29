# Plano de Unificação dos Clientes IRC

## Visão Geral

Este documento detalha o plano para unificar os clientes IRC em uma única interface de usuário que ofereça uma experiência excelente e consistente. Atualmente, o projeto mantém dois clientes separados (p2p-irc e p2p-irc-tui), o que gera confusão e duplicação de código.

## Problemas Atuais

1. **Duplicação de Código**: Manter dois clientes separados resulta em duplicação de código e esforço de desenvolvimento.
2. **Experiência Inconsistente**: Os usuários têm experiências diferentes dependendo do cliente que escolhem.
3. **Manutenção Complexa**: Corrigir bugs e adicionar recursos requer alterações em dois lugares diferentes.
4. **Problemas de Interface**: Sobreposição de frames e outros problemas visuais prejudicam a experiência do usuário.

## Solução Proposta

### 1. Cliente Unificado

Criar um único cliente IRC que combine o melhor dos dois clientes existentes:

- **Nome**: `p2p-irc`
- **Interface**: TUI moderna e intuitiva
- **Recursos**: Todos os recursos atualmente disponíveis em ambos os clientes

### 2. Nova Estrutura de Diretórios

```
p2p-irc/
├── cmd/
│   └── p2p-irc/         # Único ponto de entrada
├── internal/
│   ├── ui/              # Interface unificada
│   ├── core/            # Lógica principal do IRC
│   ├── discovery/       # Descoberta de peers
│   ├── transport/       # Camada de transporte
│   └── ...
└── ...
```

### 3. Melhorias na Interface

A nova interface unificada incluirá:

- **Layout de Duas Colunas**: Canais e peers à esquerda, chat e logs à direita
- **Sistema de Abas**: Navegação fácil entre diferentes conversas
- **Área de Logs Separada**: Separação clara entre mensagens de sistema e chat
- **Notificações Visuais**: Indicadores para canais com mensagens não lidas
- **Suporte a Temas**: Personalização visual da interface

### 4. Plano de Implementação

#### Fase 1: Preparação

1. Criar branch de desenvolvimento para a unificação
2. Analisar e documentar todos os recursos de ambos os clientes
3. Identificar componentes comuns que podem ser reutilizados

#### Fase 2: Implementação da Interface Unificada

1. Implementar o novo layout de interface
2. Migrar recursos do cliente TUI experimental
3. Integrar todos os comandos IRC
4. Implementar sistema de buffer para logs e mensagens

#### Fase 3: Migração e Testes

1. Migrar usuários e configurações existentes
2. Realizar testes extensivos com diferentes cenários
3. Coletar feedback de usuários

#### Fase 4: Lançamento

1. Atualizar documentação
2. Remover código obsoleto
3. Lançar a versão unificada

## Benefícios Esperados

1. **Experiência Consistente**: Todos os usuários terão a mesma experiência de alta qualidade
2. **Código Mais Limpo**: Menos duplicação e melhor organização
3. **Manutenção Simplificada**: Apenas um cliente para manter e atualizar
4. **Desenvolvimento Mais Rápido**: Novos recursos podem ser implementados mais rapidamente

## Próximos Passos Imediatos

1. Finalizar as correções na interface atual para resolver problemas de sobreposição
2. Criar um protótipo da interface unificada
3. Apresentar o plano para a equipe e coletar feedback
4. Iniciar a implementação da Fase 1

## Conclusão

A unificação dos clientes IRC em uma única interface proporcionará uma experiência de usuário superior, simplificará o desenvolvimento e manutenção, e permitirá um foco maior na adição de novos recursos em vez de manter múltiplas implementações.
