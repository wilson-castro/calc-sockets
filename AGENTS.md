# AGENTS.md

Instruções para quem altera este repositório, seja pessoa ou agente de código. Leia
antes de editar. Os comandos abaixo usam `./taskw`; com o Task instalado, `task` é
equivalente.

## Comandos

| Objetivo | Comando |
| --- | --- |
| Preparar o ambiente | `./taskw setup` |
| Testes de uma parte | `./taskw udp:test`, `./taskw tcp:test`, `./taskw proto:test`, `./taskw experiment:test`, `./taskw shared:test` |
| Todos os testes | `./taskw test` |
| Antes de concluir qualquer mudança | `./taskw verify` |
| Regenerar o código protobuf | `./taskw env:gen` (depois de editar `calc.proto`) |
| Formatar | `./taskw env:fmt` |
| Testes do console e do painel | `./taskw console:test` |
| Validar a versão de produção | `./taskw release:verify` |
| Parar o que estiver rodando | `./taskw stop` |
| Limpar binários | `./taskw clean` (nunca apaga `results/`, que faz parte da entrega) |

Tudo roda no contêiner de desenvolvimento. Não instale ferramentas no host para
contornar uma falha do ambiente. Corrija o `Dockerfile` ou o `docker-compose.yml`.

## Organização

- Cada parte da atividade tem um pacote em `internal/` (`udp`, `tcp`, `protocalc`,
  `experiment`) e um Taskfile em `tasks/`. Código de uma parte não importa código de
  outra parte. Só os orquestradores importam várias partes: `experiment`, que as
  compara, e `interactive` e `live`, que as exibem.
- Um pacote novo com testes precisa entrar em `testSuites`
  (`internal/interactive/tests.go`); `TestSuitesCoverEveryPackageWithTests` falha se
  isso for esquecido. O build da release compila os testes de todo pacote que os tiver.
- Servidor e cliente ficam sempre em arquivos separados (`server.go`, `client.go`), e os
  testes da parte ficam no mesmo pacote (`<parte>_test.go`).
- Código usado por duas ou mais partes vai para `internal/shared/<responsabilidade>`.
  Não crie pacotes genéricos como `utils` ou `helpers`.
- `cmd/<parte>/<papel>/main.go` só monta dependências e chama `internal/`. Lógica
  testável não fica em `cmd/`.
- `internal/protocalc/calcpb/calc.pb.go` é gerado. Nunca edite esse arquivo à mão.

## Estilo de código

- O código deve se explicar sozinho. Use nomes que digam a intenção
  (`shouldDrop`, `waitOut`, `withServer`) e funções curtas com uma responsabilidade.
  Se um comentário só repete o que a linha faz, melhore o nome e apague o comentário.
- Identificadores em inglês, seguindo o idioma do Go. Comentários, mensagens de log e
  mensagens de erro em português.
- Mensagens de log em minúsculas e curtas. Os dados vão em atributos `chave=valor`, não
  interpolados na mensagem: `log.Info("requisição respondida", "seq", n)`.
- Obtenha loggers por `app.Env.Logger("<parte>-<papel>")`. O prefixo antes do hífen
  escolhe a cor, e o papel `server` fica em negrito.
- Parâmetros novos entram em `config.Config`, no `config.json` e na tabela de
  configuração do README. Não crie flags de linha de comando.
- Erros são tratados sem derrubar o processo. Servidores registram o erro e seguem
  atendendo. Clientes marcam a requisição como perdida e seguem a sequência.

## Documentação no código

- Todo pacote tem comentário de pacote que diz qual parte da atividade ele implementa.
- Todo identificador exportado tem comentário de documentação no formato do Go,
  começando pelo nome do identificador.
- Comentários de implementação explicam o **porquê**: uma decisão, uma invariante, uma
  limitação do protocolo. Exemplos no repositório: `waitOut` em `internal/udp/client.go`
  e `maxMessageSize` em `internal/protocalc/server.go`.
- Não escreva motivos que você não confirmou. Se não souber por que algo existe,
  pergunte ou deixe o código como está.
- Ao mudar comportamento, atualize na mesma mudança o comentário, o README e, se a
  medição mudar, o RESPOSTAS.md.

## Testes

- Testes sobem servidores reais em `127.0.0.1:0` e usam os sockets do sistema. Não
  use mocks de rede.
- Cubra, em cada parte: os exemplos do enunciado, a sequência completa de N
  requisições, clientes concorrentes e ao menos um caminho de erro (servidor ausente,
  mensagem inválida, divisão por zero).
- Testes com perda usam semente fixa e timeouts curtos (dezenas de ms) para rodar
  rápido e de forma determinística.
- A suíte roda com `-race`. Uma condição de corrida conta como teste falhando.
