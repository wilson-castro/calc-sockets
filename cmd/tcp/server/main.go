// Command server é o CalcServerTCP (Parte 2).
package main

import (
	"context"

	"calcsockets/internal/shared/app"
	"calcsockets/internal/tcp"
)

func main() {
	app.Run("tcp-server", func(ctx context.Context, env app.Env) error {
		server, err := tcp.Listen(env.Config.ListenAddress(env.Config.Network.TCPPort), env.Log)
		if err != nil {
			return err
		}
		return server.Serve(ctx)
	})
}
