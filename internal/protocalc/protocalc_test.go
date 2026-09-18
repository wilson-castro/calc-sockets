package protocalc

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"calcsockets/internal/protocalc/calcpb"
	"calcsockets/internal/shared/calc"
	"calcsockets/internal/shared/sequence"
	"calcsockets/internal/shared/textprotocol"
	"calcsockets/internal/shared/workload"
)

var (
	discard = slog.New(slog.NewTextHandler(io.Discard, nil))
	options = ClientOptions{DialTimeout: time.Second, IOTimeout: 2 * time.Second}
)

func startServer(t *testing.T) string {
	t.Helper()
	server, err := Listen("127.0.0.1:0", discard)
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

func TestStatementExamplesOverProtobuf(t *testing.T) {
	client, err := Dial(startServer(t), options, discard)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

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
			t.Errorf("seq %d: %q (%s), esperado %q", tc.request.Seq, sample.Reply, sample.Failure, tc.reply)
		}
		if sample.RequestBytes == 0 || sample.ResponseBytes == 0 {
			t.Errorf("seq %d: tamanhos não medidos: %+v", tc.request.Seq, sample)
		}
	}
}

func TestUnspecifiedOperatorIsRejected(t *testing.T) {
	response := respond(&calcpb.CalcRequest{Seq: 9, Left: 1, Right: 2})
	if response.GetError() == "" || response.GetSeq() != 9 {
		t.Fatalf("operador ausente deveria gerar erro: %v", response)
	}
}

func TestOperatorMappingIsComplete(t *testing.T) {
	for _, op := range calc.Operators {
		request := calc.Request{Seq: 1, Left: 6, Op: op, Right: 3}
		back, err := fromProto(toProto(request))
		if err != nil || back != request {
			t.Fatalf("ida e volta de %q falhou: %+v, %v", op, back, err)
		}
	}
}

// TestSequenceAndSizeComparison roda a sequência completa e confere que os
// tamanhos medidos batem com a serialização textual das mesmas requisições.
func TestSequenceAndSizeComparison(t *testing.T) {
	client, err := Dial(startServer(t), options, discard)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	requests := workload.Generate(20, 5)
	summary := sequence.Run(context.Background(), "Protobuf", client, requests, discard)
	if summary.Answered != 20 || summary.Lost != 0 {
		t.Fatalf("sequência incompleta: %+v", summary)
	}

	for i, sample := range summary.Samples {
		if sample.Request != textprotocol.EncodeRequest(requests[i]) {
			t.Fatalf("descrição divergente: %q vs %q", sample.Request, textprotocol.EncodeRequest(requests[i]))
		}
	}
}

func TestServerHandlesConcurrentClients(t *testing.T) {
	address := startServer(t)

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := Dial(address, options, discard)
			if err != nil {
				errs <- err
				return
			}
			defer client.Close()
			summary := sequence.Run(context.Background(), "Protobuf", client, workload.Generate(20, int64(i+1)), discard)
			if summary.Answered != 20 {
				errs <- io.ErrUnexpectedEOF
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
