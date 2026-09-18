package experiment

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"calcsockets/internal/shared/workload"
)

func quietLoggers() Loggers {
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	quiet := func(string) *slog.Logger { return discard }
	return Loggers{Client: quiet, Server: quiet}
}

func settings(lossRates ...float64) Settings {
	return Settings{
		Requests:       workload.Generate(20, 11),
		LossRates:      lossRates,
		LossSeed:       11,
		UDPTimeout:     30 * time.Millisecond,
		UDPMaxAttempts: 5,
		DialTimeout:    time.Second,
		IOTimeout:      2 * time.Second,
	}
}

func TestRunProducesOneResultPerScenario(t *testing.T) {
	results, err := Run(context.Background(), settings(0, 0.3), quietLoggers())
	if err != nil {
		t.Fatal(err)
	}

	scenarios := make([]string, len(results))
	for i, result := range results {
		scenarios[i] = result.Scenario
	}
	want := "UDP perda 0%,UDP perda 30%,TCP,TCP + Protobuf"
	if got := strings.Join(scenarios, ","); got != want {
		t.Fatalf("cenários = %s, esperado %s", got, want)
	}

	if s := results[0].Summary; s.Retransmissions != 0 || s.Lost != 0 {
		t.Errorf("UDP sem perda não deveria retransmitir: %+v", s)
	}
	if s := results[1].Summary; s.Retransmissions == 0 {
		t.Errorf("UDP com 30%% de perda deveria retransmitir: %+v", s)
	}
	for _, result := range results[2:] {
		if s := result.Summary; s.Answered != 20 || s.Retransmissions != 0 {
			t.Errorf("%s deveria responder tudo sem retransmitir: %+v", result.Scenario, s)
		}
	}
}

func TestReportIsSaved(t *testing.T) {
	results, err := Run(context.Background(), settings(0), quietLoggers())
	if err != nil {
		t.Fatal(err)
	}
	report := Report{GeneratedAt: time.Now(), Requests: 20, Seed: 11, UDPTimeoutMS: 30, UDPMaxAttempts: 5, Results: results}

	dir := t.TempDir()
	paths, err := report.Save(dir)
	if err != nil || len(paths) != 2 {
		t.Fatalf("Save: %v %v", paths, err)
	}
	markdown, err := os.ReadFile(filepath.Join(dir, "experiment.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range []string{"## Parte 3", "## Parte 4", "Protobuf (Parte 4)"} {
		if !strings.Contains(string(markdown), section) {
			t.Errorf("relatório sem a seção %q", section)
		}
	}
}
