// Command experiment executa a Parte 3 de ponta a ponta e grava o relatório em
// experiment.resultsDir (padrão: results/experiment.md e results/experiment.json).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"calcsockets/internal/experiment"
	"calcsockets/internal/shared/app"
)

func main() {
	app.Run("experiment", func(ctx context.Context, env app.Env) error {
		cfg := env.Config
		env.Log.Info("iniciando experimento", "requisições", cfg.Client.Requests,
			"perdas_udp", fmt.Sprint(cfg.Experiment.LossRates), "seed", cfg.Experiment.Seed)

		paths, err := experiment.RunAndSave(ctx, cfg, Loggers(env), os.Stdout, env.Color)
		if err != nil {
			return err
		}
		env.Log.Info("relatório gravado", "arquivos", strings.Join(paths, ","))
		return nil
	})
}

// Loggers mostra os clientes em INFO e os servidores em WARN: o terminal exibe os
// descartes simulados sem repetir cada resposta, que já aparece no log do cliente.
func Loggers(env app.Env) experiment.Loggers {
	return experiment.Loggers{
		Client: func(protocol string) *slog.Logger {
			return env.Logger(experiment.Component(protocol, "client"))
		},
		Server: func(protocol string) *slog.Logger {
			return env.LoggerAt(experiment.Component(protocol, "server"), slog.LevelWarn)
		},
	}
}
