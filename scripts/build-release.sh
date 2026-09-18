#!/bin/sh
# Compila a versão de produção: executáveis estáticos (sem cgo) e binários de teste
# pré-compilados, que o console usa para rodar os testes sem o Go instalado.
#
# Uso: build-release.sh <diretório-de-saída> [versão] [GOOS] [GOARCH]
# Sem GOOS/GOARCH, compila para a plataforma de quem executa.
set -eu

out=$1
version=${2:-dev}
export GOOS="${3:-$(go env GOOS)}"
export GOARCH="${4:-$(go env GOARCH)}"
export CGO_ENABLED=0

ext=""
[ "$GOOS" = "windows" ] && ext=".exe"
ldflags="-s -w -X main.version=$version"

mkdir -p "$out/tests"
echo "→ $GOOS/$GOARCH ($version) em $out"

go build -trimpath -ldflags "$ldflags" -o "$out/calc$ext" ./cmd/calc
for part in udp tcp proto; do
  go build -trimpath -ldflags "$ldflags" -o "$out/calc-server-$part$ext" "./cmd/$part/server"
  go build -trimpath -ldflags "$ldflags" -o "$out/calc-client-$part$ext" "./cmd/$part/client"
done
go build -trimpath -ldflags "$ldflags" -o "$out/calc-experiment$ext" ./cmd/experiment

# Um binário por pacote com testes, nomeado pelo diretório do pacote (udp.test,
# tcp.test, ...), como espera internal/interactive/tests.go.
for pkg in $(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./internal/...); do
  go test -c -trimpath -ldflags "-s -w" -o "$out/tests/$(basename "$pkg").test$ext" "$pkg"
done

cp config.json README.md RESPOSTAS.md "$out/"
