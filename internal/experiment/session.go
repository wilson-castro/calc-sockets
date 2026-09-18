package experiment

import (
	"context"
	"io"
	"strings"
	"time"

	"calcsockets/internal/shared/config"
	"calcsockets/internal/shared/workload"
)

// NewSettings monta os parâmetros do experimento a partir da configuração.
// A carga usa experiment.seed para que todas as execuções vejam as mesmas requisições.
func NewSettings(cfg config.Config) Settings {
	return Settings{
		Requests:       workload.Generate(cfg.Client.Requests, cfg.Experiment.Seed),
		LossRates:      cfg.Experiment.LossRates,
		LossSeed:       cfg.Experiment.Seed,
		UDPTimeout:     cfg.UDPTimeout(),
		UDPMaxAttempts: cfg.Client.UDPMaxAttempts,
		DialTimeout:    cfg.DialTimeout(),
		IOTimeout:      cfg.IOTimeout(),
	}
}

// RunAndSave executa todos os cenários, exibe as tabelas em out e grava o
// relatório em experiment.resultsDir. Devolve os caminhos gravados.
func RunAndSave(ctx context.Context, cfg config.Config, loggers Loggers, out io.Writer, color bool) ([]string, error) {
	results, err := Run(ctx, NewSettings(cfg), loggers)
	if err != nil {
		return nil, err
	}

	report := Report{
		GeneratedAt:    time.Now(),
		Requests:       cfg.Client.Requests,
		Seed:           cfg.Experiment.Seed,
		UDPTimeoutMS:   cfg.UDPTimeout().Milliseconds(),
		UDPMaxAttempts: cfg.Client.UDPMaxAttempts,
		Results:        results,
	}
	if _, err := io.WriteString(out, report.ConsoleTables(color)); err != nil {
		return nil, err
	}
	return report.Save(cfg.Experiment.ResultsDir)
}

// Component devolve o nome de logger <parte>-<papel> de um protocolo, padrão que
// o pacote logger usa para escolher a cor.
func Component(protocol, role string) string {
	part := strings.ToLower(protocol)
	if protocol == ProtocolProtobuf {
		part = "proto"
	}
	return part + "-" + role
}
