package interactive

import (
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Tamanho assumido quando o terminal não informa o seu, como fora de um TTY.
const (
	defaultRows = 40
	defaultCols = 120
	// sizeCacheTTL evita executar stty a cada quadro do painel.
	sizeCacheTTL = time.Second
)

// Terminal controla o modo do terminal com o utilitário stty, presente em Linux,
// macOS e nas imagens do projeto. Assim o projeto não depende de biblioteca
// externa. Sem stty, por exemplo no Windows, o console funciona em modo de linha:
// cada tecla do painel precisa de Enter.
type Terminal struct {
	file        *os.File
	interactive bool

	mu         sync.Mutex
	rows, cols int
	measuredAt time.Time
}

// NewTerminal associa o terminal à entrada in, quando ela é um arquivo.
func NewTerminal(in io.Reader) *Terminal {
	file, _ := in.(*os.File)
	t := &Terminal{file: file}
	t.interactive = t.detectInteractive()
	return t
}

// Interactive informa se há uma pessoa digitando num terminal.
func (t *Terminal) Interactive() bool { return t.interactive }

// detectInteractive exige que o stty consiga ler o terminal, além de a entrada
// ser um dispositivo de caractere: /dev/null, que o `docker run` sem -i usa como
// entrada, também é um dispositivo de caractere, mas não um terminal.
func (t *Terminal) detectInteractive() bool {
	if t.file == nil {
		return false
	}
	info, err := t.file.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	_, err = t.stty("-g")
	return err == nil
}

// KeyMode desliga o eco e o modo de linha, para que cada tecla chegue na hora.
// Devolve a função que restaura o modo anterior; ela deve ser chamada mesmo em erro.
func (t *Terminal) KeyMode() (restore func()) {
	noop := func() {}
	if !t.interactive {
		return noop
	}
	saved, err := t.stty("-g")
	if err != nil {
		return noop
	}
	if _, err := t.stty("-icanon", "-echo", "min", "1", "time", "0"); err != nil {
		return noop
	}
	return func() { t.stty(strings.TrimSpace(saved)) }
}

// Size devolve linhas e colunas do terminal, com cache curto.
func (t *Terminal) Size() (rows, cols int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if time.Since(t.measuredAt) < sizeCacheTTL {
		return t.rows, t.cols
	}

	t.rows, t.cols, t.measuredAt = defaultRows, defaultCols, time.Now()
	if !t.interactive {
		return t.rows, t.cols
	}
	output, err := t.stty("size")
	if err != nil {
		return t.rows, t.cols
	}
	fields := strings.Fields(output)
	if len(fields) == 2 {
		rows, rowsErr := strconv.Atoi(fields[0])
		cols, colsErr := strconv.Atoi(fields[1])
		if rowsErr == nil && colsErr == nil && rows > 0 && cols > 0 {
			t.rows, t.cols = rows, cols
		}
	}
	return t.rows, t.cols
}

// stty executa o utilitário sobre este terminal, que precisa ser sua entrada padrão.
func (t *Terminal) stty(args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = t.file
	output, err := cmd.Output()
	return string(output), err
}
