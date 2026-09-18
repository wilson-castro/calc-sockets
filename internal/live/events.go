package live

import (
	"strings"
	"sync"
)

// EventLog guarda as últimas linhas de log dos servidores e clientes do painel.
// Serve de destino (io.Writer) para o pacote logger, que escreve uma linha
// completa por chamada.
type EventLog struct {
	mu    sync.Mutex
	lines []string
	limit int
}

// NewEventLog cria um registro que mantém no máximo limit linhas.
func NewEventLog(limit int) *EventLog {
	return &EventLog{limit: limit}
}

// Write acrescenta as linhas de p, descartando as mais antigas acima do limite.
func (e *EventLog) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		e.lines = append(e.lines, line)
	}
	if excess := len(e.lines) - e.limit; excess > 0 {
		e.lines = append([]string(nil), e.lines[excess:]...)
	}
	return len(p), nil
}

// Last devolve até n linhas, da mais antiga para a mais recente.
func (e *EventLog) Last(n int) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	start := max(len(e.lines)-n, 0)
	return append([]string(nil), e.lines[start:]...)
}
