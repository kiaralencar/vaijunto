# Protocolo VaiJunto

Protocolo de aplicação próprio, sobre socket TCP puro (pacote `net` do Go).
Sem HTTP, sem framework de mensageria ou RPC.

## Transporte e enquadramento

TCP entrega um fluxo contínuo de bytes — não sabe onde uma mensagem termina e
a próxima começa. Quem define isso é este protocolo:

- Cada mensagem é um objeto **JSON compacto** (sem quebras de linha internas).
- Cada mensagem termina com **`\n`**.
- O receptor lê até encontrar `\n` (`bufio.NewReader(conn).ReadString('\n')`)
  e considera aquilo uma mensagem completa.

Essa escolha (JSON Lines) foi preferida a um prefixo de tamanho por ser mais
simples de depurar — dá para inspecionar o tráfego com `netcat`/`telnet`.

## Fluxo de conexão

1. Cliente conecta com `net.Dial("tcp", "host:porta")`.
2. Servidor aceita a conexão (`Listener.Accept()`) e entrega a uma goroutine
   dedicada — a conexão permanece **aberta** durante toda a sessão.
3. Primeira operação esperada do cliente: `LOGIN`.
4. Depois do login, o cliente manda quantas operações quiser, na ordem que
   quiser, todas na mesma conexão (um pedido, uma resposta, cada vez).
5. Desconexão: o cliente fecha o socket, ou cai. O servidor detecta erro na
   leitura, encerra **só aquela goroutine**. O estado do servidor e as demais
   conexões não são afetados.
6. Mensagem malformada (JSON inválido, campo faltando): o servidor responde
   com `"ok": false` e um `erro`, e continua atendendo normalmente na mesma
   conexão — nunca derruba o servidor.

## Envelope

Todo pedido (cliente → servidor):

```json
{"tipo": "NOME_DA_OPERACAO", "id": "opcional", "dados": { ... }}
```

Toda resposta (servidor → cliente):

```json
{"tipo": "NOME_DA_OPERACAO", "id": "opcional", "ok": true, "erro": "", "dados": { ... }}
```

- `id` é opcional e apenas ecoado de volta — reservado para um cliente futuro
  que precise casar pedidos e respostas fora de ordem.
- Quando `ok` é `false`, `erro` traz uma mensagem legível para humano e
  `dados` vem vazio.

## Operações

| Operação | Quem usa | Requer login |
|---|---|---|
| `LOGIN` | motorista e passageiro | — |
| `PUBLICAR_CARONA` | motorista | sim |
| `LISTAR_CARONAS` | motorista e passageiro | não |
| `CONSULTAR_PASSAGEIROS` | motorista | não* |
| `CANCELAR_CARONA` | motorista | sim |
| `BUSCAR_ITINERARIOS` | passageiro | não |
| `RESERVAR` | passageiro | sim |
| `LISTAR_MINHAS_RESERVAS` | passageiro | sim |
| `CANCELAR_RESERVA` | passageiro | sim |

*(`CONSULTAR_PASSAGEIROS` não valida hoje que quem pergunta é o dono da
carona — ver seção "Pendências e perguntas ao tutor" no guia de estudo.)*

### `LOGIN`

Pedido `dados`: `{"nome": "Maria"}`
Resposta `dados`: `{}`

Efeito: associa o nome informado a esta conexão (sessão). Sem senha nesta
versão — decisão a confirmar com o tutor.

### `PUBLICAR_CARONA`

Pedido `dados`:
```json
{
  "rota": ["Salvador", "Vitoria da Conquista"],
  "data": "2026-09-20",
  "preco_por_trecho": [50],
  "assentos_por_trecho": [2]
}
```
Resposta `dados`: `{"carona_id": "c1"}`

Regra: `preco_por_trecho` e `assentos_por_trecho` têm exatamente um valor por
trecho, isto é, `len(rota) - 1` valores.

### `LISTAR_CARONAS`

Pedido `dados`: `{}`
Resposta `dados`:
```json
{
  "caronas": [
    {
      "id": "c1", "motorista": "Joao",
      "rota": ["Salvador", "Vitoria da Conquista"],
      "data": "2026-09-20",
      "preco_por_trecho": [50],
      "assentos_livres": [2]
    }
  ]
}
```

### `CONSULTAR_PASSAGEIROS`

