package ui

import "sync"

// CommandHistory gerencia o histórico de comandos com navegação
// thread-safe e limite máximo de itens.
type CommandHistory struct {
	commands []string
	limit    int
	index    int
	mu       sync.RWMutex
}

// NewCommandHistory cria um novo histórico com limite máximo.
func NewCommandHistory(limit int) *CommandHistory {
	return &CommandHistory{
		commands: make([]string, 0, limit),
		limit:    limit,
		index:    0,
	}
}

// Add adiciona um comando ao histórico (não adiciona duplicado sequencial).
func (h *CommandHistory) Add(cmd string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.commands) == 0 || h.commands[len(h.commands)-1] != cmd {
		if len(h.commands) >= h.limit {
			h.commands = h.commands[1:]
		}
		h.commands = append(h.commands, cmd)
	}
	h.index = len(h.commands)
}

// Prev retorna o comando anterior no histórico.
func (h *CommandHistory) Prev() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.commands) == 0 {
		return ""
	}
	if h.index > 0 {
		h.index--
	}
	return h.commands[h.index]
}

// Next retorna o próximo comando no histórico (ou vazio se no fim).
func (h *CommandHistory) Next() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.commands) == 0 {
		return ""
	}
	if h.index < len(h.commands)-1 {
		h.index++
		return h.commands[h.index]
	}
	h.index = len(h.commands)
	return ""
}

// ResetIndex reposiciona o índice para o fim do histórico.
func (h *CommandHistory) ResetIndex() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.index = len(h.commands)
}
