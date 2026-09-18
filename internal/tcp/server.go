// Package tcp implementa a Parte 2: CalcServerTCP e CalcClientTCP com o mesmo
// protocolo textual da Parte 1, uma mensagem por linha terminada em '\n'.
// Não há retransmissão na aplicação: o TCP já garante entrega e ordem.
package tcp

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"calcsockets/internal/shared/textprotocol"
)

// acceptRetryDelay evita laço ocupado quando Accept falha repetidamente, por
// exemplo ao atingir o limite de descritores abertos.
const acceptRetryDelay = 100 * time.Millisecond

// Server é o CalcServerTCP. Cada conexão aceita é atendida por uma goroutine
// própria, o equivalente em Go a uma thread por cliente.
type Server struct {
	listener net.Listener
	log      *slog.Logger
	handlers sync.WaitGroup

	mu     sync.Mutex
	active map[net.Conn]struct{}
}

// Listen abre o socket de escuta em host:porta. Porta 0 escolhe uma porta livre.
func Listen(address string, log *slog.Logger) (*Server, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	return &Server{listener: listener, log: log, active: map[net.Conn]struct{}{}}, nil
}

// Addr é o endereço efetivo de escuta.
func (s *Server) Addr() string { return s.listener.Addr().String() }

// Serve aceita conexões até ctx ser cancelado ou Close ser chamado. No
// encerramento fecha as conexões abertas e espera suas goroutines.
func (s *Server) Serve(ctx context.Context) error {
	stopOnCancel := context.AfterFunc(ctx, func() { s.Close() })
	defer stopOnCancel()

	s.log.Info("servidor TCP escutando", "endereço", s.Addr())
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				s.closeActive()
				s.handlers.Wait()
				s.log.Info("servidor TCP encerrado")
				return nil
			}
			s.log.Warn("falha ao aceitar conexão", "erro", err)
			time.Sleep(acceptRetryDelay)
			continue
		}

		s.track(conn)
		s.handlers.Add(1)
		go s.handle(conn)
	}
}

// Close para de aceitar conexões, o que faz Serve retornar.
func (s *Server) Close() error { return s.listener.Close() }

func (s *Server) handle(conn net.Conn) {
	defer s.handlers.Done()
	defer s.untrack(conn)

	log := s.log.With("cliente", conn.RemoteAddr().String())
	log.Info("cliente conectado")

	answered := 0
	scanner := bufio.NewScanner(conn)
	writer := bufio.NewWriter(conn)
	for scanner.Scan() {
		// TrimRight aceita também clientes que terminam linhas com "\r\n", como o telnet.
		request := strings.TrimRight(scanner.Text(), "\r")
		response := textprotocol.Respond(request)

		if _, err := writer.WriteString(response + "\n"); err != nil {
			log.Warn("falha ao responder", "erro", err)
			return
		}
		if err := writer.Flush(); err != nil {
			log.Warn("falha ao responder", "erro", err)
			return
		}
		answered++
		log.Info("requisição respondida", "req", request, "resp", response)
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Warn("conexão interrompida", "erro", err)
	}
	log.Info("cliente desconectado", "respondidas", answered)
}

func (s *Server) track(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[conn] = struct{}{}
}

func (s *Server) untrack(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, conn)
	conn.Close()
}

func (s *Server) closeActive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.active {
		conn.Close()
	}
}
