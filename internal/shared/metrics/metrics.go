// Package metrics define as medidas coletadas por requisição e o resumo por
// execução pedidos na Parte 3: tempo total, RTT médio e máximo, retransmissões,
// perdas definitivas e, para a Parte 4, o tamanho médio das mensagens.
package metrics

import "time"

// Sample é o resultado de uma requisição.
type Sample struct {
	Seq uint64
	// Request e Reply são as mensagens em forma legível, para logs e relatórios.
	Request string
	Reply   string
	// RTT vai do primeiro envio até a chegada da resposta. No UDP inclui as esperas
	// de retransmissão, que é o custo que o cliente de fato percebe.
	RTT time.Duration
	// Attempts é a quantidade de envios. Sempre 1 no TCP.
	Attempts int
	// Lost indica que a requisição ficou sem resposta.
	Lost bool
	// Failure explica a perda; vazio quando houve resposta.
	Failure string
	// RequestBytes e ResponseBytes medem a mensagem serializada, sem o enquadramento
	// do transporte.
	RequestBytes  int
	ResponseBytes int
}

// Retransmissions é o número de reenvios além do primeiro envio.
func (s Sample) Retransmissions() int { return max(s.Attempts-1, 0) }

// Summary agrega as amostras de uma sequência completa.
type Summary struct {
	Protocol        string
	Requests        int
	Answered        int
	Lost            int
	Retransmissions int
	TotalTime       time.Duration
	// AvgRTT, MinRTT e MaxRTT consideram apenas requisições respondidas.
	AvgRTT time.Duration
	MinRTT time.Duration
	MaxRTT time.Duration
	// AvgRequestBytes considera todas as requisições; AvgResponseBytes, só as respondidas.
	AvgRequestBytes  float64
	AvgResponseBytes float64
	Samples          []Sample
}

// Summarize calcula o resumo. totalTime é medido por quem executa a sequência.
func Summarize(protocol string, samples []Sample, totalTime time.Duration) Summary {
	summary := Summary{Protocol: protocol, Requests: len(samples), TotalTime: totalTime, Samples: samples}

	var rttSum time.Duration
	var requestBytes, responseBytes int
	for _, sample := range samples {
		summary.Retransmissions += sample.Retransmissions()
		requestBytes += sample.RequestBytes
		if sample.Lost {
			summary.Lost++
			continue
		}

		summary.Answered++
		rttSum += sample.RTT
		responseBytes += sample.ResponseBytes
		if summary.MinRTT == 0 || sample.RTT < summary.MinRTT {
			summary.MinRTT = sample.RTT
		}
		summary.MaxRTT = max(summary.MaxRTT, sample.RTT)
	}

	if summary.Requests > 0 {
		summary.AvgRequestBytes = float64(requestBytes) / float64(summary.Requests)
	}
	if summary.Answered > 0 {
		summary.AvgRTT = rttSum / time.Duration(summary.Answered)
		summary.AvgResponseBytes = float64(responseBytes) / float64(summary.Answered)
	}
	return summary
}
