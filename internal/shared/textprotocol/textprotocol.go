// Package textprotocol implementa o formato textual das Partes 1 e 2:
//
//	CALC:<n>:<operando1>:<op>:<operando2>
//	RESULT:<n>:<resultado>
//	ERROR:<n>:<mensagem>
//
// O pacote só converte mensagens. Enquadramento (um datagrama no UDP, uma linha
// terminada em '\n' no TCP) é responsabilidade de cada transporte.
package textprotocol

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"calcsockets/internal/shared/calc"
)

const (
	requestKind = "CALC"
	resultKind  = "RESULT"
	errorKind   = "ERROR"
	separator   = ":"

	// unknownSeq ocupa o lugar de <n> quando a requisição é ilegível demais para
	// que o número de sequência seja recuperado.
	unknownSeq = "?"
)

// ErrMalformed indica mensagem que não segue o formato do protocolo.
var ErrMalformed = errors.New("mensagem malformada")

// Response é uma resposta RESULT ou ERROR já decodificada.
type Response struct {
	Seq uint64
	// Value só tem significado quando Failed é falso.
	Value float64
	// Failed distingue ERROR de RESULT.
	Failed       bool
	ErrorMessage string
}

// EncodeRequest produz CALC:<n>:<operando1>:<op>:<operando2>.
func EncodeRequest(r calc.Request) string {
	return strings.Join([]string{
		requestKind,
		strconv.FormatUint(r.Seq, 10),
		formatOperand(r.Left),
		string(r.Op),
		formatOperand(r.Right),
	}, separator)
}

// DecodeRequest interpreta uma mensagem CALC.
//
// Quando o erro não impede ler o número de sequência, a requisição devolvida
// traz Seq preenchido para que o servidor possa responder ERROR:<n>.
func DecodeRequest(message string) (calc.Request, error) {
	fields := strings.Split(message, separator)
	if len(fields) != 5 || fields[0] != requestKind {
		return calc.Request{}, fmt.Errorf("%w: esperado CALC:<n>:<a>:<op>:<b>", ErrMalformed)
	}

	seq, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return calc.Request{}, fmt.Errorf("%w: número de sequência %q", ErrMalformed, fields[1])
	}
	request := calc.Request{Seq: seq}

	if request.Left, err = parseOperand(fields[2]); err != nil {
		return request, err
	}
	if request.Op, err = calc.ParseOperator(fields[3]); err != nil {
		return request, err
	}
	if request.Right, err = parseOperand(fields[4]); err != nil {
		return request, err
	}
	return request, nil
}

// EncodeResponse produz RESULT:<n>:<valor> ou ERROR:<n>:<mensagem>.
func EncodeResponse(r Response) string {
	seq := strconv.FormatUint(r.Seq, 10)
	if r.Failed {
		return strings.Join([]string{errorKind, seq, r.ErrorMessage}, separator)
	}
	return strings.Join([]string{resultKind, seq, FormatNumber(r.Value)}, separator)
}

// DecodeResponse interpreta RESULT ou ERROR. A mensagem de erro pode conter ':'.
func DecodeResponse(message string) (Response, error) {
	fields := strings.SplitN(message, separator, 3)
	if len(fields) != 3 {
		return Response{}, fmt.Errorf("%w: resposta %q", ErrMalformed, message)
	}

	seq, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return Response{}, fmt.Errorf("%w: número de sequência %q", ErrMalformed, fields[1])
	}

	switch fields[0] {
	case resultKind:
		value, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			return Response{}, fmt.Errorf("%w: resultado %q", ErrMalformed, fields[2])
		}
		return Response{Seq: seq, Value: value}, nil
	case errorKind:
		return Response{Seq: seq, Failed: true, ErrorMessage: fields[2]}, nil
	default:
		return Response{}, fmt.Errorf("%w: tipo %q", ErrMalformed, fields[0])
	}
}

// Respond é o tratamento completo do lado servidor: decodifica, calcula e codifica.
// Nunca falha; qualquer problema vira uma mensagem ERROR.
func Respond(message string) string {
	request, err := DecodeRequest(message)
	if err != nil {
		if errors.Is(err, ErrMalformed) {
			return strings.Join([]string{errorKind, unknownSeq, err.Error()}, separator)
		}
		return EncodeResponse(Response{Seq: request.Seq, Failed: true, ErrorMessage: err.Error()})
	}

	value, err := request.Evaluate()
	if err != nil {
		return EncodeResponse(Response{Seq: request.Seq, Failed: true, ErrorMessage: err.Error()})
	}
	return EncodeResponse(Response{Seq: request.Seq, Value: value})
}

// FormatNumber escreve o número com a menor representação exata e sempre com
// parte decimal, como no exemplo do enunciado (RESULT:0:15.0).
func FormatNumber(v float64) string {
	text := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(text, ".") {
		text += ".0"
	}
	return text
}

// formatOperand escreve operandos inteiros sem parte decimal, como em CALC:0:10:+:5.
func formatOperand(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func parseOperand(text string) (float64, error) {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", calc.ErrInvalidOperand, text)
	}
	return value, nil
}
