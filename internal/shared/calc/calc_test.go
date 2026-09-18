package calc

import (
	"errors"
	"math"
	"testing"
)

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name    string
		request Request
		want    float64
		wantErr error
	}{
		{"soma do enunciado", Request{Left: 10, Op: Add, Right: 5}, 15, nil},
		{"multiplicação decimal do enunciado", Request{Left: 3.5, Op: Multiply, Right: 2}, 7, nil},
		{"subtração com negativo", Request{Left: -3, Op: Subtract, Right: 2}, -5, nil},
		{"divisão", Request{Left: 9, Op: Divide, Right: 4}, 2.25, nil},
		{"divisão por zero do enunciado", Request{Left: 8, Op: Divide, Right: 0}, 0, ErrDivisionByZero},
		{"operador desconhecido", Request{Left: 1, Op: "%", Right: 2}, 0, ErrInvalidOperator},
		{"operando NaN", Request{Left: math.NaN(), Op: Add, Right: 1}, 0, ErrInvalidOperand},
		{"estouro", Request{Left: math.MaxFloat64, Op: Multiply, Right: 10}, 0, ErrOutOfRange},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.request.Evaluate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("erro = %v, esperado %v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("resultado = %v, esperado %v", got, tc.want)
			}
		})
	}
}

func TestParseOperator(t *testing.T) {
	for _, op := range Operators {
		got, err := ParseOperator(string(op))
		if err != nil || got != op {
			t.Fatalf("ParseOperator(%q) = %q, %v", op, got, err)
		}
	}
	if _, err := ParseOperator("^"); !errors.Is(err, ErrInvalidOperator) {
		t.Fatalf("operador inválido aceito: %v", err)
	}
}
