// Package logger fornece um slog.Handler de uma linha por evento, alinhado em
// colunas e colorido por nível e por componente:
//
//	22:15:03.120  INFO   udp-server   descartado (perda simulada)   seq=3 cliente=127.0.0.1:51234
//
// Cada componente (udp-server, tcp-client, ...) tem cor própria, o que permite
// acompanhar cliente e servidor intercalados no mesmo terminal.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Códigos ANSI usados pelo handler e pelo pacote console.
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[90m"
)

const (
	componentWidth = 16
	messageWidth   = 30
	timeLayout     = "15:04:05.000"
)

// componentColors associa o prefixo do componente, que identifica a parte da
// atividade, a uma cor. Servidores aparecem em negrito para diferenciá-los dos clientes.
var componentColors = map[string]string{
	"udp":        Blue,
	"tcp":        Cyan,
	"proto":      Magenta,
	"experiment": Yellow,
	"console":    Green,
}

var levelStyles = map[slog.Level]struct{ label, color string }{
	slog.LevelDebug: {"DEBUG", Gray},
	slog.LevelInfo:  {"INFO", Green},
	slog.LevelWarn:  {"WARN", Yellow},
	slog.LevelError: {"ERROR", Red},
}

// Options controla nível mínimo e uso de cor.
type Options struct {
	Level slog.Level
	Color bool
}

// New cria um logger para o componente informado, escrevendo em w.
// Loggers criados sobre o mesmo w compartilham um mutex e podem ser usados de
// goroutines diferentes; por isso w precisa ser comparável, como *os.File.
func New(component string, w io.Writer, opts Options) *slog.Logger {
	return slog.New(&handler{
		out:       lockedWriter(w),
		opts:      opts,
		component: component,
	})
}

// ParseLevel converte "debug", "info", "warn" ou "error"; outros valores viram info.
func ParseLevel(text string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(text)); err != nil {
		return slog.LevelInfo
	}
	return level
}

// ColorEnabled resolve o modo "auto", "always" ou "never" para o arquivo de saída.
// A variável NO_COLOR, quando definida, desliga cor mesmo no modo "auto".
func ColorEnabled(mode string, out *os.File) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	}
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return false
	}
	info, err := out.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// Paint envolve text com o código de cor quando enabled é verdadeiro.
func Paint(enabled bool, color, text string) string {
	if !enabled || color == "" {
		return text
	}
	return color + text + Reset
}

// PadRight completa text com espaços até width runas; texto maior é mantido.
func PadRight(text string, width int) string {
	if gap := width - utf8.RuneCountInString(text); gap > 0 {
		return text + strings.Repeat(" ", gap)
	}
	return text
}

type handler struct {
	out         *syncWriter
	opts        Options
	component   string
	attrs       []slog.Attr
	groupPrefix string
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level
}

func (h *handler) Handle(_ context.Context, record slog.Record) error {
	var line strings.Builder
	paint := func(color, text string) string { return Paint(h.opts.Color, color, text) }

	style := levelStyles[record.Level]
	if style.label == "" {
		style.label, style.color = record.Level.String(), Red
	}

	line.WriteString(paint(Gray, record.Time.Format(timeLayout)))
	line.WriteString("  ")
	line.WriteString(paint(Bold+style.color, PadRight(style.label, 5)))
	line.WriteString("  ")
	line.WriteString(paint(h.componentColor(), PadRight(h.component, componentWidth)))
	line.WriteString(" ")
	line.WriteString(PadRight(record.Message, messageWidth))

	for _, attr := range h.attrs {
		h.writeAttr(&line, "", attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		h.writeAttr(&line, h.groupPrefix, attr)
		return true
	})
	// Mensagens sem atributos não devem terminar com o preenchimento da coluna.
	return h.out.write(strings.TrimRight(line.String(), " ") + "\n")
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(cloneAttrs(h.attrs), prefixed(h.groupPrefix, attrs)...)
	return &clone
}

func (h *handler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.groupPrefix = h.groupPrefix + name + "."
	return &clone
}

func (h *handler) componentColor() string {
	part, role, _ := strings.Cut(h.component, "-")
	color := componentColors[part]
	if role == "server" {
		color = Bold + color
	}
	return color
}

func (h *handler) writeAttr(line *strings.Builder, prefix string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return
	}
	if attr.Value.Kind() == slog.KindGroup {
		for _, member := range attr.Value.Group() {
			h.writeAttr(line, prefix+attr.Key+".", member)
		}
		return
	}

	value := formatValue(attr.Value)
	if strings.ContainsAny(value, " \t") {
		value = `"` + value + `"`
	}
	valueColor := ""
	if _, isError := attr.Value.Any().(error); isError {
		valueColor = Red
	}

	line.WriteByte(' ')
	line.WriteString(Paint(h.opts.Color, Dim, prefix+attr.Key+"="))
	line.WriteString(Paint(h.opts.Color, valueColor, value))
}

// formatValue arredonda durações para microssegundos, precisão suficiente para
// RTT em loopback e mais legível que os nanossegundos padrão.
func formatValue(value slog.Value) string {
	switch value.Kind() {
	case slog.KindDuration:
		return value.Duration().Round(time.Microsecond).String()
	case slog.KindTime:
		return value.Time().Format(timeLayout)
	default:
		return value.String()
	}
}

func prefixed(prefix string, attrs []slog.Attr) []slog.Attr {
	if prefix == "" {
		return attrs
	}
	out := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		out[i] = slog.Attr{Key: prefix + attr.Key, Value: attr.Value}
	}
	return out
}

// cloneAttrs copia attrs para que loggers derivados não compartilhem o array subjacente.
func cloneAttrs(attrs []slog.Attr) []slog.Attr {
	return append([]slog.Attr(nil), attrs...)
}

// syncWriter serializa escritas de loggers diferentes sobre o mesmo destino,
// evitando linhas intercaladas quando servidor e cliente rodam no mesmo processo.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

var (
	writersMu sync.Mutex
	writers   = map[io.Writer]*syncWriter{}
)

func lockedWriter(w io.Writer) *syncWriter {
	writersMu.Lock()
	defer writersMu.Unlock()
	if existing, ok := writers[w]; ok {
		return existing
	}
	created := &syncWriter{w: w}
	writers[w] = created
	return created
}

func (s *syncWriter) write(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := io.WriteString(s.w, text)
	return err
}
