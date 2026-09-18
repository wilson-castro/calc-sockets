# Ambiente de desenvolvimento com versões fixas de Go, protoc e protoc-gen-go.
# O código não é copiado para a imagem: o repositório é montado em /app pelo
# docker-compose.yml, então a imagem só precisa ser reconstruída quando uma
# destas versões mudar.
FROM golang:1.25-bookworm

ARG PROTOC_GEN_GO_VERSION=v1.36.6

RUN apt-get update \
 && apt-get install -y --no-install-recommends protobuf-compiler \
 && rm -rf /var/lib/apt/lists/*

RUN GOBIN=/usr/local/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION}

# O contêiner roda com o UID do usuário do host para que bin/ e results/ não
# fiquem com dono root; por isso os caches ficam em diretórios graváveis por todos.
RUN mkdir -p /cache/go-build /cache/go-mod /cache/gopath && chmod -R 0777 /cache
ENV GOPATH=/cache/gopath \
    GOCACHE=/cache/go-build \
    GOMODCACHE=/cache/go-mod \
    GOTOOLCHAIN=local \
    GOFLAGS=-buildvcs=false \
    HOME=/tmp

WORKDIR /app
