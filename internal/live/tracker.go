// Package live implementa o painel em tempo real do console interativo: os três
// protocolos rodam ao mesmo tempo, com carga contínua, e o terminal mostra as
// métricas, o histórico de RTT e os eventos à medida que acontecem.
package live

import (
	"sync"
	"time"

	"calcsockets/internal/shared/metrics"
)

// historySize é quantas requisições recentes o gráfico de RTT mostra.
const historySize = 48

// Point é uma requisição no histórico do gráfico.
type Point struct {
	RTT      time.Duration
	Attempts int
	Lost     bool
}

// Snapshot é uma cópia consistente das métricas de um protocolo.
type Snapshot struct {
	Sent            int
	Answered        int
	Retransmissions int
	Lost            int
	LastRTT         time.Duration
	AvgRTT          time.Duration
	MaxRTT          time.Duration
	AvgRequestB     float64
	AvgResponseB    float64
	History         []Point
	Last            metrics.Sample
}

// Tracker acumula as amostras de um protocolo. É seguro para uso concorrente:
// o cliente grava enquanto o renderizador lê.
type Tracker struct {
	mu            sync.Mutex
	snapshot      Snapshot
	rttSum        time.Duration
	requestBytes  int
	responseBytes int
}

// Record contabiliza uma requisição concluída, respondida ou perdida.
func (t *Tracker) Record(sample metrics.Sample) {
	t.mu.Lock()
	defer t.mu.Unlock()

	s := &t.snapshot
	s.Sent++
	s.Retransmissions += sample.Retransmissions()
	s.Last = sample
	t.requestBytes += sample.RequestBytes

	if sample.Lost {
		s.Lost++
	} else {
		s.Answered++
		s.LastRTT = sample.RTT
		s.MaxRTT = max(s.MaxRTT, sample.RTT)
		t.rttSum += sample.RTT
		t.responseBytes += sample.ResponseBytes
	}

	s.History = append(s.History, Point{RTT: sample.RTT, Attempts: sample.Attempts, Lost: sample.Lost})
	if len(s.History) > historySize {
		s.History = s.History[len(s.History)-historySize:]
	}
}

// Snapshot devolve as métricas atuais com as médias já calculadas.
func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()

	s := t.snapshot
	s.History = append([]Point(nil), t.snapshot.History...)
	if s.Answered > 0 {
		s.AvgRTT = t.rttSum / time.Duration(s.Answered)
		s.AvgResponseB = float64(t.responseBytes) / float64(s.Answered)
	}
	if s.Sent > 0 {
		s.AvgRequestB = float64(t.requestBytes) / float64(s.Sent)
	}
	return s
}

// Reset zera as métricas, como se o painel tivesse acabado de abrir.
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.snapshot, t.rttSum, t.requestBytes, t.responseBytes = Snapshot{}, 0, 0, 0
}
