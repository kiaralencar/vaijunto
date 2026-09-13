# VaiJunto

Sistema de caronas compartilhadas — servidor central em Go, sobre socket TCP
puro (pacote `net`, sem HTTP e sem framework de mensageria/RPC), com clientes
de terminal para motorista e passageiro. Trabalho da disciplina TEC502 (MI
Concorrência e Conectividade), PBL "VaiJunto".

## Arquitetura

```
cmd/
  servidor/            servidor central (estado em memoria, protegido por mutex)
  cliente-motorista/   cliente de terminal do motorista
  cliente-passageiro/  cliente de terminal do passageiro
internal/
  dominio/             structs do dominio: Carona, Reserva, ItemReserva
  protocolo/           envelope e mensagens do protocolo (compartilhado por todos)
docs/
  protocolo.md          especificacao completa do protocolo, com exemplos
  vaijunto-guia.md       guia de estudo do PBL
  diagrama.pdf           diagrama do problema
docker/                  Dockerfile de cada um dos 3 binarios
docker-compose.yml       para testar os 3 containers juntos, na mesma maquina
```

Ver [docs/protocolo.md](docs/protocolo.md) para o formato de mensagens,
enquadramento (JSON + `\n` sobre TCP), fluxo de conexão e exemplos.

## Por que Go / por que TCP puro

O enunciado proíbe framework de mensageria/RPC e exige socket nativo do
TCP/IP: o pacote `net` da biblioteca padrão do Go cobre exatamente isso
(`Listen`/`Accept`/`Dial`/`Conn`), sem dependências externas. Concorrência é
tratada com uma goroutine por conexão de cliente e um único `sync.Mutex`
protegendo todo o estado do servidor (ver `cmd/servidor/estado.go`).

## Rodando localmente (sem Docker)

Em três terminais, a partir da raiz do repositório:

```bash
go run ./cmd/servidor 8080
go run ./cmd/cliente-motorista 127.0.0.1:8080
go run ./cmd/cliente-passageiro 127.0.0.1:8080
```

O motorista faz login, publica uma carona (rota, data, preço e assentos por
trecho). O passageiro faz login, busca itinerários (origem, destino, data —
o servidor monta um grafo com as caronas daquele dia e faz uma busca em
profundidade) ou reserva manualmente informando carona e trechos, e confirma.

## Rodando com Docker (mesma máquina)

```bash
docker compose up --build servidor
# em outro terminal, para cada cliente:
docker compose run --rm cliente-motorista
docker compose run --rm cliente-passageiro
```

## Rodando em duas máquinas distintas (requisito do enunciado)

Na máquina A (servidor):
```bash
docker build -f docker/servidor.Dockerfile -t vaijunto-servidor .
docker run -p 8080:8080 vaijunto-servidor
```

Na máquina B (cliente), substituindo `<IP-DA-MAQUINA-A>` pelo IP da máquina A
na rede do laboratório:
```bash
docker build -f docker/cliente-motorista.Dockerfile -t vaijunto-cliente-motorista .
docker run -it vaijunto-cliente-motorista <IP-DA-MAQUINA-A>:8080
```

*(Ainda não testado em Docker de verdade neste ambiente de desenvolvimento —
validar no LARSID/LADICA antes da apresentação.)*

## Testes

```bash
go test ./... -v
```

Sobe o servidor de verdade (TCP real) e dispara dezenas de clientes
concorrentes disputando o mesmo trecho, verificando que a capacidade nunca é
excedida, que uma reserva multi-carona nunca confirma pela metade, e mede o
tempo de resposta sob carga (aparece no output do teste). Detalhes em
`cmd/servidor/carga_test.go`.

`go test -race ./...` é recomendado adicionalmente, mas requer um compilador C
de 64 bits — normalmente disponível por padrão em Linux (validar no
laboratório).

## Decisões conscientes (documentar no relatório)

- **Um único mutex global**, em vez de um lock por carona: evita o risco de
  deadlock por ordenação de locks quando uma reserva mexe em várias caronas
  ao mesmo tempo. Custo: menos paralelismo interno — aceitável na escala
  medida pelo teste de carga.
- **Sem persistência**: estado só em memória. O enunciado exige apenas que a
  queda de um *cliente* não corrompa o estado, não que o servidor sobreviva a
  reinícios.
- **Login sem senha**: apenas identificação por nome.

Essas três decisões estão pendentes de confirmação com o tutor (ver
`docs/vaijunto-guia.md`, seção de perguntas em aberto).
