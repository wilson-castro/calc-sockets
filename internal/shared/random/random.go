// Package random centraliza a criação de geradores pseudoaleatórios para que
// "semente zero" signifique a mesma coisa em todo o projeto.
package random

import "math/rand/v2"

// New devolve um gerador determinístico para seed diferente de zero e um gerador
// com semente sorteada para seed igual a zero.
//
// O gerador devolvido não é seguro para uso concorrente.
func New(seed int64) *rand.Rand {
	s := uint64(seed)
	if seed == 0 {
		s = rand.Uint64()
	}
	return rand.New(rand.NewPCG(s, s^0x9e3779b97f4a7c15))
}
