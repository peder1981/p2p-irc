package ui

import (
	"strings"
	"sync"
)

// IRCCommand representa um comando IRC parseado
// Ex: /who, /msg nick texto
// Pode ser expandido para conter argumentos, etc.
type IRCCommand struct {
	Raw   string
	Name  string
	Args  []string
}

// ParseIRCCommand faz parsing simples de um input IRC
func ParseIRCCommand(input string) (*IRCCommand, bool) {
	if !strings.HasPrefix(input, "/") {
		return nil, false
	}
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil, false
	}
	return &IRCCommand{
		Raw:  input,
		Name: strings.TrimPrefix(parts[0], "/"),
		Args: parts[1:],
	}, true
}

// IRCDispatcher define a assinatura para despachantes de comandos
// Retorna true se o comando foi tratado
//
type IRCDispatcher func(cmd *IRCCommand, g *GUI, fallback func(string)) bool

// IRCRouter centraliza o roteamento de comandos IRC
// Permite registro dinâmico de comandos e handlers
// Thread-safe para registro e despacho
type IRCRouter struct {
	mu       sync.RWMutex
	registry map[string]IRCDispatcher
}

// NewIRCRouter cria um novo roteador vazio
func NewIRCRouter() *IRCRouter {
	return &IRCRouter{
		registry: make(map[string]IRCDispatcher),
	}
}

// Register registra um handler para um comando
func (r *IRCRouter) Register(command string, handler IRCDispatcher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registry[command] = handler
}

// Dispatch roteia o comando para o handler apropriado
func (r *IRCRouter) Dispatch(input string, g *GUI, fallback func(string)) bool {
	cmd, ok := ParseIRCCommand(input)
	if !ok {
		return false
	}
	r.mu.RLock()
	handler, found := r.registry[cmd.Name]
	r.mu.RUnlock()
	if found {
		return handler(cmd, g, fallback)
	}
	return false
}
