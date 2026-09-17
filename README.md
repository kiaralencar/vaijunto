# VAIJUNTO - Caronas Compartilhadas 

Sistema de caronas compartilhadas para viagens de média e longa distância: um
servidor central mantém o estado de caronas e reservas; motoristas publicam
caronas e passageiros buscam, combinam e reservam trechos delas.

## Contexto

Este projeto é o Problema 1 da disciplina TEC502 — MI Concorrência e
Conectividade, do semestre 2026.2 da Universidade Estadual de Feira de
Santana (UEFS), curso de Engenharia de Computação, desenvolvido como 
trabalho individual dentro de um PBL (Problem-Based Learning). Esta seção 
existe para que alguém sem contato com a disciplina entenda o que o projeto 
precisa cumprir e por quê; as seções seguintes descrevem a solução construída.

### O problema proposto

Uma startup de mobilidade mantém um servidor central onde motoristas
publicam caronas — informando rota (sequência ordenada de cidades), data,
assentos disponíveis e preço por trecho — e passageiros consultam e
reservam assentos, sem intermediação humana. Como o veículo passa por
cidades intermediárias, um passageiro pode embarcar e desembarcar em
qualquer par de cidades da rota, e a disponibilidade de assentos é
controlada por trecho, não pela carona inteira. Um itinerário pode ser
formado por trechos de uma única carona ou combinar trechos de caronas de
motoristas diferentes (ex.: Salvador → Feira de Santana com um motorista,
Feira de Santana → Vitória da Conquista com outro). A confirmação de um
itinerário precisa ser atômica: ou todos os trechos são reservados, ou
nenhum — nunca o mesmo assento do mesmo trecho para dois passageiros
distintos, e nunca assentos permanentemente bloqueados por uma reserva
nunca concluída.

### Entregáveis exigidos

- Servidor central mantendo o estado das caronas e reservas.
- Cliente motorista: autenticar-se, publicar carona (rota, data, assentos e
  preço por trecho), consultar caronas publicadas e passageiros confirmados
  por trecho, cancelar carona.
- Cliente passageiro: autenticar-se, buscar itinerários entre origem e
  destino numa data, confirmar reserva de um itinerário (um ou mais
  trechos), consultar e cancelar reservas.
- Especificação completa do protocolo de aplicação, documentada no
  repositório: formato das mensagens, campos de cada operação, fluxo de
  conexão/desconexão, exemplos concretos.
- Teste de software automatizado que submeta o servidor a múltiplos
  clientes simultâneos disputando os mesmos trechos, verificando ausência
  de venda dupla de assento e de reserva confirmada pela metade, e medindo
  o tempo de resposta sob carga.

### Restrições impostas pelo enunciado

- Comunicação implementada sobre a interface de socket nativa do TCP/IP —
  nada de HTTP/HTTPS nem de `net/http`.
- Nenhum framework de troca de mensagens ou de chamada remota de
  procedimento (sem gRPC, RabbitMQ, ZeroMQ, etc.).
- Um único servidor central, sem réplicas.
- Controle de concorrência é responsabilidade da própria solução — não pode
  ser delegado a um SGBD ou serviço externo de coordenação.
- O servidor deve atender vários clientes simultaneamente e permanecer
  disponível mesmo que um cliente caia abruptamente.
- Dados trocados em uma representação intermediária bem definida, cabendo
  ao receptor validar e descartar mensagens malformadas.
- Backend desenvolvido e testado por meio de contêineres Docker, executados
  em computadores distintos do laboratório para uma emulação realista.

## Arquitetura

O sistema tem três programas independentes, todos em Go, dentro do mesmo
módulo:

- **Servidor** (`cmd/servidor`) — mantém todo o estado em memória (caronas,
  reservas, senhas), atende múltiplas conexões simultâneas e é a única
  parte que decide o que é ou não permitido.
- **Cliente motorista** (`cmd/cliente-motorista`) e **cliente passageiro**
  (`cmd/cliente-passageiro`) — programas de terminal que só sabem falar o
  protocolo; nenhuma regra de negócio é decidida no lado do cliente.

