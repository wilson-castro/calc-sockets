package live

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"calcsockets/internal/shared/config"
	"calcsockets/internal/shared/console"
	"calcsockets/internal/shared/metrics"
)

func TestTrackerAggregatesAndKeepsRecentHistory(t *testing.T) {
	var tracker Tracker
	tracker.Record(metrics.Sample{RTT: 2 * time.Millisecond, Attempts: 1, RequestBytes: 10, ResponseBytes: 20})
	tracker.Record(metrics.Sample{RTT: 502 * time.Millisecond, Attempts: 2, RequestBytes: 10, ResponseBytes: 20})
	tracker.Record(metrics.Sample{Attempts: 5, Lost: true, RequestBytes: 10})

	s := tracker.Snapshot()
	if s.Sent != 3 || s.Answered != 2 || s.Lost != 1 || s.Retransmissions != 5 {
		t.Fatalf("contagens erradas: %+v", s)
	}
	if s.AvgRTT != 252*time.Millisecond || s.MaxRTT != 502*time.Millisecond || s.AvgResponseB != 20 {
		t.Fatalf("médias erradas: %+v", s)
	}

	for range historySize + 10 {
		tracker.Record(metrics.Sample{RTT: time.Millisecond, Attempts: 1})
	}
	if got := len(tracker.Snapshot().History); got != historySize {
		t.Fatalf("histórico com %d pontos, esperado %d", got, historySize)
	}

	tracker.Reset()
	if tracker.Snapshot().Sent != 0 {
		t.Fatal("Reset deveria zerar as métricas")
	}
}

func TestEventLogKeepsOnlyTheNewestLines(t *testing.T) {
	events := NewEventLog(3)
	for _, line := range []string{"a\n", "b\n", "c\nd\n"} {
		events.Write([]byte(line))
	}
	if got := strings.Join(events.Last(10), ","); got != "b,c,d" {
		t.Fatalf("eventos = %s, esperado b,c,d", got)
	}
}

func TestSparkLevelIsMonotonic(t *testing.T) {
	previous := -1
	for _, rtt := range []time.Duration{10 * time.Microsecond, 200 * time.Microsecond, 5 * time.Millisecond, 500 * time.Millisecond, 10 * time.Second} {
		level := sparkLevel(rtt)
		if level < previous || level >= len(sparkLevels) {
			t.Fatalf("nível %d para %v fora de ordem (anterior %d)", level, rtt, previous)
		}
		previous = level
	}
}

func TestRenderFitsTheTerminal(t *testing.T) {
	var tracker Tracker
	tracker.Record(metrics.Sample{Request: "CALC:0:1:+:1", Reply: "RESULT:0:2.0", RTT: time.Millisecond, Attempts: 1})
	frame := Frame{
		LossRate:  0.3,
		Protocols: []ProtocolView{{Name: "UDP", Snapshot: tracker.Snapshot()}},
		Events:    []string{strings.Repeat("evento muito longo ", 20)},
	}

	lines := Render(frame, 80, 30, true)
	if len(lines) > 30 {
		t.Fatalf("%d linhas não cabem em 30", len(lines))
	}
	for _, line := range lines {
		if console.VisibleWidth(line) > 80 {
			t.Fatalf("linha com %d colunas: %q", console.VisibleWidth(line), line)
		}
	}
	if joined := strings.Join(lines, "\n"); !strings.Contains(joined, "30%") || !strings.Contains(joined, "RESULT:0:2.0") {
		t.Fatal("quadro sem a perda ou a última troca")
	}
}

// TestRunDrivesAllProtocolsAndObeysKeys abre o painel de verdade, com sockets
// reais, aumenta a perda pelo teclado e fecha com q.
func TestRunDrivesAllProtocolsAndObeysKeys(t *testing.T) {
	cfg := config.Default()
	cfg.Live.IntervalMS = 5
	cfg.Live.LossRate = 0
	cfg.Client.UDPTimeoutMS = 20

	keys := make(chan byte, 4)
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), Options{
			Config: cfg, Keys: keys, Out: &out, Color: false,
			Size: func() (int, int) { return 40, 140 },
		})
	}()

	time.Sleep(300 * time.Millisecond)
	keys <- '+'
	time.Sleep(300 * time.Millisecond)
	keys <- 'q'

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("o painel não encerrou após q")
	}

	screen := out.String()
	for _, want := range []string{"TEMPO REAL", "perda UDP 10%", "RESULT:", "Protobuf"} {
		if !strings.Contains(screen, want) {
			t.Errorf("painel sem %q", want)
		}
	}
}
