package interactive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"calcsockets/internal/experiment"
	"calcsockets/internal/live"
	"calcsockets/internal/shared/console"
	"calcsockets/internal/shared/logger"
	"calcsockets/internal/shared/metrics"
	"calcsockets/internal/shared/workload"
)

// defaultDemoLoss é a perda sugerida na demonstração UDP: alta o bastante para
// que retransmissões apareçam em quase toda execução de 20 requisições.
const defaultDemoLoss = 0.3

// menu define as opções do menu principal, na ordem de exibição.
func (c *Console) menu() []item {
	return []item{
		{"1", "Partes da atividade", "Parte 1 · UDP", "servidor + cliente, perda simulada e retransmissão", logger.Blue, c.demoUDP},
		{"2", "Partes da atividade", "Parte 2 · TCP", "servidor + cliente, entrega confiável", logger.Cyan, c.demo(experiment.ProtocolTCP, "CalcClientTCP")},
		{"3", "Partes da atividade", "Parte 3 · Experimento", "UDP 0/10/30% vs. TCP, grava o relatório", logger.Yellow, c.runExperiment},
		{"4", "Partes da atividade", "Parte 4 · Protobuf", "servidor + cliente com mensagens binárias", logger.Magenta, c.demo(experiment.ProtocolProtobuf, "CalcClientProto")},
		{"5", "Acompanhar", "Tempo real", "painel ao vivo com os três protocolos", logger.Green, c.live},
		{"6", "Acompanhar", "Testes", "testes automatizados de cada parte", logger.Bold, c.chooseTests},
		{"7", "Acompanhar", "Resultados", "tabelas do último experimento", logger.Yellow, c.showResults},
		{"8", "Acompanhar", "Configuração", "valores em vigor (config.json + CALC_*)", logger.Gray, c.showConfig},
	}
}

func (c *Console) demoUDP(ctx context.Context) error {
	rate := defaultDemoLoss
	for {
		answer, ok := c.prompt(ctx, fmt.Sprintf("Taxa de perda entre 0 e 1 [%.1f]", defaultDemoLoss))
		if !ok || answer == "" {
			break
		}
		parsed, err := strconv.ParseFloat(strings.Replace(answer, ",", ".", 1), 64)
		if err == nil && parsed >= 0 && parsed <= 1 {
			rate = parsed
			break
		}
		c.println(c.paint(logger.Red, "digite um número entre 0 e 1, por exemplo 0.3"))
	}

	c.println("")
	summary, err := experiment.RunUDP(ctx, c.demoSettings(), rate, c.demoLoggers())
	if err != nil {
		return err
	}
	c.printSummary("CalcClientUDP", summary)
	return nil
}

// demo devolve a ação de demonstração de um protocolo TCP (texto ou Protobuf).
func (c *Console) demo(protocol, title string) func(context.Context) error {
	run := experiment.RunTCP
	if protocol == experiment.ProtocolProtobuf {
		run = experiment.RunProtobuf
	}
	return func(ctx context.Context) error {
		summary, err := run(ctx, c.demoSettings(), c.demoLoggers())
		if err != nil {
			return err
		}
		c.printSummary(title, summary)
		return nil
	}
}

// demoSettings usa a carga do cliente (client.workloadSeed, aleatória por padrão),
// e não a semente fixa do experimento, para que cada demonstração seja diferente.
func (c *Console) demoSettings() experiment.Settings {
	cfg := c.opts.Config
	settings := experiment.NewSettings(cfg)
	settings.Requests = workload.Generate(cfg.Client.Requests, cfg.Client.WorkloadSeed)
	settings.LossSeed = cfg.UDP.LossSeed
	return settings
}

// demoLoggers mostra servidor e cliente intercalados, como nas tarefas demo.
func (c *Console) demoLoggers() experiment.Loggers {
	level := logger.ParseLevel(c.opts.Config.Log.Level)
	return experiment.Loggers{
		Client: func(protocol string) *slog.Logger {
			return c.logger(experiment.Component(protocol, "client"), level)
		},
		Server: func(protocol string) *slog.Logger {
			return c.logger(experiment.Component(protocol, "server"), level)
		},
	}
}

func (c *Console) runExperiment(ctx context.Context) error {
	level := logger.ParseLevel(c.opts.Config.Log.Level)
	loggers := experiment.Loggers{
		Client: func(protocol string) *slog.Logger {
			return c.logger(experiment.Component(protocol, "client"), level)
		},
		Server: func(protocol string) *slog.Logger {
			return c.logger(experiment.Component(protocol, "server"), slog.LevelWarn)
		},
	}
	paths, err := experiment.RunAndSave(ctx, c.opts.Config, loggers, c.opts.Out, c.opts.Color)
	if err != nil {
		return err
	}
	c.println("\n " + c.paint(logger.Green, "relatório gravado: ") + strings.Join(paths, ", "))
	return nil
}

func (c *Console) live(ctx context.Context) error {
	restore := c.terminal.KeyMode()
	defer restore()
	return live.Run(ctx, live.Options{
		Config: c.opts.Config,
		Keys:   c.input.Keys(),
		Out:    c.opts.Out,
		Color:  c.opts.Color,
		Size:   c.terminal.Size,
	})
}

func (c *Console) showResults(context.Context) error {
	path := filepath.Join(c.opts.Config.Experiment.ResultsDir, "experiment.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		c.println(c.paint(logger.Yellow, "nenhum experimento em "+path+"; rode a opção 3 primeiro"))
		return nil
	}
	if err != nil {
		return err
	}

	var report experiment.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("relatório %s ilegível: %w", path, err)
	}
	c.println(c.paint(logger.Gray, fmt.Sprintf("gerado em %s · N = %d · seed %d · arquivo %s",
		report.GeneratedAt.Format("02/01/2006 15:04:05"), report.Requests, report.Seed, path)))
	fmt.Fprint(c.opts.Out, report.ConsoleTables(c.opts.Color))
	return nil
}

func (c *Console) showConfig(context.Context) error {
	data, err := json.MarshalIndent(c.opts.Config, " ", "  ")
	if err != nil {
		return err
	}
	c.println(" " + string(data))
	c.println("\n " + c.paint(logger.Gray, "altere em config.json ou com variáveis CALC_* (veja o README)"))
	return nil
}

func (c *Console) printSummary(title string, summary metrics.Summary) {
	table := console.Table{
		Title:   title,
		Headers: console.SummaryHeaders,
		Rows:    [][]string{console.SummaryRow(summary.Protocol, summary)},
	}
	fmt.Fprint(c.opts.Out, table.Render(c.opts.Color))
}
