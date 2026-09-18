package metrics

import (
	"reflect"
	"testing"
	"time"
)

func TestSummarizeIgnoresLostRequestsInRTT(t *testing.T) {
	samples := []Sample{
		{Seq: 0, RTT: 2 * time.Millisecond, Attempts: 1, RequestBytes: 10, ResponseBytes: 12},
		{Seq: 1, RTT: 504 * time.Millisecond, Attempts: 2, RequestBytes: 10, ResponseBytes: 14},
		{Seq: 2, Attempts: 5, Lost: true, RequestBytes: 13},
	}

	got := Summarize("UDP", samples, time.Second)

	want := Summary{
		Protocol:         "UDP",
		Requests:         3,
		Answered:         2,
		Lost:             1,
		Retransmissions:  5,
		TotalTime:        time.Second,
		AvgRTT:           253 * time.Millisecond,
		MinRTT:           2 * time.Millisecond,
		MaxRTT:           504 * time.Millisecond,
		AvgRequestBytes:  11,
		AvgResponseBytes: 13,
	}
	got.Samples = nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resumo = %+v\nesperado %+v", got, want)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	if got := Summarize("TCP", nil, 0); got.AvgRTT != 0 || got.Requests != 0 {
		t.Fatalf("resumo vazio inesperado: %+v", got)
	}
}
