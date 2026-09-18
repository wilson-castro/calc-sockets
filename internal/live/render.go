package live

import (
	"fmt"
	"math"
	"strings"
	"time"

	"calcsockets/internal/shared/console"
	"calcsockets/internal/shared/logger"
)

// Limites da escala logarítmica do gráfico: RTT de loopback fica perto do
// mínimo e cinco timeouts de 500 ms perto do máximo.
const (
	sparkMin = 50 * time.Microsecond
	sparkMax = 3 * time.Second
)

var sparkLevels = []rune("▁▂▃▄▅▆▇█")

// ProtocolView é uma linha do painel.
type ProtocolView struct {
	Name     string
	Color    string
	Snapshot Snapshot
}

// Frame é tudo o que um quadro do painel mostra.
type Frame struct {
	Uptime    time.Duration
	LossRate  float64
	Interval  time.Duration
	Paused    bool
	Stopping  bool
	Protocols []ProtocolView
	Events    []string
}

// column descreve uma coluna da tabela de métricas.
type column struct {
	title string
	width int
	value func(Snapshot) string
}

var columns = []column{
	{"Enviadas", 9, func(s Snapshot) string { return fmt.Sprint(s.Sent) }},
	{"Respondidas", 12, func(s Snapshot) string { return fmt.Sprint(s.Answered) }},
	{"Retransm.", 10, func(s Snapshot) string { return fmt.Sprint(s.Retransmissions) }},
	{"Perdidas", 9, func(s Snapshot) string { return fmt.Sprint(s.Lost) }},
	{"RTT último", 13, func(s Snapshot) string { return console.FormatDuration(s.LastRTT) }},
	{"RTT médio", 13, func(s Snapshot) string { return console.FormatDuration(s.AvgRTT) }},
	{"RTT máx", 13, func(s Snapshot) string { return console.FormatDuration(s.MaxRTT) }},
	{"Bytes req/resp", 16, func(s Snapshot) string { return fmt.Sprintf("%.1f / %.1f", s.AvgRequestB, s.AvgResponseB) }},
}

const nameWidth = 10

// Render desenha o quadro em linhas de no máximo width colunas. Os eventos
// ocupam as linhas que sobrarem até height.
func Render(f Frame, width, height int, color bool) []string {
	paint := func(style, text string) string { return logger.Paint(color, style, text) }
	var lines []string
	add := func(line string) { lines = append(lines, line) }

	add(header(f, paint))
	add("")

	titles := logger.PadRight("Protocolo", nameWidth)
	for _, c := range columns {
		titles += console.PadLeft(c.title, c.width)
	}
	add(" " + paint(logger.Bold+logger.Cyan, titles))
	add(" " + paint(logger.Gray, strings.Repeat("─", console.VisibleWidth(titles))))
	for _, p := range f.Protocols {
		row := paint(logger.Bold+p.Color, logger.PadRight(p.Name, nameWidth))
		for _, c := range columns {
			row += console.PadLeft(c.value(p.Snapshot), c.width)
		}
		add(" " + row)
	}

	add("")
	add(" " + paint(logger.Bold, "RTT por requisição") + paint(logger.Gray,
		fmt.Sprintf("  escala log de %v a %v · ", sparkMin, sparkMax)) +
		paint(logger.Green, "▆") + paint(logger.Gray, " respondida  ") +
		paint(logger.Yellow, "▆") + paint(logger.Gray, " retransmitida  ") +
		paint(logger.Red, "✗") + paint(logger.Gray, " perdida"))
	for _, p := range f.Protocols {
		add(" " + paint(p.Color, logger.PadRight(p.Name, nameWidth)) + sparkline(p.Snapshot.History, paint))
	}

	add("")
	add(" " + paint(logger.Bold, "Última troca"))
	for _, p := range f.Protocols {
		add(" " + paint(p.Color, logger.PadRight(p.Name, nameWidth)) + lastExchange(p.Snapshot, paint))
	}

	add("")
	add(" " + paint(logger.Bold, "Eventos"))
	footer := " " + keyHint("+", "mais perda", paint) + keyHint("-", "menos perda", paint) +
		keyHint("p", "pausar", paint) + keyHint("r", "zerar", paint) + keyHint("q", "voltar ao menu", paint)

	room := max(height-len(lines)-2, 3)
	events := f.Events
	if len(events) > room {
		events = events[len(events)-room:]
	}
	for _, event := range events {
		add(" " + event)
	}
	for range room - len(events) {
		add("")
	}
	add("")
	add(footer)

	for i, line := range lines {
		lines[i] = console.Truncate(line, width)
	}
	return lines
}

func header(f Frame, paint func(string, string) string) string {
	status := paint(logger.Green, "● rodando")
	switch {
	case f.Stopping:
		status = paint(logger.Yellow, "◌ encerrando")
	case f.Paused:
		status = paint(logger.Yellow, "❚❚ pausado")
	}
	uptime := f.Uptime.Truncate(time.Second)
	return " " + paint(logger.Bold+logger.Magenta, "▌ CALCULADORA REMOTA · TEMPO REAL") +
		paint(logger.Gray, "   tempo ") + uptime.String() +
		paint(logger.Gray, "   perda UDP ") + paint(logger.Bold+logger.Yellow, fmt.Sprintf("%.0f%%", f.LossRate*100)) +
		paint(logger.Gray, "   intervalo ") + f.Interval.String() + "   " + status
}

func sparkline(points []Point, paint func(string, string) string) string {
	var b strings.Builder
	for _, p := range points {
		if p.Lost {
			b.WriteString(paint(logger.Red, "✗"))
			continue
		}
		style := logger.Green
		if p.Attempts > 1 {
			style = logger.Yellow
		}
		b.WriteString(paint(style, string(sparkLevels[sparkLevel(p.RTT)])))
	}
	return b.String()
}

// sparkLevel mapeia o RTT para um dos oito blocos em escala logarítmica. A escala
// linear esconderia a diferença entre 100 µs e 1 ms, apagada pelos timeouts de 500 ms.
func sparkLevel(rtt time.Duration) int {
	if rtt <= sparkMin {
		return 0
	}
	ratio := math.Log(float64(rtt)/float64(sparkMin)) / math.Log(float64(sparkMax)/float64(sparkMin))
	return min(int(ratio*float64(len(sparkLevels)-1)+0.5), len(sparkLevels)-1)
}

func lastExchange(s Snapshot, paint func(string, string) string) string {
	last := s.Last
	switch {
	case s.Sent == 0:
		return paint(logger.Gray, "aguardando a primeira requisição")
	case last.Lost:
		return last.Request + paint(logger.Red, "  ✗ perdida: "+last.Failure)
	default:
		detail := fmt.Sprintf("  (%s", console.FormatDuration(last.RTT))
		if last.Attempts > 1 {
			detail += fmt.Sprintf(", %d tentativas", last.Attempts)
		}
		return last.Request + paint(logger.Gray, "  →  ") + last.Reply + paint(logger.Gray, detail+")")
	}
}

func keyHint(key, label string, paint func(string, string) string) string {
	return paint(logger.Bold+logger.Cyan, "["+key+"]") + " " + label + "   "
}
