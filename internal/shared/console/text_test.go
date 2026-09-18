package console

import (
	"testing"

	"calcsockets/internal/shared/logger"
)

func TestVisibleWidthIgnoresColors(t *testing.T) {
	colored := logger.Paint(true, logger.Red, "perda") + " ok"
	if got := VisibleWidth(colored); got != 8 {
		t.Fatalf("largura visível = %d, esperado 8", got)
	}
}

func TestTruncateKeepsColorsBalanced(t *testing.T) {
	colored := logger.Paint(true, logger.Green, "requisição respondida")
	got := Truncate(colored, 10)
	if VisibleWidth(got) != 10 {
		t.Fatalf("largura após corte = %d (%q)", VisibleWidth(got), got)
	}
	if got[len(got)-len(logger.Reset):] != logger.Reset {
		t.Fatalf("corte deveria terminar fechando a cor: %q", got)
	}
	if Truncate("curto", 10) != "curto" {
		t.Fatal("texto menor que a largura não deveria mudar")
	}
}
