// Package console desenha as tabelas de resumo exibidas no terminal ao fim de
// cada cliente e do experimento.
package console

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"calcsockets/internal/shared/logger"
	"calcsockets/internal/shared/metrics"
)

// Table é uma tabela de texto com bordas; a largura de cada coluna se ajusta ao conteúdo.
type Table struct {
	Title   string
	Headers []string
	Rows    [][]string
}

// Render devolve a tabela pronta para impressão. Com color verdadeiro, título e
// cabeçalhos são destacados; o alinhamento é calculado sobre o texto sem cor.
func (t Table) Render(color bool) string {
	widths := make([]int, len(t.Headers))
	for i, header := range t.Headers {
		widths[i] = utf8.RuneCountInString(header)
	}
	for _, row := range t.Rows {
		for i, cell := range row {
			widths[i] = max(widths[i], utf8.RuneCountInString(cell))
		}
	}

	border := func(left, middle, right string) string {
		parts := make([]string, len(widths))
		for i, width := range widths {
			parts[i] = strings.Repeat("─", width+2)
		}
		return logger.Paint(color, logger.Gray, left+strings.Join(parts, middle)+right) + "\n"
	}
	line := func(cells []string, style string) string {
		var b strings.Builder
		bar := logger.Paint(color, logger.Gray, "│")
		b.WriteString(bar)
		for i, cell := range cells {
			b.WriteString(" " + logger.Paint(color, style, logger.PadRight(cell, widths[i])) + " " + bar)
		}
		return b.String() + "\n"
	}

	var out strings.Builder
	if t.Title != "" {
		out.WriteString("\n" + logger.Paint(color, logger.Bold, t.Title) + "\n")
	}
	out.WriteString(border("┌", "┬", "┐"))
	out.WriteString(line(t.Headers, logger.Bold+logger.Cyan))
	out.WriteString(border("├", "┼", "┤"))
	for _, row := range t.Rows {
		out.WriteString(line(row, ""))
	}
	out.WriteString(border("└", "┴", "┘"))
	return out.String()
}

// SummaryHeaders são as colunas pedidas na Parte 3.
var SummaryHeaders = []string{"Execução", "Tempo total", "RTT médio", "RTT máx", "Retransm.", "Perdidas", "Req (B)", "Resp (B)"}

// SummaryRow formata um resumo nas colunas de SummaryHeaders.
func SummaryRow(name string, s metrics.Summary) []string {
	return []string{
		name,
		FormatDuration(s.TotalTime),
		FormatDuration(s.AvgRTT),
		FormatDuration(s.MaxRTT),
		fmt.Sprint(s.Retransmissions),
		fmt.Sprintf("%d/%d", s.Lost, s.Requests),
		fmt.Sprintf("%.1f", s.AvgRequestBytes),
		fmt.Sprintf("%.1f", s.AvgResponseBytes),
	}
}

// FormatDuration usa milissegundos com três casas, unidade comum a RTT de
// loopback (dezenas de µs) e a esperas de retransmissão (centenas de ms).
func FormatDuration(d time.Duration) string {
	return fmt.Sprintf("%.3f ms", float64(d)/float64(time.Millisecond))
}
