// Package app concentra o que todo executável de cmd/ faz antes e depois da
// sua lógica: carregar a configuração, montar loggers, tratar Ctrl+C e SIGTERM,
// e converter erro em código de saída.
package app

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	// Embute a base de fusos para que a variável TZ repassada ao contêiner
	// funcione mesmo sem /usr/share/zoneinfo instalado na imagem.
	_ "time/tzdata"

	"calcsockets/internal/shared/config"
	"calcsockets/internal/shared/console"
	"calcsockets/internal/shared/logger"
	"calcsockets/internal/shared/metrics"
)

// Env é o ambiente entregue à função principal de cada executável.
type Env struct {
	Config config.Config
	// Log é o logger do componente que está executando.
	Log   *slog.Logger
	Color bool
}

// Logger cria o logger de outro componente com o mesmo destino e o mesmo modo de cor.
func (e Env) Logger(component string) *slog.Logger {
	return e.LoggerAt(component, logger.ParseLevel(e.Config.Log.Level))
}

// LoggerAt é como Logger, mas com nível mínimo explícito.
func (e Env) LoggerAt(component string, level slog.Level) *slog.Logger {
	return logger.New(component, os.Stdout, logger.Options{Level: level, Color: e.Color})
}

// PrintSummary exibe a tabela de resumo de um cliente.
func (e Env) PrintSummary(title string, summary metrics.Summary) {
	table := console.Table{
		Title:   title,
		Headers: console.SummaryHeaders,
		Rows:    [][]string{console.SummaryRow(summary.Protocol, summary)},
	}
	fmt.Print(table.Render(e.Color))
}

// Run executa main como o componente informado e encerra o processo.
//
// Flags registradas pelo chamador antes de Run são interpretadas aqui. O
// contexto entregue a main é cancelado por SIGINT ou SIGTERM.
func Run(component string, main func(ctx context.Context, env Env) error) {
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: configuração inválida: %v\n", component, err)
		os.Exit(2)
	}

	color := logger.ColorEnabled(cfg.Log.Color, os.Stdout)
	env := Env{Config: cfg, Color: color}
	env.Log = env.Logger(component)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := main(ctx, env); err != nil {
		env.Log.Error("execução encerrada com erro", "erro", err)
		stop()
		os.Exit(1)
	}
}