Pedido `dados`: `{"carona_id": "c1"}`
Resposta `dados`:
```json
{"passageiros": [{"passageiro": "Maria", "trecho_inicio": 0, "trecho_fim": 0}]}
```

### `CANCELAR_CARONA`

Pedido `dados`: `{"carona_id": "c1"}`
Resposta `dados`: `{}`

Só o motorista dono da carona pode cancelar.

### `BUSCAR_ITINERARIOS`

Pedido `dados`: `{"origem": "Salvador", "destino": "Vitoria da Conquista", "data": "2026-09-20"}`
Resposta `dados`:
```json
{
  "itinerarios": [
    {"itens": [{"carona_id": "c1", "trecho_inicio": 0, "trecho_fim": 0}], "preco": 50}
  ]
}
```

Como é feito: o servidor monta um **grafo** a partir das caronas ativas
naquela data — cada **cidade** é um nó, cada **trecho com assento livre** é
uma **aresta** (marcada com a carona dona do trecho e o preço). Uma busca em
profundidade (DFS) de `origem` a `destino`, sem revisitar cidade, encontra os
caminhos possíveis; trechos consecutivos da mesma carona são agrupados num
único item de reserva. A busca é limitada a 6 trechos e 10 itinerários por
consulta, para não explodir combinatoriamente.

### `RESERVAR`

Pedido `dados`:
```json
{"itens": [{"carona_id": "c1", "trecho_inicio": 0, "trecho_fim": 0}]}
```
Resposta `dados`: `{"reserva_id": "r1"}`

Regra central do sistema: **todos os itens são validados e confirmados
atomicamente**, sob o mesmo lock do servidor — ou entram todos, ou nenhum.
Um `ReservarReq` pode conter itens de caronas diferentes (itinerário
combinado) ou vários trechos consecutivos da mesma carona; é a mesma
operação nos dois casos, e é a mesma operação usada tanto por uma reserva
manual quanto por uma reserva feita a partir de um resultado do
`BUSCAR_ITINERARIOS`.

### `LISTAR_MINHAS_RESERVAS`

Pedido `dados`: `{}`
Resposta `dados`:
```json
{"reservas": [{"id": "r1", "itens": [{"carona_id": "c1", "trecho_inicio": 0, "trecho_fim": 0}]}]}
```

### `CANCELAR_RESERVA`

Pedido `dados`: `{"reserva_id": "r1"}`
Resposta `dados`: `{}`

Devolve o(s) assento(s) em cada trecho da reserva cancelada.

## Exemplo de troca real (capturado rodando o sistema)

```
cliente-motorista -> servidor:
{"tipo":"LOGIN","dados":{"nome":"Motorista1"}}

servidor -> cliente-motorista:
{"tipo":"LOGIN","ok":true,"dados":{}}

cliente-motorista -> servidor:
{"tipo":"PUBLICAR_CARONA","dados":{"rota":["Salvador","Vitoria da Conquista"],"data":"2026-09-20","preco_por_trecho":[50],"assentos_por_trecho":[2]}}

servidor -> cliente-motorista:
{"tipo":"PUBLICAR_CARONA","ok":true,"dados":{"carona_id":"c1"}}

cliente-passageiro -> servidor:
{"tipo":"LOGIN","dados":{"nome":"Passageiro1"}}

servidor -> cliente-passageiro:
{"tipo":"LOGIN","ok":true,"dados":{}}

cliente-passageiro -> servidor:
{"tipo":"RESERVAR","dados":{"itens":[{"carona_id":"c1","trecho_inicio":0,"trecho_fim":0}]}}

servidor -> cliente-passageiro:
{"tipo":"RESERVAR","ok":true,"dados":{"reserva_id":"r1"}}
```

## Concorrência (resumo — detalhes no relatório)

Todo o estado (caronas, reservas) vive atrás de **um único `sync.Mutex`**.
Cada goroutine (uma por conexão de cliente) só mexe no estado dentro de
`Lock()`/`Unlock()`. Um lock único evita a necessidade de ordenar múltiplos
locks (o que geraria risco de deadlock quando uma reserva mexe em várias
caronas ao mesmo tempo), ao custo de menos paralelismo interno — trade-off
medido pelo teste de carga (`docs/` ou `cmd/servidor/*_test.go`).
