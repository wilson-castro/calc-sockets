package workload

import (
	"slices"
	"testing"
)

func TestGenerateIsNumberedAndReproducible(t *testing.T) {
	first := Generate(20, 42)
	second := Generate(20, 42)

	if !slices.Equal(first, second) {
		t.Fatal("a mesma seed deveria gerar a mesma sequência")
	}
	for i, request := range first {
		if request.Seq != uint64(i) {
			t.Fatalf("requisição %d com Seq %d", i, request.Seq)
		}
	}
}
