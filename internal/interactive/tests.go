package interactive

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"calcsockets/internal/shared/logger"
)

// testSuite agrupa os testes de uma parte. binaries são os nomes dos binários de
// teste gerados por scripts/build-release.sh (<pacote>.test), e packages os padrões
// equivalentes para `go test` quando o console roda a partir do código-fonte.
type testSuite struct {
	name     string
	color    string
	binaries []string
	packages []string
}

var testSuites = []testSuite{
	{"Compartilhado", logger.Gray, []string{"calc", "textprotocol", "metrics", "workload", "console"}, []string{"./internal/shared/..."}},
	{"Parte 1 · UDP", logger.Blue, []string{"udp"}, []string{"./internal/udp/..."}},
	{"Parte 2 · TCP", logger.Cyan, []string{"tcp"}, []string{"./internal/tcp/..."}},
	{"Parte 3 · Experimento", logger.Yellow, []string{"experiment"}, []string{"./internal/experiment/..."}},
	{"Parte 4 · Protobuf", logger.Magenta, []string{"protocalc"}, []string{"./internal/protocalc/..."}},
	{"Console e tempo real", logger.Green, []string{"interactive", "live"}, []string{"./internal/interactive/...", "./internal/live/..."}},
}

// allSuitesKey executa todas as suítes em sequência.
const allSuitesKey = "t"

func (c *Console) chooseTests(ctx context.Context) error {
	for i, suite := range testSuites {
		c.println(fmt.Sprintf("   %s  %s", c.paint(logger.Bold+suite.color, strconv.Itoa(i+1)), c.paint(suite.color, suite.name)))
	}
	c.println(fmt.Sprintf("   %s  Todas\n   %s  Voltar", c.paint(logger.Bold, allSuitesKey), c.paint(logger.Bold, quitKey)))

	choice, ok := c.prompt(ctx, "Quais testes")
	switch {
	case !ok || choice == quitKey:
		return nil
	case strings.EqualFold(choice, allSuitesKey):
		return c.runSuites(ctx, testSuites)
	}
	index, err := strconv.Atoi(choice)
	if err != nil || index < 1 || index > len(testSuites) {
		return fmt.Errorf("opção %q não existe", choice)
	}
	return c.runSuites(ctx, testSuites[index-1:index])
}

// runSuites executa as suítes e resume quais passaram. Uma suíte que falha não
// impede as seguintes.
func (c *Console) runSuites(ctx context.Context, suites []testSuite) error {
	runner, err := newTestRunner()
	if err != nil {
		return err
	}

	c.println(c.paint(logger.Gray, "executando com "+runner.describe()))
	var failed []string
	for _, suite := range suites {
		c.println("\n" + c.paint(logger.Bold+suite.color, "━━ "+suite.name+" ━━"))
		if err := runner.run(ctx, suite, c.opts.Out, c.opts.Color); err != nil {
			failed = append(failed, suite.name)
		}
	}

	c.println("")
	if len(failed) > 0 {
		return fmt.Errorf("suítes com falha: %s", strings.Join(failed, ", "))
	}
	c.println(c.paint(logger.Bold+logger.Green, fmt.Sprintf("✔ %d suíte(s) passaram", len(suites))))
	return nil
}

// testRunner executa testes de uma de duas formas: binários pré-compilados, que
// acompanham a versão de produção e dispensam o Go, ou `go test` a partir do fonte.
type testRunner struct {
	binaryDir string
}

func newTestRunner() (testRunner, error) {
	if dir, ok := compiledTestsDir(); ok {
		return testRunner{binaryDir: dir}, nil
	}
	if _, err := exec.LookPath("go"); err != nil {
		return testRunner{}, errors.New("sem binários de teste ao lado do executável e sem Go instalado")
	}
	if _, err := os.Stat("go.mod"); err != nil {
		return testRunner{}, errors.New("rode o console a partir da raiz do projeto para usar `go test`")
	}
	return testRunner{}, nil
}

// compiledTestsDir procura a pasta tests/ ao lado do executável, como no pacote da release.
func compiledTestsDir() (string, bool) {
	executable, err := os.Executable()
	if err != nil {
		return "", false
	}
	dir := filepath.Join(filepath.Dir(executable), "tests")
	info, err := os.Stat(dir)
	return dir, err == nil && info.IsDir()
}

func (r testRunner) describe() string {
	if r.binaryDir != "" {
		return "binários pré-compilados em " + r.binaryDir
	}
	return "go test a partir do código-fonte"
}

func (r testRunner) run(ctx context.Context, suite testSuite, out io.Writer, color bool) error {
	if r.binaryDir == "" {
		args := append([]string{"test", "-count=1", "-v"}, suite.packages...)
		return runColorized(exec.CommandContext(ctx, "go", args...), out, color)
	}

	var failures error
	for _, name := range suite.binaries {
		path := filepath.Join(r.binaryDir, name+".test"+executableSuffix())
		if _, err := os.Stat(path); err != nil {
			failures = errors.Join(failures, fmt.Errorf("binário de teste ausente: %s", path))
			fmt.Fprintln(out, logger.Paint(color, logger.Red, "binário de teste ausente: "+path))
			continue
		}
		fmt.Fprintln(out, logger.Paint(color, logger.Gray, "▸ "+name))
		failures = errors.Join(failures, runColorized(exec.CommandContext(ctx, path, "-test.v", "-test.count=1"), out, color))
	}
	return failures
}

// runColorized executa o comando destacando o resultado de cada teste.
func runColorized(cmd *exec.Cmd, out io.Writer, color bool) error {
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	colorizeTestOutput(pipe, out, color)
	return cmd.Wait()
}

// colorizeTestOutput pinta a saída de `go test -v` linha a linha: aprovados em
// verde, falhas em vermelho e o ruído de "=== RUN" esmaecido.
func colorizeTestOutput(r io.Reader, w io.Writer, color bool) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		style := ""
		switch {
		case strings.HasPrefix(trimmed, "--- PASS"), strings.HasPrefix(trimmed, "ok "), trimmed == "PASS":
			style = logger.Green
		case strings.HasPrefix(trimmed, "--- FAIL"), strings.HasPrefix(trimmed, "FAIL"), strings.HasPrefix(trimmed, "panic:"):
			style = logger.Bold + logger.Red
		case strings.HasPrefix(trimmed, "=== "):
			style = logger.Gray
		}
		fmt.Fprintln(w, logger.Paint(color, style, line))
	}
}

func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
