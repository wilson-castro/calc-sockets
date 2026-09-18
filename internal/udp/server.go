// Package udp implementa a Parte 1: CalcServerUDP e CalcClientUDP sobre
// datagramas, com perda simulada no servidor e timeout mais retransmissão no cliente.
package udp

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"math/rand/v2"
	"net"
	"sync"
	"sync/atomic"

	"calcsockets/internal/shared/random"
	"calcsockets/internal/shared/textprotocol"
)

// maxDatagramSize comporta qualquer datagrama UDP sobre IPv4.
const maxDatagramSize = 65507

// ServerOptions configura a falha de omissão simulada.
type ServerOptions struct {
	// LossRate é a fração, entre 0 e 1, dos datagramas recebidos que o servidor
	// descarta sem responder.
	LossRate float64
	// LossSeed igual a zero sorteia perdas diferentes a cada execução.
	LossSeed int64
}

// Server é o CalcServerUDP. Um único socket atende todos os clientes: o laço de
// leitura só recebe e sorteia a perda, e cada resposta é calculada e enviada em
// goroutine própria, de modo que um cliente não bloqueia os demais.
type Server struct {
	conn     *net.UDPConn
	loss     *lossSimulator
	log      *slog.Logger
	replying sync.WaitGroup

	received atomic.Uint64
	dropped  atomic.Uint64
	answered atomic.Uint64
}

// Listen abre o socket UDP no endereço host:porta. Porta 0 escolhe uma porta livre.
func Listen(address string, opts ServerOptions, log *slog.Logger) (*Server, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return &Server{conn: conn, loss: newLossSimulator(opts.LossRate, opts.LossSeed), log: log}, nil
}

// Addr é o endereço efetivo de escuta, útil quando Listen recebeu porta 0.
func (s *Server) Addr() string { return s.conn.LocalAddr().String() }

// Serve atende datagramas até ctx ser cancelado ou Close ser chamado, e então
// espera as respostas em andamento. Devolve nil no encerramento normal.
func (s *Server) Serve(ctx context.Context) error {
	stopOnCancel := context.AfterFunc(ctx, func() { s.conn.Close() })
	defer stopOnCancel()

	s.log.Info("servidor UDP escutando", "endereço", s.Addr(), "perda_simulada", s.LossRate())
	buffer := make([]byte, maxDatagramSize)
	for {
		n, client, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				s.replying.Wait()
				s.logTotals()
				return nil
			}
			s.log.Warn("falha ao ler datagrama", "erro", err)
			continue
		}

		s.received.Add(1)
		message := string(buffer[:n])
		if s.loss.shouldDrop() {
			s.dropped.Add(1)
			s.log.Warn("descartado (perda simulada)", "cliente", client, "req", message)
			continue
		}

		s.replying.Add(1)
		go s.reply(message, client)
	}
}

// Close encerra o socket, o que faz Serve retornar.
func (s *Server) Close() error { return s.conn.Close() }

func (s *Server) reply(message string, client *net.UDPAddr) {
	defer s.replying.Done()

	response := textprotocol.Respond(message)
	if _, err := s.conn.WriteToUDP([]byte(response), client); err != nil {
		if !errors.Is(err, net.ErrClosed) {
			s.log.Warn("falha ao responder", "cliente", client, "erro", err)
		}
		return
	}
	s.answered.Add(1)
	s.log.Info("requisição respondida", "cliente", client, "req", message, "resp", response)
}

func (s *Server) logTotals() {
	s.log.Info("servidor UDP encerrado",
		"recebidos", s.received.Load(), "descartados", s.dropped.Load(), "respondidos", s.answered.Load())
}

// SetLossRate troca a taxa de perda com o servidor em execução. O painel em tempo
// real usa isso para mostrar o efeito da perda sem reiniciar o servidor.
func (s *Server) SetLossRate(rate float64) { s.loss.setRate(rate) }

// LossRate é a taxa de perda em vigor.
func (s *Server) LossRate() float64 { return s.loss.currentRate() }

// lossSimulator sorteia quais datagramas o servidor "perde".
type lossSimulator struct {
	// rate guarda os bits de um float64 para ser lido e trocado sem trava.
	rate atomic.Uint64
	mu   sync.Mutex
	rng  *rand.Rand
}

func newLossSimulator(rate float64, seed int64) *lossSimulator {
	simulator := &lossSimulator{rng: random.New(seed)}
	simulator.setRate(rate)
	return simulator
}

func (l *lossSimulator) setRate(rate float64) { l.rate.Store(math.Float64bits(min(max(rate, 0), 1))) }

func (l *lossSimulator) currentRate() float64 { return math.Float64frombits(l.rate.Load()) }

func (l *lossSimulator) shouldDrop() bool {
	rate := l.currentRate()
	if rate <= 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rng.Float64() < rate
}
