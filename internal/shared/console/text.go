package console

import (
	"strings"
	"unicode/utf8"

	"calcsockets/internal/shared/logger"
)

// escape inicia uma sequência ANSI de cor, que ocupa bytes mas não colunas.
const escape = '\033'

// VisibleWidth conta as colunas que text ocupa no terminal, ignorando códigos de cor.
func VisibleWidth(text string) int {
	width, inEscape := 0, false
	for _, r := range text {
		switch {
		case r == escape:
			inEscape = true
		case inEscape:
			inEscape = r != 'm'
		default:
			width++
		}
	}
	return width
}

// Truncate corta text em width colunas visíveis, preservando os códigos de cor, e
// fecha a cor aberta para que o corte não "vaze" para a linha seguinte.
func Truncate(text string, width int) string {
	if VisibleWidth(text) <= width {
		return text
	}
	var out strings.Builder
	visible, inEscape := 0, false
	for _, r := range text {
		switch {
		case r == escape:
			inEscape = true
		case inEscape:
			inEscape = r != 'm'
		default:
			if visible == width-1 {
				out.WriteString("…" + logger.Reset)
				return out.String()
			}
			visible++
		}
		out.WriteRune(r)
	}
	return out.String()
}

// PadLeft alinha text à direita em width colunas, para colunas numéricas.
func PadLeft(text string, width int) string {
	if gap := width - utf8.RuneCountInString(text); gap > 0 {
		return strings.Repeat(" ", gap) + text
	}
	return text
}
