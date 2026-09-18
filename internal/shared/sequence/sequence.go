// Package sequence executa a sequência de requisições comum aos três clientes:
// uma requisição por vez, aguardando a resposta antes de enviar a próxima.
package sequence

import (
	"context"
	"log/slog"
	"time"

	"calcsockets/internal/shared/calc"
	"calcsockets/internal/shared/metrics"
)

// Exchanger envia uma requisição e bloqueia até obter resposta ou desistir.
// Cada transporte (UDP, TCP, Protobuf) fornece sua implementação.
type Exchanger interface {
	Exchange(request calc.Request) metrics.Sample
}

// Run envia as requisições em ordem e devolve o resumo da execução.
// Um cancelamento de ctx interrompe a sequência entre duas requisições.
func Run(ctx context.Context, protocol string, exchanger Exchanger, requests []calc.Request, log *slog.Logger) metrics.Summary {
	log.Info("iniciando sequência", "protocolo", protocol, "requisições", len(requests))

	samples := make([]metrics.Sample, 0, len(requests))
	start := time.Now()
	for _, request := range requests {
		if ctx.Err() != nil {
			log.Warn("sequência interrompida", "enviadas", len(samples))
			break
		}
		sample := exchanger.Exchange(request)
		logSample(log, sample)
		samples = append(samples, sample)
	}

	return metrics.Summarize(protocol, samples, time.Since(start))
}

func logSample(log *slog.Logger, sample metrics.Sample) {
	if sample.Lost {
		log.Error("requisição perdida", "seq", sample.Seq, "req", sample.Request,
			"tentativas", sample.Attempts, "motivo", sample.Failure)
		return
	}
	log.Info("resposta recebida", "seq", sample.Seq, "req", sample.Request, "resp", sample.Reply,
		"rtt", sample.RTT, "tentativas", sample.Attempts)
}
