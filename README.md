# VAIJUNTO — Sistema de Caronas Compartilhadas

## Contexto acadêmico

Este projeto foi desenvolvido no contexto da disciplina MI — Concorrência e
Conectividade, do curso de Engenharia de Computação da Universidade Estadual
de Feira de Santana (UEFS), como trabalho individual dentro de um problema
baseado em aprendizagem (PBL).

## Sobre o projeto

VAIJUNTO é um sistema de coordenação de caronas compartilhadas para viagens
de média e longa distância. Um servidor central mantém o estado de todas as
caronas e reservas; motoristas publicam caronas informando uma rota e um
preço por trecho, e passageiros pesquisam e reservam itinerários entre uma
origem e um destino, sem intermediação humana.

O ponto central do problema que o sistema resolve é que uma carona não é
vendida como um bloco único: como o veículo passa por cidades
intermediárias, cada trecho da rota tem sua própria disponibilidade de
assentos e seu próprio preço. Um passageiro pode embarcar e desembarcar em
qualquer par de cidades da rota, e um itinerário pode inclusive combinar
trechos de caronas de motoristas diferentes (por exemplo, ir de Salvador a
Feira de Santana com um motorista e de Feira de Santana a Vitória da
Conquista com outro). A confirmação desse itinerário precisa ser atômica:
ou todos os trechos envolvidos são reservados, ou nenhum é.

### Requisitos do problema e o que foi implementado

O problema original exige, entre outras coisas: comunicação cliente-servidor
sobre sockets TCP/IP nativos, atendimento simultâneo de múltiplos clientes,
controle de concorrência nas reservas feito pela própria aplicação (sem
delegar a um banco de dados ou serviço externo), atomicidade na confirmação
de itinerários com vários trechos, e controle de disponibilidade de assento
por trecho, não por carona inteira. Todos esses pontos estão implementados
e são descritos nas seções seguintes.

Duas decisões de implementação vão além do que o enunciado exige
literalmente e vale deixar claras:

- **Autenticação por nome e senha** foi adicionada para resolver um
  problema prático — impedir que dois clientes diferentes usem o mesmo
  nome ao mesmo tempo. Não é um sistema de contas: a senha de um nome é
  definida no primeiro uso e existe apenas em memória, sem hashing ou
  persistência.
- **Não há persistência em disco.** Todo o estado (caronas, reservas,
  senhas) vive na memória do processo do servidor e é perdido se ele for
  reiniciado. O enunciado exige apenas que a queda de um *cliente* não
  derrube o servidor nem corrompa o estado — nada é exigido sobre
  sobreviver a um reinício do servidor, e essa simplificação foi assumida
  conscientemente.

## Funcionalidades principais

**Cliente motorista**
- Autenticar-se com nome e senha.
- Publicar uma carona, informando rota (sequência de cidades), data,
  preço e quantidade de assentos por trecho.
- Listar as caronas ativas no sistema.
- Consultar os passageiros confirmados em uma carona própria, por trecho.
- Cancelar uma carona publicada (cancela também, em cascata, as reservas
  de passageiros que dependiam dela).

**Cliente passageiro**
- Autenticar-se com nome e senha.
- Listar as caronas ativas no sistema.
- Buscar itinerários entre uma origem e um destino em uma data, incluindo
  itinerários que combinam trechos de caronas de motoristas diferentes, e
  reservar diretamente um dos resultados encontrados.
- Reservar manualmente, informando o ID de uma carona e um intervalo de
  trechos.
- Listar as próprias reservas ativas.
- Cancelar uma reserva.

## Como funciona

O sistema é formado por três programas Go independentes, dentro de um único
módulo:

- **Servidor** (`cmd/servidor`) — mantém todo o estado em memória e é a
  única parte do sistema que decide o que é ou não permitido. Nenhuma regra
  de negócio roda no cliente.
- **Cliente motorista** e **cliente passageiro** (`cmd/cliente-motorista`,
  `cmd/cliente-passageiro`) — programas de terminal que só sabem falar o
  protocolo da aplicação.

Alguns pontos de funcionamento relevantes:

- Cada cliente conectado é atendido em sua própria goroutine. Todas as
  goroutines compartilham o mesmo estado, protegido por um único
  `sync.Mutex` — toda leitura ou escrita de caronas, reservas ou senhas
  acontece dentro desse lock.
