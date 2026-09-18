// Command client é o CalcClientProto (Parte 4): sequência da Parte 2 com
// mensagens protobuf; a tabela final inclui o tamanho médio das mensagens.
package main

import (
	"context"

	"calcsockets/internal/protocalc"
	"calcsockets/internal/shared/app"
	"calcsockets/internal/shared/sequence"
	"calcsockets/internal/shared/workload"
)

func main() {
	app.Run("proto-client", func(ctx context.Context, env app.Env) error {
		cfg := env.Config
		client, err := protocalc.Dial(cfg.ServerAddress(cfg.Network.ProtoPort),
			protocalc.ClientOptions{DialTimeout: cfg.DialTimeout(), IOTimeout: cfg.IOTimeout()}, env.Log)
		if err != nil {
			return err
		}
		defer client.Close()

		requests := workload.Generate(cfg.Client.Requests, cfg.Client.WorkloadSeed)
		summary := sequence.Run(ctx, "Protobuf", client, requests, env.Log)
		env.PrintSummary("CalcClientProto", summary)
		return nil
	})
}
