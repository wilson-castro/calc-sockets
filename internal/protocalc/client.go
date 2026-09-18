package protocalc

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"

	"calcsockets/internal/protocalc/calcpb"
	"calcsockets/internal/shared/calc"
	"calcsockets/internal/shared/metrics"
)

// ClientOptions tem o mesmo significado que em tcp.ClientOptions.
type ClientOptions struct {
	DialTimeout time.Duration
	IOTimeout   time.Duration
}

// Client é o CalcClientProto. Usa uma única conexão TCP para toda a sequência.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	opts   ClientOptions
}

// Dial estabelece a conexão com o servidor Protobuf.
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

// Exchange envia a requisição serializada e lê a resposta.
//
// RequestBytes e ResponseBytes registram o tamanho da mensagem protobuf sem o
// prefixo varint, comparável ao tamanho da linha textual sem o '\n'.
func (c *Client) Exchange(request calc.Request) metrics.Sample {
	message := toProto(request)
	sample := metrics.Sample{
		Seq:          request.Seq,
		Request:      describeRequest(message),
		RequestBytes: proto.Size(message),
		Attempts:     1,
	}

	start := time.Now()
	if err := c.conn.SetDeadline(start.Add(c.opts.IOTimeout)); err != nil {
		return lost(sample, err)
	}
	if _, err := protodelim.MarshalTo(c.conn, message); err != nil {
		return lost(sample, err)
	}
	response := &calcpb.CalcResponse{}
	if err := (protodelim.UnmarshalOptions{MaxSize: maxMessageSize}).UnmarshalFrom(c.reader, response); err != nil {
		return lost(sample, err)
	}
	sample.RTT = time.Since(start)

	if response.GetSeq() != request.Seq {
		return lost(sample, fmt.Errorf("resposta com seq %d fora de ordem", response.GetSeq()))
	}
	sample.Reply = describeResponse(response)
	sample.ResponseBytes = proto.Size(response)
	return sample
}

func lost(sample metrics.Sample, cause error) metrics.Sample {
	sample.Lost = true
	sample.Failure = cause.Error()
	return sample
}
