// Command client é o CalcClientUDP (Parte 1): envia N requisições aleatórias,
// uma por vez, com timeout e retransmissão, e exibe o resumo da execução.
package main

import (
	"context"

	"calcsockets/internal/shared/app"
	"calcsockets/internal/shared/sequence"
	"calcsockets/internal/shared/workload"
	"calcsockets/internal/udp"
)

func main() {
	app.Run("udp-client", func(ctx context.Context, env app.Env) error {
		cfg := env.Config
		client, err := udp.Dial(cfg.ServerAddress(cfg.Network.UDPPort),
			udp.ClientOptions{Timeout: cfg.UDPTimeout(), MaxAttempts: cfg.Client.UDPMaxAttempts}, env.Log)
		if err != nil {
			return err
		}
		defer client.Close()

		requests := workload.Generate(cfg.Client.Requests, cfg.Client.WorkloadSeed)
		summary := sequence.Run(ctx, "UDP", client, requests, env.Log)
		env.PrintSummary("CalcClientUDP", summary)
		return nil
	})
}
