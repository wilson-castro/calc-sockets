// Command server é o CalcServerProto (Parte 4).
package main

import (
	"context"

	"calcsockets/internal/protocalc"
	"calcsockets/internal/shared/app"
)

func main() {
	app.Run("proto-server", func(ctx context.Context, env app.Env) error {
		server, err := protocalc.Listen(env.Config.ListenAddress(env.Config.Network.ProtoPort), env.Log)
		if err != nil {
			return err
		}
		return server.Serve(ctx)
	})
}
