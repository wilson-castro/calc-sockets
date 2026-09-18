# Imagem de produção: só os executáveis estáticos, os testes pré-compilados e a
# configuração, sobre uma base mínima. Não contém Go nem o código-fonte.
#
#   docker build -f deploy/Dockerfile -t calc-sockets .
#   docker run --rm -it calc-sockets                 # console interativo
#   docker run --rm -p 9000:9000/udp calc-sockets calc-server-udp

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY scripts ./scripts
COPY config.json README.md RESPOSTAS.md ./
ARG VERSION=dev
RUN sh scripts/build-release.sh /out "$VERSION"

# Alpine traz o stty (busybox), que o painel em tempo real usa para ler teclas
# sem Enter; uma imagem distroless não teria.
FROM alpine:3.22
RUN adduser -D -u 10001 calc
WORKDIR /app
COPY --from=build /out/ /app/
RUN mkdir results && chown calc:calc results
USER calc
# Os servidores escutam em 0.0.0.0 dentro do contêiner; CALC_SERVER_HOST aponta
# os clientes para outro contêiner quando necessário.
ENV PATH=/app:$PATH \
    CALC_LOG_COLOR=auto
EXPOSE 9000/udp 9001/tcp 9002/tcp
CMD ["calc"]
