package udp

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"calcsockets/internal/shared/calc"
	"calcsockets/internal/shared/sequence"
	"calcsockets/internal/shared/workload"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// startServer sobe um CalcServerUDP em porta livre e o encerra ao fim do teste.
func startServer(t *testing.T, opts ServerOptions) string {
	t.Helper()
	server, err := Listen("127.0.0.1:0", opts, discard)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Serve devolveu erro: %v", err)
		}
	})
	return server.Addr()
}

func dial(t *testing.T, address string, opts ClientOptions) *Client {
	t.Helper()
	client, err := Dial(address, opts, discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestServerAnswersStatementExamples(t *testing.T) {
	client := dial(t, startServer(t, ServerOptions{}), ClientOptions{Timeout: time.Second, MaxAttempts: 1})

	cases := []struct {
		request calc.Request
		reply   string
	}{
		{calc.Request{Seq: 0, Left: 10, Op: calc.Add, Right: 5}, "RESULT:0:15.0"},
		{calc.Request{Seq: 1, Left: 8, Op: calc.Divide, Right: 0}, "ERROR:1:divisão por zero"},
		{calc.Request{Seq: 2, Left: 3.5, Op: calc.Multiply, Right: 2}, "RESULT:2:7.0"},
	}
	for _, tc := range cases {
		sample := client.Exchange(tc.request)
		if sample.Lost || sample.Reply != tc.reply {
			t.Errorf("seq %d: resposta %q (perdida=%v), esperado %q", tc.request.Seq, sample.Reply, sample.Lost, tc.reply)
		}
	}
}

func TestWithoutLossThereAreNoRetransmissions(t *testing.T) {
	client := dial(t, startServer(t, ServerOptions{}), ClientOptions{Timeout: time.Second, MaxAttempts: 5})

	summary := sequence.Run(context.Background(), "UDP", client, workload.Generate(20, 1), discard)

	if summary.Answered != 20 || summary.Retransmissions != 0 || summary.Lost != 0 {
		t.Fatalf("sem perda: respondidas=%d retransmissões=%d perdidas=%d",
			summary.Answered, summary.Retransmissions, summary.Lost)
	}
}

func TestRetransmissionRecoversFromSimulatedLoss(t *testing.T) {
	address := startServer(t, ServerOptions{LossRate: 0.5, LossSeed: 7})
	client := dial(t, address, ClientOptions{Timeout: 50 * time.Millisecond, MaxAttempts: 10})

	summary := sequence.Run(context.Background(), "UDP", client, workload.Generate(20, 1), discard)

	if summary.Retransmissions == 0 {
		t.Fatal("com 50% de perda deveria haver retransmissões")
	}
	if summary.Answered+summary.Lost != 20 {
		t.Fatalf("contagem inconsistente: %+v", summary)
	}
}

func TestExhaustedAttemptsMarkRequestAsLost(t *testing.T) {
	address := startServer(t, ServerOptions{LossRate: 1})
	client := dial(t, address, ClientOptions{Timeout: 20 * time.Millisecond, MaxAttempts: 3})

	sample := client.Exchange(calc.Request{Seq: 0, Left: 1, Op: calc.Add, Right: 1})

	if !sample.Lost || sample.Attempts != 3 || sample.Retransmissions() != 2 {
		t.Fatalf("esperada perda após 3 tentativas: %+v", sample)
	}
}

func TestLossRateChangesWhileRunning(t *testing.T) {
	server, err := Listen("127.0.0.1:0", ServerOptions{LossRate: 1}, discard)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Serve(ctx)
	client := dial(t, server.Addr(), ClientOptions{Timeout: 20 * time.Millisecond, MaxAttempts: 1})
	request := calc.Request{Seq: 0, Left: 2, Op: calc.Add, Right: 2}

	if sample := client.Exchange(request); !sample.Lost {
		t.Fatal("com perda 100% a requisição deveria se perder")
	}
	server.SetLossRate(0)
	if sample := client.Exchange(request); sample.Lost || server.LossRate() != 0 {
		t.Fatalf("depois de zerar a perda deveria haver resposta: %+v", sample)
	}
}

func TestServerUnavailableDoesNotHang(t *testing.T) {
	// Abre e fecha um servidor para obter uma porta que certamente não está escutando.
	server, err := Listen("127.0.0.1:0", ServerOptions{}, discard)
	if err != nil {
		t.Fatal(err)
	}
	address := server.Addr()
	server.Close()

	client := dial(t, address, ClientOptions{Timeout: 20 * time.Millisecond, MaxAttempts: 2})
	start := time.Now()
	sample := client.Exchange(calc.Request{Seq: 0, Left: 1, Op: calc.Add, Right: 1})

	if !sample.Lost {
		t.Fatal("sem servidor a requisição deveria ser perdida")
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("tentativas deveriam respeitar o timeout, duraram %v", elapsed)
	}
}

func TestServerHandlesConcurrentClients(t *testing.T) {
	address := startServer(t, ServerOptions{})

	var wg sync.WaitGroup
	failures := make(chan string, 10)
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := Dial(address, ClientOptions{Timeout: time.Second, MaxAttempts: 3}, discard)
			if err != nil {
				failures <- err.Error()
				return
			}
			defer client.Close()
			summary := sequence.Run(context.Background(), "UDP", client, workload.Generate(20, int64(i+1)), discard)
			if summary.Answered != 20 {
				failures <- "cliente com respostas faltando"
			}
		}()
	}
	wg.Wait()
	close(failures)

	var problems []string
	for failure := range failures {
		problems = append(problems, failure)
	}
	if len(problems) > 0 {
		t.Fatal(strings.Join(problems, "; "))
	}
}
