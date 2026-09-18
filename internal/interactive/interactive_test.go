package interactive

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"calcsockets/internal/shared/config"
)

// runScript executa o console com as opções dadas, como `calc <opções...>`.
func runScript(t *testing.T, script ...string) string {
	t.Helper()
	cfg := config.Default()
	cfg.Client.Requests = 5
	cfg.Experiment.ResultsDir = t.TempDir()

	var out bytes.Buffer
	console := New(Options{Config: cfg, Version: "teste", Script: script, In: strings.NewReader(""), Out: &out})
	if err := console.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestScriptedDemoRunsThePart(t *testing.T) {
	out := runScript(t, "2", "0")
	for _, want := range []string{"Parte 2 · TCP", "tcp-server", "RESULT:", "CalcClientTCP", "até mais!"} {
		if !strings.Contains(out, want) {
			t.Errorf("saída sem %q", want)
		}
	}
}

func TestScriptedUDPDemoAcceptsLossRate(t *testing.T) {
	out := runScript(t, "1", "0", "0")
	if !strings.Contains(out, "perda_simulada=0") || !strings.Contains(out, "CalcClientUDP") {
		t.Fatalf("demonstração UDP sem a perda escolhida:\n%s", out)
	}
}

func TestUnknownOptionKeepsTheMenuAlive(t *testing.T) {
	out := runScript(t, "x", "8")
	if !strings.Contains(out, `opção "x" não existe`) {
		t.Error("opção inválida não foi informada")
	}
	if !strings.Contains(out, `"requests": 5`) {
		t.Error("a opção seguinte (configuração) deveria ter executado")
	}
}

func TestResultsWithoutExperimentExplainWhatToDo(t *testing.T) {
	if out := runScript(t, "7"); !strings.Contains(out, "rode a opção 3") {
		t.Fatalf("mensagem de orientação ausente:\n%s", out)
	}
}

// TestSuitesCoverEveryPackageWithTests impede que um pacote novo com testes fique
// de fora do menu de testes e da versão de produção.
func TestSuitesCoverEveryPackageWithTests(t *testing.T) {
	// O binário de teste da release roda sem o código-fonte ao lado.
	if _, err := os.Stat("console.go"); err != nil {
		t.Skip("código-fonte indisponível; verificação feita no build a partir do fonte")
	}
	var listed []string
	for _, suite := range testSuites {
		listed = append(listed, suite.binaries...)
	}

	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return err
		}
		name := filepath.Base(filepath.Dir(path))
		if !slices.Contains(listed, name) {
			t.Errorf("pacote %s tem testes mas não está em testSuites", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestColorizeTestOutput(t *testing.T) {
	var out bytes.Buffer
	colorizeTestOutput(strings.NewReader("=== RUN   TestX\n--- PASS: TestX (0.00s)\n--- FAIL: TestY (0.00s)\n"), &out, true)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || !strings.Contains(lines[1], "\033[32m") || !strings.Contains(lines[2], "\033[31m") {
		t.Fatalf("cores inesperadas: %q", lines)
	}
}

func TestInputReadsLinesAndFinalLineWithoutNewline(t *testing.T) {
	input := NewInput(strings.NewReader("1\r\n2\nfim"))
	var lines []string
	for {
		line, ok := input.ReadLine(context.Background())
		if !ok {
			break
		}
		lines = append(lines, line)
	}
	if strings.Join(lines, ",") != "1,2,fim" {
		t.Fatalf("linhas = %q", lines)
	}
}