- A confirmação de uma reserva com vários trechos é feita em duas
  passadas sob o mesmo lock: primeiro valida a disponibilidade de todos os
  itens do pedido, depois aplica a mudança em todos eles. Se qualquer item
  falhar na validação, nada é alterado — é essa construção que garante a
  atomicidade exigida pelo problema.
- A busca de itinerários monta um grafo a partir das caronas ativas na
  data pedida (cada cidade é um nó, cada trecho com assento livre é uma
  aresta) e faz uma busca em profundidade entre a origem e o destino,
  permitindo caminhos que atravessam caronas de motoristas diferentes.
- Conexões ociosas por mais de 10 minutos são encerradas pelo servidor.
  A queda de um cliente (abrupta ou por timeout) encerra apenas a
  goroutine daquela conexão; o servidor e as demais conexões continuam
  funcionando normalmente.

## Protocolo de aplicação

A comunicação entre cliente e servidor usa um protocolo de aplicação
próprio, implementado sobre socket TCP puro (pacote `net` da biblioteca
padrão do Go), sem HTTP e sem framework de mensageria ou RPC. Cada mensagem
é um objeto JSON compacto terminado em `\n`, trocado numa conexão que
permanece aberta durante toda a sessão do cliente.

A especificação completa — formato do envelope, as operações disponíveis,
o fluxo de conexão e autenticação, e exemplos concretos de mensagens — está
em [`docs/protocolo.md`](docs/protocolo.md).

## Tecnologias utilizadas

- **Go** (módulo `vaijunto`, `go 1.19` no `go.mod`) — sem dependências
  externas, apenas biblioteca padrão (`net`, `encoding/json`, `sync`,
  `bufio`, `time`, entre outras).
- **Docker** e **Docker Compose** — para empacotar e emular a execução do
  servidor e dos clientes em máquinas distintas.

## Estrutura do projeto

```
cmd/
  servidor/
    main.go            ponto de entrada, aceitação de conexões, ciclo de vida de cada cliente
    estado.go          estado do servidor (caronas, reservas, senhas) e suas operações
    handlers.go        despacho e validação de cada tipo de pedido do protocolo
    busca.go           busca de itinerários por grafo
    carga_test.go      teste automatizado de concorrência e atomicidade
  cliente-motorista/
    main.go            cliente de terminal do motorista
  cliente-passageiro/
    main.go            cliente de terminal do passageiro
internal/
  dominio/
    dominio.go         entidades do problema (Carona, Reserva, ItemReserva)
  protocolo/
    protocolo.go       formato das mensagens trocadas entre cliente e servidor
docker/
  servidor.Dockerfile
  cliente-motorista.Dockerfile
  cliente-passageiro.Dockerfile
docker-compose.yml     orquestra os três containers numa única máquina
docs/
  protocolo.md         especificação completa do protocolo
  comandos-docker.txt  roteiro de comandos para execução em Docker
  diagrama.pdf         diagrama do problema
  referencias.txt      referências utilizadas no relatório
Makefile               atalhos para build, testes e execução
go.mod
```

## Pré-requisitos

- Go 1.19 ou superior, para execução nativa ou para rodar os testes.
- Docker e Docker Compose, para execução em containers (opcional, mas é a
  forma usada para emular a comunicação entre máquinas distintas).

O projeto não tem dependências externas de terceiros — não é necessário
rodar `go mod download` ou similar.

## Instalação

```
git clone https://github.com/kiaralencar/vaijunto.git
cd vaijunto
```

Não há passo de configuração adicional: não existem variáveis de ambiente
nem arquivos de configuração — porta e endereço são passados como argumento
de linha de comando, conforme as seções abaixo.

## Execução nativa

O servidor precisa estar rodando antes dos clientes se conectarem. Em três
terminais separados, a partir da raiz do repositório:

| Programa | Comando | Argumento |
|---|---|---|
| Servidor | `go run ./cmd/servidor [porta]` | opcional, padrão `8080` |
| Cliente motorista | `go run ./cmd/cliente-motorista <host:porta>` | obrigatório |
| Cliente passageiro | `go run ./cmd/cliente-passageiro <host:porta>` | obrigatório |

Exemplo, tudo na mesma máquina:

```
go run ./cmd/servidor 8080
go run ./cmd/cliente-motorista 127.0.0.1:8080
go run ./cmd/cliente-passageiro 127.0.0.1:8080
```

