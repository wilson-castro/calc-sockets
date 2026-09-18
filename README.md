# Calculadora remota: UDP vs. TCP vs. Protobuf
 O mesmo
serviço de calculadora roda sobre UDP (Parte 1), TCP (Parte 2) e TCP com Protocol
Buffers (Parte 4). Um experimento automatizado (Parte 3) compara os três.

As respostas às questões estão em [RESPOSTAS.md](RESPOSTAS.md). Os números medidos
estão em [results/experiment.md](results/experiment.md).

## Requisitos

Só o **Docker** (com Compose v2), testado com Docker 29 e Compose 5.

Go 1.25, `protoc` 3.21 e `protoc-gen-go` 1.36.6 vêm na imagem de desenvolvimento
(`Dockerfile`), com versões fixas. O código é montado no contêiner, então as edições
feitas no host valem na hora.

Os comandos usam o `./taskw`, um wrapper do [Task](https://taskfile.dev). Se o `task`
estiver instalado, o wrapper o usa. Se não estiver, na primeira execução ele compila o
Task 3.45.4 (versão fixada em `tools/go.mod`) dentro do contêiner, para o sistema da
máquina, e o guarda em `.tools/`. Quem tem o Task pode trocar `./taskw` por `task` em
todos os exemplos. O wrapper funciona em Linux e macOS; no Windows, use o WSL.

> Quem já tem Go 1.25+ e `protoc` instalados pode rodar tudo no host acrescentando
> `RUNTIME=native` a qualquer tarefa, por exemplo `./taskw test RUNTIME=native`.

## Início rápido

```bash
./taskw setup            # imagem, go.sum e código protobuf gerado (uma única vez)
./taskw console          # menu interativo: cada parte, testes, resultados e painel ao vivo
./taskw test             # testes de todas as partes
./taskw experiment:run   # Parte 3 completa; grava results/experiment.{md,json}
```

`./taskw` sem argumentos lista todas as tarefas.

## Console interativo e painel em tempo real

`./taskw console` abre um menu que executa cada parte sem que seja preciso decorar
comandos:

```text
  PARTES DA ATIVIDADE
   1  Parte 1 · UDP            servidor + cliente, perda simulada e retransmissão
   2  Parte 2 · TCP            servidor + cliente, entrega confiável
   3  Parte 3 · Experimento    UDP 0/10/30% vs. TCP, grava o relatório
   4  Parte 4 · Protobuf       servidor + cliente com mensagens binárias

  ACOMPANHAR
   5  Tempo real               painel ao vivo com os três protocolos
   6  Testes                   testes automatizados de cada parte
   7  Resultados               tabelas do último experimento
   8  Configuração             valores em vigor (config.json + CALC_*)
```

A opção **5** (ou `./taskw console:live`) abre um painel que roda UDP, TCP e Protobuf
ao mesmo tempo, com carga contínua. A cada 200 ms ele mostra:

- as métricas de cada protocolo: enviadas, respondidas, retransmissões, perdas, RTT
  último, médio e máximo, e bytes médios;
- um gráfico do RTT das últimas requisições, em escala logarítmica, com as
  retransmitidas em amarelo e as perdidas como ✗;
- a última troca de cada protocolo;
- os eventos dos servidores e clientes, como descartes, retransmissões e respostas.

As teclas `+` e `-` mudam a perda do servidor UDP em 10 pontos com o painel rodando.
`p` pausa, `r` zera as métricas e `q` volta ao menu. Em terminais sem `stty`, como no
Windows fora do WSL, cada tecla precisa de Enter.

As opções também podem ser passadas como argumentos, lidos antes do teclado. Isso
serve para automatizar: `calc 2 0` roda a Parte 2 e sai, `calc 6 t 0` roda todos os
testes, e `calc 1 0.3 0` roda a Parte 1 com 30% de perda.

## Executando cada parte

Cada parte tem três formas de execução:

- **`demo`**: servidor e cliente no mesmo terminal. Os logs aparecem intercalados e
  cada componente tem sua cor.
- **`server` + `client`**: servidor e cliente em terminais separados, cada um no seu
  contêiner, conversando pela rede do Docker Compose.
- **`test`**: testes automatizados da parte.

| Parte | Demo | Servidor (terminal 1) | Cliente (terminal 2) | Testes |
| --- | --- | --- | --- | --- |
| 1: UDP | `./taskw udp:demo LOSS=0.3` | `./taskw udp:server LOSS=0.1` | `./taskw udp:client` | `./taskw udp:test` |
| 2: TCP | `./taskw tcp:demo` | `./taskw tcp:server` | `./taskw tcp:client` | `./taskw tcp:test` |
| 3: Experimento | `./taskw experiment:run` | – | – | `./taskw experiment:test` |
| 4: Protobuf | `./taskw proto:demo` | `./taskw proto:server` | `./taskw proto:client` | `./taskw proto:test` |

`./taskw <parte>:verify` roda os testes e a demo da parte. `./taskw verify` roda formatação,
`go vet`, todos os testes e as três demos.

### Parada e limpeza

| Comando | Efeito |
| --- | --- |
| `./taskw stop` | Para servidores, clientes e demos, em contêiner ou no host (`RUNTIME=native`) |
| `./taskw clean` | `stop` e remove `bin/` e `dist/`; mantém `results/`, as imagens e os caches |
| `./taskw env:purge` | `clean` e também remove as imagens, os volumes de cache e o Task de `.tools/` (pede confirmação; depois rode `./taskw setup`) |

As portas são publicadas no host (UDP 9000, TCP 9001, Protobuf 9002), então também dá
para testar o protocolo textual à mão:

```bash
echo -n "CALC:0:10:+:5" | nc -u -w1 127.0.0.1 9000   # RESULT:0:15.0
printf "CALC:1:8:/:0\n" | nc -q1 127.0.0.1 9001      # ERROR:1:divisão por zero
```

## Versão de produção

`tasks/release.yml` gera a versão de produção. Ela não exige Go nem o código-fonte:
são executáveis estáticos com os testes pré-compilados junto, e o console os usa na
opção 6.

| Comando | Resultado |
| --- | --- |
| `./taskw release:build` | `dist/calc-sockets_<versão>_<so>_<arq>/` para Linux, macOS (amd64 e arm64) e Windows |
| `./taskw release:package` | um `.tar.gz` por plataforma e o `SHA256SUMS` |
| `./taskw release:console` | compila para esta máquina e abre o console do pacote |
| `./taskw release:image` | imagem Docker `calc-sockets:<versão>` (Alpine, usuário sem privilégios, cerca de 86 MB) |
| `./taskw release:run` | abre o console interativo dentro da imagem |
| `./taskw release:server PART=udp LOSS=0.2` | sobe um servidor da imagem com a porta publicada no host |
| `./taskw release:verify` | constrói a imagem e roda nela todos os testes e uma demonstração roteirizada |
| `./taskw release:clean` | remove `dist/` e as imagens de produção |

`VERSION=` (padrão `1.0.0`) e `PLATFORMS=` (lista separada por vírgula, como
`linux/amd64,darwin/arm64`) personalizam o build. Cada pacote contém:

```text
calc                          console interativo (ponto de entrada)
calc-server-{udp,tcp,proto}   servidores de cada parte
calc-client-{udp,tcp,proto}   clientes de cada parte
calc-experiment               Parte 3
tests/*.test                  testes pré-compilados, um por pacote
config.json README.md RESPOSTAS.md
```

Uso direto da imagem:

```bash
docker run --rm -it calc-sockets:1.0.0                                   # console
docker run --rm -p 9000:9000/udp -e CALC_LOSS_RATE=0.1 calc-sockets:1.0.0 calc-server-udp
docker run --rm calc-sockets:1.0.0 calc 6 t 0                            # testes, sem interação
```

## Configuração

Nenhum executável exige argumentos. Os parâmetros saem de três camadas, da menor para a
maior precedência:

1. valores padrão do enunciado, definidos em `internal/shared/config`;
2. `config.json` na raiz (ou o arquivo apontado por `CALC_CONFIG`);
3. variáveis de ambiente `CALC_*`.

| Parâmetro | `config.json` | Variável | Padrão |
| --- | --- | --- | --- |
| Número de requisições (N) | `client.requests` | `CALC_REQUESTS` | 20 |
| Timeout antes de retransmitir | `client.udpTimeoutMs` | `CALC_UDP_TIMEOUT_MS` | 500 |
| Máximo de tentativas UDP | `client.udpMaxAttempts` | `CALC_UDP_MAX_ATTEMPTS` | 5 |
| Taxa de perda do servidor UDP | `udp.lossRate` | `CALC_LOSS_RATE` (ou `LOSS=` no task) | 0.0 |
| Semente da carga (0 = aleatória) | `client.workloadSeed` | `CALC_WORKLOAD_SEED` | 0 |
| Taxas de perda do experimento | `experiment.lossRates` | – | [0, 0.1, 0.3] |
| Semente do experimento | `experiment.seed` | – | 2026 |
| Pausa entre requisições no painel | `live.intervalMs` | – | 300 |
| Perda UDP inicial do painel | `live.lossRate` | – | 0.2 |
| Nível de log | `log.level` | `CALC_LOG_LEVEL` | info |
| Cor nos logs | `log.color` (`auto`, `always`, `never`) | `CALC_LOG_COLOR`, `NO_COLOR` | auto |

O servidor UDP também aceita a flag `--loss-rate`, citada no enunciado:
`go run ./cmd/udp/server --loss-rate 0.1`.

Com `CALC_LOG_LEVEL=debug`, o cliente UDP registra as respostas atrasadas que descarta.

## Estrutura

```text
cmd/                         executáveis finos: só montam dependências e chamam internal/
  udp/server  udp/client     Parte 1: CalcServerUDP e CalcClientUDP
  tcp/server  tcp/client     Parte 2: CalcServerTCP e CalcClientTCP
  proto/server proto/client  Parte 4: CalcServerProto e CalcClientProto
  experiment                 Parte 3: executa os cenários e grava o relatório
  calc                       console interativo (ponto de entrada da versão de produção)
internal/
  udp/                       Parte 1: server.go, client.go, udp_test.go
  tcp/                       Parte 2: server.go, client.go, tcp_test.go
  protocalc/                 Parte 4: server.go, client.go, convert.go, protocalc_test.go
    calcpb/                  calc.proto e o calc.pb.go gerado pelo protoc
  experiment/                Parte 3: experiment.go (cenários), session.go, report.go, testes
  interactive/               console: menu, ações de cada opção, executor de testes, terminal
  live/                      painel em tempo real: métricas, eventos, renderização, orquestração
  shared/                    código usado por mais de uma parte
    calc/                    regra de negócio (operações e erros de domínio)
    textprotocol/            formato CALC / RESULT / ERROR
    sequence/                laço "uma requisição por vez" comum aos clientes
    metrics/                 amostra por requisição e resumo (RTT, retransmissões, bytes)
    workload/                gerador das requisições aleatórias
    config/                  config.json + variáveis CALC_*
    logger/                  logs estruturados coloridos (slog)
    console/                 tabelas e texto com cor no terminal
    app/                     ciclo de vida comum dos executáveis (config, sinais, saída)
    random/                  geradores pseudoaleatórios com semente
tasks/                       Taskfiles: udp, tcp, proto, experiment, console, release, env
scripts/build-release.sh     build da versão de produção (usado pelo Taskfile e pela imagem)
deploy/Dockerfile            imagem de produção
tools/                       módulo Go separado que fixa a versão do Task usada pelo taskw
taskw                        wrapper do Task (só exige Docker)
results/                     relatório do último experimento
```

## Decisões de implementação

- **Somente a API de sockets da biblioteca padrão** (`net.ListenUDP`, `net.DialUDP`,
  `net.Listen`, `net.Dial`). O servidor UDP não usa conexão; o TCP mantém uma conexão
  por cliente.
- **Concorrência.** O servidor UDP lê num único laço e responde cada datagrama numa
  goroutine. Os servidores TCP e Protobuf atendem cada conexão numa goroutine própria,
  que é o equivalente em Go a uma thread por cliente.
- **Perda simulada** no servidor UDP: o datagrama é recebido, mas nenhuma resposta é
  enviada. A perda ocorre depois da leitura, então não depende da rede.
- **Retransmissão no cliente UDP.** Timeout de 500 ms, até 5 envios. Respostas
  atrasadas, com número de sequência diferente do esperado, são descartadas.
- **RTT** vai do primeiro envio até a resposta. No UDP ele inclui as esperas de
  retransmissão, que é a latência que o usuário percebe.
- **Tamanho das mensagens** conta só a mensagem serializada, sem o enquadramento: `\n`
  no texto, prefixo varint no protobuf, 1 byte cada nas mensagens deste trabalho.
- **Reprodutibilidade.** O experimento fixa a semente da carga e da perda. Com isso,
  retransmissões e bytes se repetem entre execuções; só os tempos variam.