Dois pacotes compartilhados, em `internal/` (só acessíveis dentro deste
módulo):

- `internal/dominio` — as entidades do problema (`Carona`, `Reserva`,
  `ItemReserva`), usadas só pelo servidor.
- `internal/protocolo` — o formato das mensagens trocadas entre cliente e
  servidor, compartilhado pelos três programas.

O estado nunca é persistido em disco — vive inteiramente na memória do
processo do servidor e é perdido se ele for reiniciado. O enunciado exige
apenas que a queda de um *cliente* não corrompa o estado nem derrube o
serviço; nada é exigido sobre sobreviver a um reinício do servidor.

## Conectividade

A comunicação usa apenas o pacote `net` da biblioteca padrão do Go — sem
HTTP e sem biblioteca de terceiros. O servidor abre uma porta com
`net.Listen`, aceita conexões em laço com `Accept()` e entrega cada uma a
uma goroutine própria, para nunca ficar preso conversando com um cliente só
enquanto outros esperam.

TCP entrega um fluxo contínuo de bytes, sem conceito de onde uma mensagem
termina — por isso o protocolo define seu próprio enquadramento: cada
mensagem é um JSON compacto seguido de `\n` (formato conhecido como *JSON
Lines*), decodificado com `bufio.Reader.ReadString('\n')`. A conexão
permanece aberta durante toda a sessão do cliente — várias operações são
trocadas na mesma conexão, sem reconectar a cada pedido.

TCP (em vez de UDP) foi escolhido porque o protocolo depende de entrega
confiável e ordenada: perder ou reordenar silenciosamente uma mensagem como
`RESERVAR` seria inaceitável, e reimplementar confirmação/reordenação por
conta própria não traria vantagem nenhuma para um protocolo
requisição-resposta como este.

Cada conexão tem um timeout de leitura: se um cliente ficar muito tempo sem
mandar nenhuma mensagem, o servidor encerra a conexão em vez de manter a
goroutine presa indefinidamente. Uma queda abrupta de cliente (ou o
timeout) encerra apenas aquela goroutine — o servidor e as demais conexões
seguem intactos.

A especificação completa do protocolo — as 9 operações, formato de cada
mensagem, exemplos concretos capturados rodando o sistema — está em
[`docs/protocolo.md`](docs/protocolo.md).

## Concorrência

Cada cliente conectado roda em sua própria goroutine, e todas compartilham
o mesmo estado (as mesmas caronas e reservas). Esse estado é protegido por
um único `sync.Mutex`: toda operação que lê ou altera caronas, reservas ou
senhas faz isso dentro de `Lock()`/`Unlock()`, nunca por fora.

Optou-se por **um mutex global**, em vez de um lock por carona, porque uma
reserva pode envolver várias caronas de motoristas diferentes ao mesmo
tempo — locks por recurso exigiriam definir uma ordem fixa de travamento
para evitar deadlock (duas reservas travando as mesmas duas caronas em
ordens opostas). Um lock único elimina esse risco por construção, ao custo
de menos paralelismo interno dentro do servidor — o teste de carga (abaixo)
mede se isso é aceitável na escala do problema.

A operação de reserva é atômica por construção: dentro de uma única seção
crítica, primeiro **valida** todos os trechos de todos os itens pedidos (a
reserva pode combinar trechos de caronas diferentes) e só depois **aplica**
a mudança se tudo passar. Como as duas passadas acontecem sob o mesmo lock,
nenhuma outra goroutine pode intercalar uma alteração no meio do caminho —
o resultado é sempre "tudo reservado" ou "nada reservado", nunca um estado
intermediário visível.

## Testes

`cmd/servidor/carga_test.go` sobe um servidor real (o mesmo código de
produção, não uma versão simplificada) e:

- dispara dezenas de clientes concorrentes disputando o mesmo trecho de uma
  carona com capacidade limitada, verificando que o número de reservas
  confirmadas nunca excede a capacidade;
