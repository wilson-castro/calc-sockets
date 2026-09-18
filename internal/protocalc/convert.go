// Package protocalc implementa a Parte 4: CalcServerProto e CalcClientProto
// sobre TCP, com mensagens serializadas pelo Protocol Buffers.
package protocalc

import (
	"fmt"

	"calcsockets/internal/protocalc/calcpb"
	"calcsockets/internal/shared/calc"
	"calcsockets/internal/shared/textprotocol"
)

var toProtoOperator = map[calc.Operator]calcpb.Operator{
	calc.Add:      calcpb.Operator_OPERATOR_ADD,
	calc.Subtract: calcpb.Operator_OPERATOR_SUBTRACT,
	calc.Multiply: calcpb.Operator_OPERATOR_MULTIPLY,
	calc.Divide:   calcpb.Operator_OPERATOR_DIVIDE,
}

var fromProtoOperator = invert(toProtoOperator)

// toProto converte a requisição de domínio na mensagem protobuf.
func toProto(request calc.Request) *calcpb.CalcRequest {
	return &calcpb.CalcRequest{
		Seq:   request.Seq,
		Left:  request.Left,
		Op:    toProtoOperator[request.Op],
		Right: request.Right,
	}
}

// fromProto converte a mensagem recebida, rejeitando operador não informado ou desconhecido.
func fromProto(message *calcpb.CalcRequest) (calc.Request, error) {
	request := calc.Request{Seq: message.GetSeq(), Left: message.GetLeft(), Right: message.GetRight()}
	op, ok := fromProtoOperator[message.GetOp()]
	if !ok {
		return request, fmt.Errorf("%w: %v", calc.ErrInvalidOperator, message.GetOp())
	}
	request.Op = op
	return request, nil
}

// respond aplica a regra de negócio a uma requisição protobuf.
func respond(message *calcpb.CalcRequest) *calcpb.CalcResponse {
	response := &calcpb.CalcResponse{Seq: message.GetSeq()}

	request, err := fromProto(message)
	if err == nil {
		var value float64
		if value, err = request.Evaluate(); err == nil {
			response.Outcome = &calcpb.CalcResponse_Result{Result: value}
			return response
		}
	}
	response.Outcome = &calcpb.CalcResponse_Error{Error: err.Error()}
	return response
}

// describeRequest e describeResponse exibem mensagens binárias no mesmo formato
// textual das Partes 1 e 2, para que os logs das três partes sejam comparáveis.
func describeRequest(message *calcpb.CalcRequest) string {
	request, err := fromProto(message)
	if err != nil {
		return fmt.Sprintf("CALC:%d:?", message.GetSeq())
	}
	return textprotocol.EncodeRequest(request)
}

func describeResponse(message *calcpb.CalcResponse) string {
	response := textprotocol.Response{Seq: message.GetSeq()}
	if failure, isError := message.GetOutcome().(*calcpb.CalcResponse_Error); isError {
		response.Failed, response.ErrorMessage = true, failure.Error
	} else {
		response.Value = message.GetResult()
	}
	return textprotocol.EncodeResponse(response)
}

func invert[K, V comparable](m map[K]V) map[V]K {
	inverted := make(map[V]K, len(m))
	for k, v := range m {
		inverted[v] = k
	}
	return inverted
}