O `Makefile` oferece os mesmos alvos como atalho (`make run-servidor`,
`make run-motorista`, `make run-passageiro`, com `PORTA` e `ENDERECO` como
variáveis opcionais); `make help` lista todos os alvos disponíveis.

## Execução com Docker

Cada programa tem seu próprio Dockerfile em `docker/`, e as três imagens
são independentes — alterar o código exige rebuildar a imagem
correspondente antes de rodar novamente.

### Numa máquina só (verificação rápida)

```
docker compose up --build servidor
```

Em outros terminais:

```
docker compose run --build --rm cliente-motorista
docker compose run --build --rm cliente-passageiro
```

O `docker-compose.yml` já configura os clientes para se conectarem ao
serviço `servidor` pela rede interna do Compose.

### Em máquinas distintas (emulação de rede real)

Na máquina que vai rodar o servidor:

```
docker build -f docker/servidor.Dockerfile -t vaijunto-servidor .
docker run -p 8080:8080 vaijunto-servidor
```

Em outro terminal da mesma máquina, para descobrir o IP a ser usado pelas
demais:

```
hostname -I
```

Nas máquinas que vão rodar os clientes, substituindo `<IP-DO-SERVIDOR>`
pelo IP obtido acima:

```
docker build -f docker/cliente-motorista.Dockerfile -t vaijunto-cliente-motorista .
docker run -it --rm vaijunto-cliente-motorista <IP-DO-SERVIDOR>:8080
```

```
docker build -f docker/cliente-passageiro.Dockerfile -t vaijunto-cliente-passageiro .
docker run -it --rm vaijunto-cliente-passageiro <IP-DO-SERVIDOR>:8080
```

Um roteiro mais detalhado de comandos, incluindo solução de problemas comuns
de rede e permissão, está em
[`docs/comandos-docker.txt`](docs/comandos-docker.txt).

## Manual de uso

Depois de conectado, cada cliente pede nome e senha. Se o nome ainda não
foi usado, a senha informada passa a valer para ele; se já existe, a senha
precisa bater com a que foi usada da primeira vez.

Fluxo típico, publicando uma carona e reservando um trecho dela:

**No cliente motorista**, publicar uma carona (opção 1 do menu):

```
Rota: Salvador, Feira de Santana, Vitoria da Conquista
Data (formato DD/MM/AAAA): 20/09/2026

Trecho 0: Salvador -> Feira de Santana
Valor: 40
Quantidade de assentos: 3

Trecho 1: Feira de Santana -> Vitoria da Conquista
Valor: 60
Quantidade de assentos: 2
```

O servidor responde com o ID da carona criada.

**No cliente passageiro**, buscar um itinerário e reservar (opção 2 do
menu):

```
Origem: Salvador
Destino: Vitoria da Conquista
Data (formato DD/MM/AAAA): 20/09/2026
```

A busca retorna a lista de itinerários encontrados, com preço total e os
trechos que os compõem; escolher um pelo número o reserva imediatamente.
Também é possível reservar manualmente (opção 3), informando diretamente o
ID de uma carona e o intervalo de trechos desejado — útil quando o
passageiro já sabe qual carona quer usar.

O motorista pode consultar quem reservou sua carona (opção 3 do menu
motorista, informando o ID da carona) e o passageiro pode listar e
cancelar suas próprias reservas (opções 4 e 5 do menu do passageiro).
Digitar `0` em qualquer menu volta à tela de login, sem encerrar o
programa.

## Testes

O arquivo `cmd/servidor/carga_test.go` sobe uma instância real do servidor
(o mesmo código usado em produção) e verifica:

- que múltiplos clientes concorrentes disputando os mesmos trechos de uma
  carona com capacidade limitada nunca resultam em mais reservas
  confirmadas do que assentos disponíveis;
- que uma reserva combinando trechos de duas caronas diferentes, quando
  uma delas não tem assento disponível, é recusada por inteiro, sem
  confirmar parcialmente a outra;
- o tempo de resposta do servidor sob essa carga concorrente (média,
  mediana e máximo).

Para rodar:

```
go test ./... -v
```

Com o detector de corrida do Go (exige compilador C de 64 bits; funciona
sem configuração adicional na maioria dos ambientes Linux):

```
go test ./... -race -v
```

Os mesmos comandos estão disponíveis como `make test` e `make test-race`.