- verifica que uma reserva combinando trechos de duas caronas diferentes,
  uma delas sem assento, é recusada por inteiro — sem confirmar a metade
  que tinha assento;
- mede e reporta o tempo de resposta (média, mediana, máximo) sob essa
  carga concorrente.

```
go test ./... -v
```

`go test ./... -race` roda os mesmos testes com o detector de corrida do
Go, que instrumenta o binário para acusar qualquer acesso à memória
compartilhada sem sincronização. Requer um compilador C de 64 bits; não
funciona neste ambiente Windows, mas funciona em Linux sem configuração
adicional.

## Busca de itinerários

O servidor monta um grafo a partir das caronas ativas na data pedida: cada
cidade é um nó, e cada trecho com assento livre é uma aresta, associada à
carona e ao preço daquele trecho. Uma busca em profundidade (DFS), com
retrocesso (*backtracking*) e sem revisitar cidade, encontra os caminhos
possíveis entre origem e destino — inclusive os que atravessam caronas de
motoristas diferentes. Trechos consecutivos da mesma carona são agrupados
num único item de reserva. Os resultados são ordenados por preço total e,
em caso de empate, pelo número de trechos (menos trocas de motorista
primeiro).

## Decisões conscientes e limitações assumidas

- **Login simplificado**: o campo de senha existe só para impedir que dois
  clientes diferentes colidam usando o mesmo nome — quem usa um nome pela
  primeira vez define a senha (em memória); não é um sistema de contas com
  hashing ou persistência.
- **Sem persistência**: todo o estado é perdido se o servidor for
  reiniciado, por decisão consciente (ver acima).
- **Datas em texto**: o campo de data segue o formato `DD/MM/AAAA` e não é
  validado como data real no protocolo em si — a validação de formato e
  calendário acontece nos clientes, antes de enviar.

Outras decisões e suas justificativas estão documentadas junto de cada
operação em `docs/protocolo.md`.

## Como executar

### Nativamente, sem Docker

Em três terminais, a partir da raiz do repositório:

```
go run ./cmd/servidor 8080
go run ./cmd/cliente-motorista 127.0.0.1:8080
go run ./cmd/cliente-passageiro 127.0.0.1:8080
```

### Com Docker, numa máquina só

```
docker compose up --build servidor
docker compose run --build --rm cliente-motorista
docker compose run --build --rm cliente-passageiro
```

### Com Docker, em duas máquinas distintas

Roteiro completo de comandos em
[`docs/comandos-docker-apresentacao.txt`](docs/comandos-docker-apresentacao.txt).
Resumo: a máquina que roda o servidor expõe a porta (`docker run -p
8080:8080 vaijunto-servidor`); as demais rodam os clientes apontando para o
IP real da primeira máquina na rede (`docker run -it vaijunto-cliente-motorista
<IP>:8080`), descoberto com `hostname -I`.

### Testes

```
go test ./... -v
```

## Estrutura do repositório

```
cmd/
  servidor/            servidor central — rede, estado, regras de negócio, busca
  cliente-motorista/    cliente de terminal do motorista
  cliente-passageiro/   cliente de terminal do passageiro
internal/
  dominio/              entidades do problema (Carona, Reserva, ItemReserva)
  protocolo/            formato das mensagens trocadas na rede
docker/                  um Dockerfile por programa
docker-compose.yml       orquestra os três containers numa única máquina
docs/
  protocolo.md           especificação completa do protocolo
  diagrama.pdf           diagrama do problema
  referencias.txt        referências usadas no relatório
  comandos-docker-apresentacao.txt   roteiro de comandos para a apresentação
```

## Requisitos para compilar

Go 1.19 ou superior (o projeto não usa nenhum recurso recente da
linguagem; a exigência baixa existe para funcionar sem ajuste em qualquer
máquina ou imagem Docker disponível). Nenhuma dependência externa — só
biblioteca padrão do Go.
