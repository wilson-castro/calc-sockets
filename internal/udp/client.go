package udp

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"calcsockets/internal/shared/calc"
	"calcsockets/internal/shared/metrics"
	"calcsockets/internal/shared/textprotocol"
)

// ClientOptions define a política de confiabilidade que o UDP não oferece sozinho.
type ClientOptions struct {
	// Timeout é a espera por resposta antes de retransmitir (enunciado: 500 ms).
	Timeout time.Duration
	// MaxAttempts conta o envio original; esgotado, a requisição é dada como perdida.
	MaxAttempts int
}

// Client é o CalcClientUDP.
type Client struct {
	conn   *net.UDPConn
	opts   ClientOptions
	log    *slog.Logger
	buffer []byte
}

// errNoReply sinaliza que a tentativa terminou sem resposta válida para a requisição.
var errNoReply = errors.New("sem resposta dentro do timeout")

// Dial cria um socket UDP conectado ao servidor. UDP não tem handshake, então
// Dial não detecta servidor ausente; isso aparece como timeout ou porta recusada.
func Dial(address string, opts ClientOptions, log *slog.Logger) (*Client, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, opts: opts, log: log, buffer: make([]byte, maxDatagramSize)}, nil
}

// Close libera o socket. Um Exchange em andamento termina logo em seguida, com a
// requisição marcada como perdida.
func (c *Client) Close() error { return c.conn.Close() }

// Exchange envia a requisição e retransmite a cada timeout até MaxAttempts envios.
func (c *Client) Exchange(request calc.Request) metrics.Sample {
	message := textprotocol.EncodeRequest(request)
	sample := metrics.Sample{Seq: request.Seq, Request: message, RequestBytes: len(message)}

	start := time.Now()
	for attempt := 1; attempt <= c.opts.MaxAttempts; attempt++ {
		sample.Attempts = attempt
		reply, err := c.attempt(request.Seq, message)
		if err == nil {
			sample.RTT = time.Since(start)
			sample.Reply = reply
			sample.ResponseBytes = len(reply)
			return sample
		}
		if errors.Is(err, net.ErrClosed) {
			break
		}
		if attempt < c.opts.MaxAttempts {
			c.log.Warn("sem resposta, retransmitindo", "seq", request.Seq,
				"próxima_tentativa", fmt.Sprintf("%d/%d", attempt+1, c.opts.MaxAttempts), "motivo", err)
		}
	}

	sample.Lost = true
	sample.Failure = fmt.Sprintf("tentativas esgotadas (%d × %v)", c.opts.MaxAttempts, c.opts.Timeout)
	if sample.Attempts < c.opts.MaxAttempts {
		sample.Failure = "socket fechado pelo cliente"
	}
	return sample
}

// attempt faz um envio e espera a resposta com o mesmo número de sequência até o
// fim do timeout. Respostas atrasadas de requisições anteriores são descartadas,
// pois uma retransmissão pode fazer o servidor responder duas vezes.
func (c *Client) attempt(seq uint64, message string) (string, error) {
	deadline := time.Now().Add(c.opts.Timeout)
	if _, err := c.conn.Write([]byte(message)); err != nil {
		if errors.Is(err, net.ErrClosed) {
			return "", err
		}
		return "", c.waitOut(deadline, err)
	}
	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return "", err
	}

	for {
		n, err := c.conn.Read(c.buffer)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return "", errNoReply
			}
			if errors.Is(err, net.ErrClosed) {
				return "", err
			}
			return "", c.waitOut(deadline, err)
		}

		reply := string(c.buffer[:n])
		response, err := textprotocol.DecodeResponse(reply)
		switch {
		case err != nil:
			c.log.Warn("resposta malformada descartada", "resp", reply, "erro", err)
		case response.Seq != seq:
			c.log.Debug("resposta atrasada descartada", "esperado", seq, "resp", reply)
		default:
			return reply, nil
		}
	}
}

// waitOut segura a tentativa até o fim do timeout. Sem isso, um erro imediato,
// como a porta recusada (ICMP port unreachable), consumiria todas as tentativas
// em microssegundos, em vez de dar ao servidor a chance de voltar. Socket fechado
// pelo próprio cliente não passa por aqui: encerra a requisição na hora.
func (c *Client) waitOut(deadline time.Time, cause error) error {
	time.Sleep(time.Until(deadline))
	return fmt.Errorf("%w (%v)", errNoReply, cause)
}
