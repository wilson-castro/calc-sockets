// Package calc contém a regra de negócio da calculadora remota, independente de
// transporte e de formato de mensagem. UDP, TCP e Protobuf reutilizam este pacote,
// de modo que as diferenças medidas no experimento vêm apenas da rede.
package calc

import (
	"errors"
	"fmt"
	"math"
)

// Operator é uma das quatro operações aceitas pelo protocolo.
type Operator string

// Operações aceitas, representadas pelo mesmo símbolo usado no protocolo textual.
const (
	Add      Operator = "+"
	Subtract Operator = "-"
	Multiply Operator = "*"
	Divide   Operator = "/"
)

// Operators lista as operações válidas; o gerador de carga sorteia entre elas.
var Operators = []Operator{Add, Subtract, Multiply, Divide}

// Erros de domínio. As mensagens são enviadas ao cliente dentro de ERROR:<n>:<mensagem>.
var (
	ErrDivisionByZero  = errors.New("divisão por zero")
	ErrInvalidOperator = errors.New("operação inválida")
	ErrInvalidOperand  = errors.New("operando inválido")
	ErrOutOfRange      = errors.New("resultado fora do intervalo representável")
)

// Request é uma requisição de cálculo numerada.
type Request struct {
	// Seq é o número de sequência, usado para casar resposta com requisição.
	Seq   uint64
	Left  float64
	Op    Operator
	Right float64
}

// String descreve a expressão, por exemplo "10 + 5".
func (r Request) String() string {
	return fmt.Sprintf("%v %s %v", r.Left, r.Op, r.Right)
}

// ParseOperator converte um símbolo em Operator, devolvendo ErrInvalidOperator
// quando o símbolo não é uma das quatro operações.
func ParseOperator(symbol string) (Operator, error) {
	for _, op := range Operators {
		if string(op) == symbol {
			return op, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidOperator, symbol)
}

// Evaluate executa a operação da requisição.
//
// Devolve ErrDivisionByZero, ErrInvalidOperator, ErrInvalidOperand para NaN ou
// infinito na entrada, e ErrOutOfRange quando o resultado estoura float64.
func (r Request) Evaluate() (float64, error) {
	if !isFinite(r.Left) || !isFinite(r.Right) {
		return 0, ErrInvalidOperand
	}

	var result float64
	switch r.Op {
	case Add:
		result = r.Left + r.Right
	case Subtract:
		result = r.Left - r.Right
	case Multiply:
		result = r.Left * r.Right
	case Divide:
		if r.Right == 0 {
			return 0, ErrDivisionByZero
		}
		result = r.Left / r.Right
	default:
		return 0, fmt.Errorf("%w: %q", ErrInvalidOperator, r.Op)
	}

	if !isFinite(result) {
		return 0, ErrOutOfRange
	}
	return result, nil
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
