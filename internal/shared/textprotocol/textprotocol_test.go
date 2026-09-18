package textprotocol

import (
	"errors"
	"strings"
	"testing"

	"calcsockets/internal/shared/calc"
)

// TestRespondStatementExamples reproduz literalmente a interação do enunciado.
func TestRespondStatementExamples(t *testing.T) {
	cases := map[string]string{
		"CALC:0:10:+:5":  "RESULT:0:15.0",
		"CALC:1:8:/:0":   "ERROR:1:divisão por zero",
		"CALC:2:3.5:*:2": "RESULT:2:7.0",
	}
	for request, want := range cases {
		if got := Respond(request); got != want {
			t.Errorf("Respond(%q) = %q, esperado %q", request, got, want)
		}
	}
}

func TestRespondInvalidInput(t *testing.T) {
	cases := map[string]string{
		"CALC:3:1:%:2":   "ERROR:3:operação inválida",
		"CALC:4:abc:+:2": "ERROR:4:operando inválido",
		"CALC:5:-3:-:-2": "RESULT:5:-1.0",
		"lixo":           "ERROR:?:mensagem malformada",
	}
	for request, wantPrefix := range cases {
		if got := Respond(request); !strings.HasPrefix(got, wantPrefix) {
			t.Errorf("Respond(%q) = %q, esperado prefixo %q", request, got, wantPrefix)
		}
	}
}

func TestRequestRoundTrip(t *testing.T) {
	original := calc.Request{Seq: 17, Left: -2.75, Op: calc.Divide, Right: 4}
	message := EncodeRequest(original)
	if message != "CALC:17:-2.75:/:4" {
		t.Fatalf("codificação inesperada: %q", message)
	}
	decoded, err := DecodeRequest(message)
	if err != nil || decoded != original {
		t.Fatalf("ida e volta falhou: %+v, %v", decoded, err)
	}
}

func TestDecodeResponse(t *testing.T) {
	result, err := DecodeResponse("RESULT:7:2.5")
	if err != nil || result != (Response{Seq: 7, Value: 2.5}) {
		t.Fatalf("RESULT decodificado errado: %+v, %v", result, err)
	}

	failure, err := DecodeResponse("ERROR:8:falha: detalhe com dois pontos")
	if err != nil || !failure.Failed || failure.ErrorMessage != "falha: detalhe com dois pontos" {
		t.Fatalf("ERROR decodificado errado: %+v, %v", failure, err)
	}

	if _, err := DecodeResponse("ERROR:?:mensagem malformada"); !errors.Is(err, ErrMalformed) {
		t.Fatalf("sequência desconhecida deveria ser malformada: %v", err)
	}
}
