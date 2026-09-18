package tcp

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"calcsockets/internal/shared/sequence"
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

// TestRawProtocol conversa com o servidor por um socket cru, como faria o nc,
// para verificar o formato exato das linhas.
func TestRawProtocol(t *testing.T) {
	conn, err := net.Dial("tcp", startServer(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)

	exchanges := map[string]string{
		"CALC:0:10:+:5\n":  "RESULT:0:15.0\n",
		"CALC:1:8:/:0\r\n": "ERROR:1:divisão por zero\n",
		"CALC:2:3.5:*:2\n": "RESULT:2:7.0\n",
		"CALC:3:1:x:2\n":   "ERROR:3:operação inválida: \"x\"\n",
	}
	for request, want := range exchanges {
		if _, err := conn.Write([]byte(request)); err != nil {
			t.Fatal(err)
		}
		got, err := reader.ReadString('\n')
		if err != nil || got != want {
			t.Errorf("%q -> %q (%v), esperado %q", request, got, err, want)
		}
	}
}

func TestSequenceIsDeliveredCompletely(t *testing.T) {
	client, err := Dial(startServer(t), options, discard)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	summary := sequence.Run(context.Background(), "TCP", client, workload.Generate(20, 3), discard)

	if summary.Answered != 20 || summary.Lost != 0 || summary.Retransmissions != 0 {
		t.Fatalf("TCP deveria entregar tudo sem retransmitir: %+v", summary)
	}
}

func TestConnectionRefusedIsReported(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()

	if _, err := Dial(address, options, discard); err == nil {
		t.Fatal("Dial deveria falhar sem servidor escutando")
	}
}

func TestServerShutdownMarksRequestLost(t *testing.T) {
	server, err := Listen("127.0.0.1:0", discard)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background()) }()

	client, err := Dial(server.Addr(), options, discard)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	server.Close()
	<-done
	sample := client.Exchange(workload.Generate(1, 1)[0])
	if !sample.Lost || sample.Failure == "" {
		t.Fatalf("com o servidor fechado a requisição deveria falhar: %+v", sample)
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
			summary := sequence.Run(context.Background(), "TCP", client, workload.Generate(20, int64(i+1)), discard)
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
