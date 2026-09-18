# Respostas e análise

Os números abaixo vêm de `./taskw experiment:run`, com N = 20, timeout de 500 ms, até 5
tentativas e seed 2026. O relatório completo, com cada requisição, está em
[results/experiment.md](results/experiment.md). Servidor e cliente rodaram na mesma
máquina (loopback), então o RTT sem perda mede basicamente o custo do sistema
operacional e da aplicação, não da rede física.

## Parte 3: resultados

| Execução | Tempo total | RTT médio | RTT máx | Retransmissões | Perdidas |
| --- | --- | --- | --- | --- | --- |
| UDP, perda 0% | 4,2 ms | 0,17 ms | 0,53 ms | 0 | 0/20 |
| UDP, perda 10% | 1006,7 ms | 50,3 ms | 501,2 ms | 2 | 0/20 |
| UDP, perda 30% | 4014,1 ms | 200,6 ms | 1002,3 ms | 8 | 0/20 |
| TCP | 2,8 ms | 0,11 ms | 0,39 ms | não se aplica | 0/20 |

### O que os números mostram

**Sem perda, UDP e TCP empatam.** A diferença entre 0,17 ms e 0,11 ms de RTT médio é
ruído de escalonamento. O handshake do TCP acontece uma única vez, antes da sequência,
e é diluído nas 20 requisições.

**Com perda, o tempo total é quase só espera de timeout.** Cada retransmissão custa um
timeout inteiro (500 ms) antes do reenvio. Por isso o tempo total fica perto de
`retransmissões × 500 ms`: 2 × 500 ≈ 1007 ms e 8 × 500 ≈ 4014 ms. O processamento
das 20 requisições leva poucos milissegundos. O RTT máximo de 1002 ms, no cenário de
30%, é uma requisição perdida duas vezes seguidas.

**As retransmissões batem com o esperado.** Se cada envio se perde com probabilidade
*p*, o número esperado de retransmissões por requisição é aproximadamente
*p* / (1 − *p*):

| Perda | Esperado para 20 requisições | Medido |
| --- | --- | --- |
| 10% | 20 × 0,1 / 0,9 ≈ 2,2 | 2 |
| 30% | 20 × 0,3 / 0,7 ≈ 8,6 | 8 |

**Nenhuma requisição foi perdida definitivamente, mas isso pode acontecer.** Uma
requisição só é abandonada se as 5 tentativas se perderem, o que ocorre com
probabilidade *p*⁵. Com 10% de perda isso dá 0,001% por requisição. Com 30%, dá 0,24%
por requisição, ou cerca de 4,7% de chance de ao menos uma perda definitiva nas 20.
O teste `TestExhaustedAttemptsMarkRequestAsLost` força esse caso com 100% de perda.

**O TCP não precisou de nenhuma retransmissão na aplicação.** Esta comparação tem um
limite: a perda simulada foi aplicada só ao servidor UDP, como pede o enunciado, e o
loopback praticamente não perde pacotes. Numa rede real com perda, o TCP também
retransmitiria, mas dentro do kernel e sem que a aplicação percebesse. O custo apareceria
como aumento de latência, não como mensagem faltando.

## "Por que o TCP não perde mensagens e o UDP sim (e o que custa resolver isso)?"

**O UDP é um serviço de datagramas sem confirmação.** Cada `send` vira um pacote IP
independente. Não há número de sequência, confirmação (ACK) nem retransmissão. Se o
pacote some na rede, num buffer cheio ou, como aqui, no servidor, ninguém avisa o
remetente. É a *falha de omissão* do Capítulo 4. O checksum do UDP só detecta
corrupção; o datagrama corrompido é descartado em silêncio. Por isso quem precisa de
confiabilidade sobre UDP tem de implementá-la na aplicação, como fez o `CalcClientUDP`
com timeout, retransmissão e número de sequência para casar resposta com requisição.

**O TCP implementa essa confiabilidade dentro do protocolo.** Cada byte do fluxo tem
número de sequência. O receptor confirma o que recebeu com ACKs cumulativos. O
remetente mantém cópia do que não foi confirmado e retransmite quando o timeout
adaptativo (RTO, calculado a partir do RTT medido) expira ou quando recebe três ACKs
duplicados (*fast retransmit*). O receptor reordena segmentos fora de ordem e descarta
duplicatas antes de entregar os bytes à aplicação. Por isso o `CalcClientTCP` não
precisa de lógica de retransmissão: para a aplicação, uma perda de pacote aparece só
como atraso.

