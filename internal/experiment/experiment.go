// Package experiment implementa a Parte 3: executa o cliente UDP contra servidores
// com cada taxa de perda configurada, depois uma vez contra o servidor TCP e uma
// vez contra o servidor Protobuf, que alimenta a comparação de tamanhos da Parte 4.
//
// Os servidores sobem no mesmo processo, em portas livres de 127.0.0.1, mas a
// comunicação passa pelos sockets reais do sistema operacional, com o mesmo código
// usado pelos executáveis de cmd/.
package experiment

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"calcsockets/internal/protocalc"
	"calcsockets/internal/shared/calc"
	"calcsockets/internal/shared/metrics"
	"calcsockets/internal/shared/sequence"
	"calcsockets/internal/tcp"
	"calcsockets/internal/udp"
)

// Protocolos medidos, usados como rótulo nos relatórios.
const (
	ProtocolUDP      = "UDP"
	ProtocolTCP      = "TCP"
	ProtocolProtobuf = "Protobuf"
)

// Settings define o experimento. Todos os cenários usam a mesma sequência de
// requisições, para que diferenças de tempo e de bytes venham só do transporte.
type Settings struct {
	Requests       []calc.Request
	LossRates      []float64
	LossSeed       int64
	UDPTimeout     time.Duration
	UDPMaxAttempts int
	DialTimeout    time.Duration
	IOTimeout      time.Duration
}

// Loggers fornece o logger de cada papel. Servidores costumam receber um nível
// mais alto para que o terminal mostre só as perdas simuladas.
type Loggers struct {
	Client func(protocol string) *slog.Logger
	Server func(protocol string) *slog.Logger
}

// Result é uma execução do experimento.
type Result struct {
	Scenario string
	Protocol string
	// LossRate só se aplica ao UDP; é zero nos demais.
	LossRate float64
	Summary  metrics.Summary
}

// Run executa os cenários em sequência. Para no primeiro erro de infraestrutura,
// como porta indisponível; perdas de mensagem não são erro, são o que se mede.
func Run(ctx context.Context, settings Settings, loggers Loggers) ([]Result, error) {
	var results []Result
	for _, rate := range settings.LossRates {
		summary, err := RunUDP(ctx, settings, rate, loggers)
		if err != nil {
			return results, fmt.Errorf("cenário UDP %.0f%%: %w", rate*100, err)
		}
		results = append(results, Result{
			Scenario: fmt.Sprintf("UDP perda %.0f%%", rate*100),
			Protocol: ProtocolUDP,
			LossRate: rate,
			Summary:  summary,
		})
	}

	summary, err := RunTCP(ctx, settings, loggers)
	if err != nil {
		return results, fmt.Errorf("cenário TCP: %w", err)
	}
	results = append(results, Result{Scenario: "TCP", Protocol: ProtocolTCP, Summary: summary})

	summary, err = RunProtobuf(ctx, settings, loggers)
	if err != nil {
		return results, fmt.Errorf("cenário Protobuf: %w", err)
	}
	results = append(results, Result{Scenario: "TCP + Protobuf", Protocol: ProtocolProtobuf, Summary: summary})

	return results, nil
}

// servable é o que os três servidores têm em comum.
type servable interface {
	Addr() string
	Serve(ctx context.Context) error
}

// withServer mantém o servidor ativo enquanto run executa e o encerra em seguida.
func withServer(ctx context.Context, server servable, run func(address string) error) error {
	serverCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- server.Serve(serverCtx) }()

	err := run(server.Addr())
	stop()
	if serveErr := <-done; err == nil {
		err = serveErr
	}
	return err
}

// RunUDP executa um cenário UDP isolado com a taxa de perda informada.
func RunUDP(ctx context.Context, s Settings, lossRate float64, loggers Loggers) (metrics.Summary, error) {
	server, err := udp.Listen("127.0.0.1:0", udp.ServerOptions{LossRate: lossRate, LossSeed: s.LossSeed}, loggers.Server(ProtocolUDP))
	if err != nil {
		return metrics.Summary{}, err
	}

	var summary metrics.Summary
	err = withServer(ctx, server, func(address string) error {
		log := loggers.Client(ProtocolUDP)
		client, err := udp.Dial(address, udp.ClientOptions{Timeout: s.UDPTimeout, MaxAttempts: s.UDPMaxAttempts}, log)
		if err != nil {
			return err
		}
		defer client.Close()
		summary = sequence.Run(ctx, ProtocolUDP, client, s.Requests, log)
		return nil
	})
	return summary, err
}

// RunTCP executa o cenário TCP isolado.
func RunTCP(ctx context.Context, s Settings, loggers Loggers) (metrics.Summary, error) {
	server, err := tcp.Listen("127.0.0.1:0", loggers.Server(ProtocolTCP))
	if err != nil {
		return metrics.Summary{}, err
	}

	var summary metrics.Summary
	err = withServer(ctx, server, func(address string) error {
		log := loggers.Client(ProtocolTCP)
		client, err := tcp.Dial(address, tcp.ClientOptions{DialTimeout: s.DialTimeout, IOTimeout: s.IOTimeout}, log)
		if err != nil {
			return err
		}
		defer client.Close()
		summary = sequence.Run(ctx, ProtocolTCP, client, s.Requests, log)
		return nil
	})
	return summary, err
}

// RunProtobuf executa o cenário TCP com Protobuf isolado.
func RunProtobuf(ctx context.Context, s Settings, loggers Loggers) (metrics.Summary, error) {
	server, err := protocalc.Listen("127.0.0.1:0", loggers.Server(ProtocolProtobuf))
	if err != nil {
		return metrics.Summary{}, err
	}

	var summary metrics.Summary
	err = withServer(ctx, server, func(address string) error {
		log := loggers.Client(ProtocolProtobuf)
		client, err := protocalc.Dial(address, protocalc.ClientOptions{DialTimeout: s.DialTimeout, IOTimeout: s.IOTimeout}, log)
		if err != nil {
			return err
		}
		defer client.Close()
		summary = sequence.Run(ctx, ProtocolProtobuf, client, s.Requests, log)
		return nil
	})
	return summary, err
}
