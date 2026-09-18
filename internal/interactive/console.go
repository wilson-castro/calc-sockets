// Package interactive implementa o console da versão de produção: um menu que
// executa cada parte da atividade, os testes, o relatório e o painel em tempo real,
// sem que seja preciso decorar comandos.
//
// As opções também podem ser passadas como argumentos, que são lidos antes da
// entrada padrão: `calc 2 0` executa a Parte 2 e sai. Isso permite automatizar
// verificações da versão empacotada.
package interactive

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"calcsockets/internal/shared/config"
	"calcsockets/internal/shared/logger"
)

const (
	clearScreen = "\033[H\033[2J"
	// quitKey encerra o console; também é "voltar" nos submenus.
	quitKey = "0"
)

// Options configura o console.
type Options struct {
	Config  config.Config
	Version string
	Color   bool
	// Script são opções lidas antes de In, como se tivessem sido digitadas.
	Script []string
	In     io.Reader
	// Out recebe menus, logs e tabelas. Precisa ser comparável (ver logger.New).
	Out io.Writer
}

// Console é o menu interativo.
type Console struct {
	opts     Options
	input    *Input
	terminal *Terminal
	items    []item
}

// item é uma opção do menu principal.
type item struct {
	key     string
	section string
	title   string
	detail  string
	color   string
	run     func(ctx context.Context) error
}

// New prepara o console. Nada é lido até Run.
func New(opts Options) *Console {
	var source io.Reader = opts.In
	if len(opts.Script) > 0 {
		source = io.MultiReader(strings.NewReader(strings.Join(opts.Script, "\n")+"\n"), opts.In)
	}
	c := &Console{opts: opts, input: NewInput(source), terminal: NewTerminal(opts.In)}
	c.items = c.menu()
	return c
}

// Run mostra o menu até a opção de saída, o fim da entrada ou o cancelamento de ctx.
// Erros de uma opção são exibidos e o menu continua.
func (c *Console) Run(ctx context.Context) error {
	for {
		c.drawMenu()
		choice, ok := c.prompt(ctx, "Escolha")
		if !ok || choice == quitKey {
			c.println(c.paint(logger.Gray, "até mais!"))
			return nil
		}

		selected, found := c.find(choice)
		if !found {
			c.println(c.paint(logger.Red, fmt.Sprintf("opção %q não existe", choice)))
			c.pause(ctx)
			continue
		}

		c.clear()
		c.println(c.paint(logger.Bold+selected.color, "▌ "+selected.title) + c.paint(logger.Gray, "  "+selected.detail) + "\n")
		if err := selected.run(ctx); err != nil {
			c.println(c.paint(logger.Red, "falhou: "+err.Error()))
		}
		if ctx.Err() != nil {
			return nil
		}
		c.pause(ctx)
	}
}

func (c *Console) find(key string) (item, bool) {
	for _, candidate := range c.items {
		if strings.EqualFold(candidate.key, key) {
			return candidate, true
		}
	}
	return item{}, false
}

func (c *Console) drawMenu() {
	c.clear()
	title := " Calculadora remota · UDP × TCP × Protobuf "
	version := " " + c.opts.Version + " "
	width := len([]rune(title)) + len([]rune(version)) + 4
	border := c.paint(logger.Magenta, strings.Repeat("─", width))

	c.println(c.paint(logger.Magenta, "╭") + border + c.paint(logger.Magenta, "╮"))
	c.println(c.paint(logger.Magenta, "│") + "  " + c.paint(logger.Bold, title) + c.paint(logger.Gray, version) + "  " + c.paint(logger.Magenta, "│"))
	c.println(c.paint(logger.Magenta, "╰") + border + c.paint(logger.Magenta, "╯"))

	section := ""
	for _, it := range c.items {
		if it.section != section {
			section = it.section
			c.println("\n  " + c.paint(logger.Gray, strings.ToUpper(section)))
		}
		c.println(fmt.Sprintf("   %s  %s %s", c.paint(logger.Bold+it.color, it.key),
			c.paint(it.color, logger.PadRight(it.title, 24)), c.paint(logger.Gray, it.detail)))
	}
	c.println("\n   " + c.paint(logger.Bold, quitKey) + "  Sair")
}

// prompt mostra a pergunta e lê uma linha já sem espaços nas pontas.
func (c *Console) prompt(ctx context.Context, question string) (string, bool) {
	fmt.Fprint(c.opts.Out, "\n "+c.paint(logger.Bold+logger.Cyan, question+" ›")+" ")
	line, ok := c.input.ReadLine(ctx)
	if !c.terminal.Interactive() {
		// Fora de um terminal a entrada não ecoa; repetir a escolha deixa a saída legível.
		fmt.Fprintln(c.opts.Out, line)
	}
	return strings.TrimSpace(line), ok
}

// pause espera Enter antes de voltar ao menu, para que o resultado possa ser lido.
// Fora de um terminal não há quem pressione Enter, então segue direto.
func (c *Console) pause(ctx context.Context) {
	if !c.terminal.Interactive() {
		return
	}
	fmt.Fprint(c.opts.Out, "\n "+c.paint(logger.Gray, "Enter para voltar ao menu"))
	c.input.ReadLine(ctx)
}

func (c *Console) clear() {
	if c.terminal.Interactive() {
		fmt.Fprint(c.opts.Out, clearScreen)
	}
}

func (c *Console) println(text string) { fmt.Fprintln(c.opts.Out, text) }

func (c *Console) paint(style, text string) string { return logger.Paint(c.opts.Color, style, text) }

// logger cria loggers que escrevem na mesma saída do console.
func (c *Console) logger(component string, level slog.Level) *slog.Logger {
	return logger.New(component, c.opts.Out, logger.Options{Level: level, Color: c.opts.Color})
}