**O que custa resolver isso:**

- **Latência na perda.** Recuperar um pacote exige esperar um timeout ou os ACKs
  duplicados. No experimento, cada perda custou 500 ms, cerca de 3.000 vezes o RTT
  normal. O TCP ajusta o RTO ao RTT medido e costuma esperar menos que um timeout fixo
  de aplicação, mas o princípio é o mesmo.
- **Estabelecimento de conexão.** O TCP gasta um RTT no *three-way handshake* antes do
  primeiro dado e mais trocas no encerramento. Numa interação curta de uma requisição só,
  esse custo pesa. Numa sequência longa na mesma conexão, como aqui, ele é diluído.
- **Estado no servidor.** Cada cliente TCP ocupa um socket, buffers de envio e
  recepção e, nesta implementação, uma goroutine. O servidor UDP atende todos os
  clientes com um único socket e não guarda estado por cliente.
- **Bloqueio na cabeça da fila (*head-of-line blocking*).** Como o TCP entrega em ordem,
  um segmento perdido retém todos os posteriores, mesmo os que já chegaram. No UDP, cada
  requisição é independente.
- **Cabeçalho e ACKs.** O cabeçalho TCP tem no mínimo 20 bytes, contra 8 do UDP. Os ACKs
  também são tráfego extra.
- **Semântica de repetição.** A retransmissão na aplicação dá semântica *pelo menos uma
  vez*: se a resposta, e não a requisição, se perdesse, o servidor executaria a mesma
  operação duas vezes. Aqui isso é inofensivo, porque as operações são idempotentes. Para
  operações não idempotentes, o servidor precisaria guardar o histórico de respostas por
  número de sequência e reenviar a resposta salva, o que dá semântica *no máximo uma vez*.
  O TCP evita esse problema dentro de uma conexão porque descarta duplicatas.

**Quando usar cada um:** UDP quando a latência importa mais que a completude ou quando
a aplicação sabe lidar melhor com a perda, como em streaming, jogos, DNS ou medições
periódicas. TCP quando cada mensagem precisa chegar, e em ordem, como nesta
calculadora, em transferência de arquivos ou em RPC.

## Parte 4: Protobuf vs. texto

| Formato | Requisição (média) | Resposta (média) | Total por troca | vs. texto |
| --- | --- | --- | --- | --- |
| Texto (Parte 2) | 17,8 B | 19,6 B | 37,4 B | referência |
| Protobuf (Parte 4) | 21,9 B | 10,9 B | 32,8 B | −12,2% |

O protobuf não é menor em todas as mensagens. O resultado depende do tipo dos dados:

- **A requisição ficou 23% maior.** Os operandos são `double`, que o protobuf codifica
  sempre em 8 bytes mais 1 byte de tag. Um operando pequeno como `10` custa 2 bytes no
  texto e 9 no protobuf. A requisição tem dois operandos, e o gerador de carga sorteia
  70% deles como inteiros de 0 a 1000, que ocupam no máximo 4 bytes em texto.
- **A resposta ficou 44% menor.** Um resultado como `0.10255952380952382` ocupa 19
  bytes em texto e os mesmos 9 bytes em binário. Divisões e multiplicações decimais
  geram resultados longos, e aí o formato binário ganha. O prefixo textual `RESULT:`
  também desaparece.
- **Campos com valor padrão não são transmitidos.** Seq 0 ou operando 0 simplesmente
  não aparecem na mensagem binária.

Para reduzir a requisição, os operandos poderiam usar um `oneof` com `sint64` para
inteiros (varint com codificação zig-zag, de 1 a 2 bytes para valores pequenos) e
`double` só para decimais. Mantivemos `double` porque o contrato fica mais simples e
corresponde diretamente ao tipo numérico da calculadora.

As vantagens do protobuf aqui vão além do tamanho. O esquema é tipado e validado (o
operador é um `enum`, sem parsing de texto). Não há ambiguidade de separador, e uma
mensagem de erro pode conter `:` sem quebrar nada. Campos novos podem ser adicionados
sem quebrar clientes antigos. O custo é a etapa de geração de código com `protoc` e a
necessidade de enquadramento explícito no TCP, feito com um prefixo varint de tamanho.
Mensagens binárias também não podem ser inspecionadas com `nc`.

O RTT com protobuf (0,20 ms) ficou na mesma ordem do texto (0,11 ms). Com 20 amostras em
loopback, essa diferença está dentro do ruído de medição.
