// Command client é o CalcClientTCP (Parte 2): mesma sequência do cliente UDP,
// sem retransmissão.
package main

import (
	"context"

	"calcsockets/internal/shared/app"
	"calcsockets/internal/shared/sequence"
	"calcsockets/internal/shared/workload"
	"calcsockets/internal/tcp"
)

func main() {
	app.Run("tcp-client", func(ctx context.Context, env app.Env) error {
		cfg := env.Config
		client, err := tcp.Dial(cfg.ServerAddress(cfg.Network.TCPPort),
			tcp.ClientOptions{DialTimeout: cfg.DialTimeout(), IOTimeout: cfg.IOTimeout()}, env.Log)
		if err != nil {
			return err
		}
		defer client.Close()

		requests := workload.Generate(cfg.Client.Requests, cfg.Client.WorkloadSeed)
		summary := sequence.Run(ctx, "TCP", client, requests, env.Log)
		env.PrintSummary("CalcClientTCP", summary)
		return nil
	})
}
