// Package workload gera a sequência de requisições aleatórias enviada pelos clientes.
package workload

import (
	"math"
	"math/rand/v2"

	"calcsockets/internal/shared/calc"
	"calcsockets/internal/shared/random"
)

const (
	maxOperand = 1000
	// decimalShare é a fração de operandos gerados com casas decimais, para exercitar
	// os dois tipos de número que o enunciado menciona.
	decimalShare = 0.3
	// zeroDivisorShare força algumas divisões por zero, cobrindo o caminho ERROR.
	zeroDivisorShare = 0.1
)

// Generate devolve n requisições numeradas de 0 a n-1.
// A mesma seed diferente de zero produz sempre a mesma sequência.
func Generate(n int, seed int64) []calc.Request {
	rng := random.New(seed)
	requests := make([]calc.Request, n)
	for i := range requests {
		requests[i] = Next(rng, uint64(i))
	}
	return requests
}

// Next sorteia uma requisição com o número de sequência informado. É usado por
// quem gera carga contínua, como o painel em tempo real.
func Next(rng *rand.Rand, seq uint64) calc.Request {
	op := calc.Operators[rng.IntN(len(calc.Operators))]
	right := operand(rng)
	if op == calc.Divide && rng.Float64() < zeroDivisorShare {
		right = 0
	}
	return calc.Request{Seq: seq, Left: operand(rng), Op: op, Right: right}
}

func operand(rng *rand.Rand) float64 {
	if rng.Float64() < decimalShare {
		return math.Round(rng.Float64()*maxOperand*100) / 100
	}
	return float64(rng.IntN(maxOperand + 1))
}
