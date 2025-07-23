package ui

import (
	"testing"
	"sync"
	"fmt"
)

func TestCommandHistoryBasic(t *testing.T) {
	h := NewCommandHistory(3)

	h.Add("/foo")
	h.Add("/bar")
	h.Add("/baz")

	if got := h.Prev(); got != "/baz" {
		t.Errorf("Prev 1: esperado /baz, obtido %q", got)
	}
	if got := h.Prev(); got != "/bar" {
		t.Errorf("Prev 2: esperado /bar, obtido %q", got)
	}
	if got := h.Prev(); got != "/foo" {
		t.Errorf("Prev 3: esperado /foo, obtido %q", got)
	}
	if got := h.Prev(); got != "/foo" {
		t.Errorf("Prev 4: esperado /foo (limite), obtido %q", got)
	}
	if got := h.Next(); got != "/bar" {
		t.Errorf("Next 1: esperado /bar, obtido %q", got)
	}
	if got := h.Next(); got != "/baz" {
		t.Errorf("Next 2: esperado /baz, obtido %q", got)
	}
	if got := h.Next(); got != "" {
		t.Errorf("Next 3: esperado vazio (fim), obtido %q", got)
	}
}

func TestCommandHistoryNoDupSequential(t *testing.T) {
	h := NewCommandHistory(5)
	h.Add("/foo")
	h.Add("/foo")
	h.Add("/bar")
	if len(h.commands) != 2 {
		t.Errorf("Deve evitar duplicados sequenciais, obtido: %v", h.commands)
	}
}

func TestCommandHistoryConcurrency(t *testing.T) {
	h := NewCommandHistory(100)
	wg := sync.WaitGroup{}
	nWriters := 10
	nPerWriter := 100
	for i := 0; i < nWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < nPerWriter; j++ {
				h.Add(fmt.Sprintf("/cmd-%d-%d", id, j))
			}
		}(i)
	}
	wg.Wait()
	// O histórico deve conter no máximo 100 comandos (limite), sem pânico ou corrupção
	if len(h.commands) > 100 {
		t.Errorf("Histórico excedeu o limite: %d", len(h.commands))
	}
}

func TestCommandHistoryLimit(t *testing.T) {
	h := NewCommandHistory(2)
	h.Add("/foo")
	h.Add("/bar")
	h.Add("/baz")
	if len(h.commands) != 2 || h.commands[0] != "/bar" || h.commands[1] != "/baz" {
		t.Errorf("Deve manter apenas os últimos N comandos: %v", h.commands)
	}
}
