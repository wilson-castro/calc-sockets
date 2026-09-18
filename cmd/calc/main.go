// Command calc é o console interativo da versão de produção: um menu para executar
// cada parte da atividade, os testes, o relatório e o painel em tempo real.
//
// Argumentos são opções do menu lidas antes do teclado. Por exemplo, `calc 5` abre
// direto o painel e `calc 6 t 0` roda todos os testes e sai.
package main

import (
	"context"
	"flag"
	"os"

	"calcsockets/internal/interactive"
	"calcsockets/internal/shared/app"
)

// version é definida na compilação da release com -ldflags "-X main.version=...".
var version = "dev"

func main() {
	app.Run("console", func(ctx context.Context, env app.Env) error {
		return interactive.New(interactive.Options{
			Config:  env.Config,
			Version: version,
			Color:   env.Color,
			Script:  flag.Args(),
			In:      os.Stdin,
			Out:     os.Stdout,
		}).Run(ctx)
	})
}
