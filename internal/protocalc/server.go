package protocalc

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"

	"calcsockets/internal/protocalc/calcpb"
)

// maxMessageSize limita o prefixo de tamanho lido da rede, para que um cliente
// defeituoso não faça o servidor alocar memória arbitrária.
const maxMessageSize = 4 << 10

const acceptRetryDelay = 100 * time.Millisecond

// Server é o CalcServerProto: mesma estrutura do CalcServerTCP, com uma
// goroutine por conexão, trocando linhas de texto por mensagens protobuf delimitadas.
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

// Serve aceita conexões até ctx ser cancelado ou Close ser chamado.
func (s *Server) Serve(ctx context.Context) error {
	stopOnCancel := context.AfterFunc(ctx, func() { s.Close() })
	defer stopOnCancel()

	s.log.Info("servidor Protobuf escutando", "endereço", s.Addr())
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				s.closeActive()
				s.handlers.Wait()
				s.log.Info("servidor Protobuf encerrado")
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

	reader := bufio.NewReader(conn)
	unmarshal := protodelim.UnmarshalOptions{MaxSize: maxMessageSize}
	answered := 0
	for {
		request := &calcpb.CalcRequest{}
		if err := unmarshal.UnmarshalFrom(reader, request); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				log.Warn("mensagem inválida, encerrando conexão", "erro", err)
			}
			break
		}

		response := respond(request)
		if _, err := protodelim.MarshalTo(conn, response); err != nil {
			log.Warn("falha ao responder", "erro", err)
			break
		}
		answered++
		log.Info("requisição respondida",
			"req", describeRequest(request), "resp", describeResponse(response),
			"bytes_req", proto.Size(request), "bytes_resp", proto.Size(response))
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
