BIN_DIR := bin
PORTA ?= 8080
ENDERECO ?= 127.0.0.1:8080

.PHONY: help build test test-race vet fmt run-servidor run-motorista run-passageiro docker-build clean

help:
	@echo "alvos disponiveis:"
	@echo "  build          - compila servidor e os dois clientes em $(BIN_DIR)/"
	@echo "  test           - roda todos os testes (inclui o teste de concorrencia)"
	@echo "  test-race      - roda os testes com -race (precisa de gcc; funciona em Linux)"
	@echo "  vet            - go vet em todo o modulo"
	@echo "  fmt            - lista arquivos fora do padrao gofmt"
	@echo "  run-servidor   - roda o servidor (PORTA=$(PORTA))"
	@echo "  run-motorista  - roda o cliente motorista (ENDERECO=$(ENDERECO))"
	@echo "  run-passageiro - roda o cliente passageiro (ENDERECO=$(ENDERECO))"
	@echo "  docker-build   - builda as 3 imagens Docker"
	@echo "  clean          - remove os binarios compilados"

test:
	go test ./... -v

test-race:
	go test ./... -race -v

vet:
	go vet ./...

fmt:
	gofmt -l .

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/servidor ./cmd/servidor
	go build -o $(BIN_DIR)/cliente-motorista ./cmd/cliente-motorista
	go build -o $(BIN_DIR)/cliente-passageiro ./cmd/cliente-passageiro

run-servidor:
	go run ./cmd/servidor $(PORTA)

run-motorista:
	go run ./cmd/cliente-motorista $(ENDERECO)

run-passageiro:
	go run ./cmd/cliente-passageiro $(ENDERECO)

docker-build:
	docker build -f docker/servidor.Dockerfile -t vaijunto-servidor .
	docker build -f docker/cliente-motorista.Dockerfile -t vaijunto-cliente-motorista .
	docker build -f docker/cliente-passageiro.Dockerfile -t vaijunto-cliente-passageiro .

clean:
	rm -rf $(BIN_DIR)
