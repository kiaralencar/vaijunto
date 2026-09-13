// Servidor VaiJunto. Escuta numa porta TCP, aceita várias conexões ao mesmo
// tempo (uma goroutine por cliente) e mantém todo o estado em memória,
// protegido por um mutex.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

// Sessao guarda quem está logado nesta conexão específica. Cada goroutine
// tem a sua, não é compartilhada (por isso não precisa de mutex).
type Sessao struct {
	Nome string
}

func main() {
	porta := "8080"
	if len(os.Args) > 1 {
		porta = os.Args[1]
	}

	// ":8080" sem IP explícito = escuta em todos os endereços desta máquina.
	l, err := net.Listen("tcp", ":"+porta)
	if err != nil {
		fmt.Println("erro ao escutar:", err)
		os.Exit(1)
	}
	defer l.Close()
	fmt.Println("servidor escutando na porta", porta)

	estado := NovoEstado()

	// Accept() bloqueia até alguém conectar.
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("erro ao aceitar conexao:", err)
			continue
		}
		go atendeCliente(conn, estado)
	}
}

// atendeCliente roda numa goroutine própria, uma por conexão. 
func atendeCliente(conn net.Conn, estado *Estado) {
	defer conn.Close()
	sessao := &Sessao{}
	leitor := bufio.NewReader(conn)

	for {
		linha, err := leitor.ReadString('\n')
		if err != nil {
			// Se o cliente desconectou (ou caiu abruptamente), só encerra esta
			// goroutine. O servidor e os outros clientes seguem intactos.
			return
		}

		resposta := processaLinha(linha, estado, sessao)

		saida, err := json.Marshal(resposta)
		if err != nil {
			continue
		}
		conn.Write(append(saida, '\n'))
	}
}
