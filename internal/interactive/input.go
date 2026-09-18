package interactive

import (
	"context"
	"io"
	"strings"
)

// Input é a única leitora da entrada padrão. Uma goroutine lê byte a byte e
// entrega a um canal, que o menu consome em linhas e o painel em teclas. Com
// duas leitoras, a que ficasse bloqueada no Read "roubaria" a próxima tecla da outra.
type Input struct {
	bytes chan byte
}

// NewInput começa a ler r em segundo plano. O fim de r fecha o canal.
func NewInput(r io.Reader) *Input {
	in := &Input{bytes: make(chan byte, 256)}
	go in.pump(r)
	return in
}

func (in *Input) pump(r io.Reader) {
	defer close(in.bytes)
	buffer := make([]byte, 256)
	for {
		n, err := r.Read(buffer)
		for _, b := range buffer[:n] {
			in.bytes <- b
		}
		if err != nil {
			return
		}
	}
}

// Keys expõe os bytes crus, para leitura tecla a tecla.
func (in *Input) Keys() <-chan byte { return in.bytes }

// ReadLine devolve a próxima linha sem o terminador. ok é falso quando a entrada
// terminou ou ctx foi cancelado sem nenhuma linha pendente.
func (in *Input) ReadLine(ctx context.Context) (line string, ok bool) {
	var b strings.Builder
	for {
		select {
		case <-ctx.Done():
			return "", false
		case c, open := <-in.bytes:
			if !open {
				return b.String(), b.Len() > 0
			}
			if c == '\n' {
				return strings.TrimRight(b.String(), "\r"), true
			}
			b.WriteByte(c)
		}
	}
}
