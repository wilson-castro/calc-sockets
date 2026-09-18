package tcp

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"calcsockets/internal/shared/calc"
	"calcsockets/internal/shared/metrics"
	"calcsockets/internal/shared/textprotocol"
)

// ClientOptions limita as esperas do cliente para que um servidor travado não
// trave o cliente. Não são timeouts de retransmissão.
type ClientOptions struct {
	DialTimeout time.Duration
	// IOTimeout limita cada troca requisição-resposta.
	IOTimeout time.Duration
}

// Client é o CalcClientTCP. Usa uma única conexão para toda a sequência.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	opts   ClientOptions
}

// Dial estabelece a conexão. Diferente do UDP, o handshake do TCP detecta aqui
// um servidor ausente, e o erro devolvido costuma ser "connection refused".
func Dial(address string, opts ClientOptions, log *slog.Logger) (*Client, error) {
	conn, err := net.DialTimeout("tcp", address, opts.DialTimeout)
	if err != nil {
		return nil, fmt.Errorf("não foi possível conectar a %s: %w", address, err)
	}
	log.Info("conectado", "servidor", conn.RemoteAddr().String(), "local", conn.LocalAddr().String())
	return &Client{conn: conn, reader: bufio.NewReader(conn), opts: opts}, nil
}

// Close encerra a conexão.
func (c *Client) Close() error { return c.conn.Close() }

// Exchange envia uma linha e lê a linha de resposta. Uma falha de transporte
// marca a requisição como perdida sem nova tentativa.
func (c *Client) Exchange(request calc.Request) metrics.Sample {
	message := textprotocol.EncodeRequest(request)
	sample := metrics.Sample{Seq: request.Seq, Request: message, RequestBytes: len(message), Attempts: 1}

	start := time.Now()
	if err := c.conn.SetDeadline(start.Add(c.opts.IOTimeout)); err != nil {
		return lost(sample, err)
	}
	if _, err := c.conn.Write([]byte(message + "\n")); err != nil {
		return lost(sample, err)
	}
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return lost(sample, err)
	}
	sample.RTT = time.Since(start)

	reply := strings.TrimRight(line, "\r\n")
	response, err := textprotocol.DecodeResponse(reply)
	if err != nil {
		return lost(sample, err)
	}
	if response.Seq != request.Seq {
		return lost(sample, fmt.Errorf("resposta com seq %d fora de ordem", response.Seq))
	}

	sample.Reply = reply
	sample.ResponseBytes = len(reply)
	return sample
}

func lost(sample metrics.Sample, cause error) metrics.Sample {
	sample.Lost = true
	sample.Failure = cause.Error()
	return sample
}
