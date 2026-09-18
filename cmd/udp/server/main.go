// Command server é o CalcServerUDP (Parte 1).
//
// Lê a configuração de config.json e de variáveis CALC_*. A única flag,
// --loss-rate, existe porque o enunciado a cita e tem precedência sobre
// udp.lossRate e CALC_LOSS_RATE.
package main

import (
	"context"
	"flag"

	"calcsockets/internal/shared/app"
	"calcsockets/internal/udp"
)

func main() {
	lossRate := flag.Float64("loss-rate", -1, "fração de datagramas descartados, entre 0 e 1 (padrão: udp.lossRate do config.json)")

	app.Run("udp-server", func(ctx context.Context, env app.Env) error {
		opts := udp.ServerOptions{LossRate: env.Config.UDP.LossRate, LossSeed: env.Config.UDP.LossSeed}
		if *lossRate >= 0 {
			opts.LossRate = *lossRate
		}

		server, err := udp.Listen(env.Config.ListenAddress(env.Config.Network.UDPPort), opts, env.Log)
		if err != nil {
			return err
		}
		return server.Serve(ctx)
	})
}
