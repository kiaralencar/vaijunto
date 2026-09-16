// Cliente passageiro: conecta no servidor, faz login, e permite listar
// caronas, reservar um itinerário, listar e cancelar suas reservas.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"vaijunto/internal/protocolo"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: cliente-passageiro <host:porta>")
		return
	}

	conn, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		fmt.Println("Erro ao conectar:", err)
		return
	}
	defer conn.Close()

	leitorServidor := bufio.NewReader(conn)
	leitorTeclado := bufio.NewReader(os.Stdin)

	enviar := func(tipo string, dados interface{}) protocolo.Resposta {
		corpo, _ := json.Marshal(dados)
		pedido := protocolo.Pedido{Tipo: tipo, Dados: corpo}
		linha, _ := json.Marshal(pedido)
		conn.Write(append(linha, '\n'))

		respLinha, err := leitorServidor.ReadString('\n')
		if err != nil {
			fmt.Println("Conexao com o servidor caiu:", err)
			os.Exit(1)
		}
		var resp protocolo.Resposta
		json.Unmarshal([]byte(respLinha), &resp)
		return resp
	}

	for {
		nome := lerLinha(leitorTeclado, "Nome: ")
		senha := lerLinha(leitorTeclado, "Senha: ")

		resp := enviar(protocolo.Login, protocolo.LoginReq{Nome: nome, Senha: senha})
		if !resp.Ok {
			fmt.Println("Erro no login:", resp.Erro)
			continue
		}
		fmt.Println("Login efetuado como", nome)

		menuPassageiro(leitorTeclado, enviar)
		fmt.Println("\n(Sessao encerrada — para fechar o programa de vez, use Ctrl+C)")
	}
}

func menuPassageiro(leitorTeclado *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	for {
		fmt.Println("\n[1] Listar caronas" +
			"\n[2] Buscar itinerarios e reservar" +
			"\n[3] Reservar manualmente" +
			"\n[4] Minhas reservas" +
			"\n[5] Cancelar reserva" +
			"\n[0] Voltar ao menu inicial")
		opcao := lerLinha(leitorTeclado, "> ")
		switch opcao {
		case "1":
			listarCaronas(enviar)
		case "2":
			buscarEReservar(leitorTeclado, enviar)
		case "3":
			reservarManual(leitorTeclado, enviar)
		case "4":
			listarMinhasReservas(enviar)
		case "5":
			cancelarReserva(leitorTeclado, enviar)
		case "0":
			return
		default:
			fmt.Println("Opcao invalida!\nTente novamente.")
		}
	}
}

func lerLinha(leitor *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	linha, err := leitor.ReadString('\n')
	if err != nil {
		fmt.Println("\nEntrada encerrada, fechando o cliente.")
		os.Exit(0)
	}
	return strings.TrimSpace(linha)
}

func listarCaronas(enviar func(string, interface{}) protocolo.Resposta) {
	resp := enviar(protocolo.ListarCaronas, struct{}{})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	var dados protocolo.ListarCaronasResp
	json.Unmarshal(resp.Dados, &dados)
	if len(dados.Caronas) == 0 {
		fmt.Println("Nenhuma carona publicada ainda.")
		return
	}
	for _, c := range dados.Caronas {
		fmt.Printf("\nID: %s\nMotorista: %s\nRota: %v\nData: %s\nAssentos livres: %v\nPreco por trecho: %v\n",
			c.ID, c.Motorista, c.Rota, c.Data, c.AssentosLivres, c.PrecoPorTrecho)
		for i := 0; i < len(c.Rota)-1; i++ {
			fmt.Printf("Trecho %d: %s -> %s\n", i, c.Rota[i], c.Rota[i+1])
		}
	}
}

func buscarEReservar(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	origem := lerLinha(leitor, "Origem: ")
	destino := lerLinha(leitor, "Destino: ")
	data := lerLinha(leitor, "Data (formato DD/MM/AAAA): ")

	resp := enviar(protocolo.BuscarItinerarios, protocolo.BuscarItinerariosReq{
		Origem: origem, Destino: destino, Data: data,
	})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	var dados protocolo.BuscarItinerariosResp
	json.Unmarshal(resp.Dados, &dados)
	if len(dados.Itinerarios) == 0 {
		fmt.Println("Nenhum itinerário encontrado para essa origem/destino/data.")
		return
	}

	for i, it := range dados.Itinerarios {
		fmt.Printf("[%d] preco total=%.2f\n", i, it.Preco)
		for _, item := range it.Itens {
			fmt.Printf("     %s -> %s (carona %s, trechos %d-%d)\n",
				item.Origem, item.Destino, item.CaronaID, item.TrechoInicio, item.TrechoFim)
		}
	}

	escolha := lerLinha(leitor, "Escolha o itinerario pelo numero (Enter para nao reservar): ")
	if escolha == "" {
		return
	}
	idx, err := strconv.Atoi(escolha)
	if err != nil || idx < 0 || idx >= len(dados.Itinerarios) {
		fmt.Println("Numero invalido!")
		return
	}

	respReserva := enviar(protocolo.Reservar, protocolo.ReservarReq{Itens: dados.Itinerarios[idx].Itens})
	if !respReserva.Ok {
		fmt.Println("Reserva recusada: ", respReserva.Erro)
		return
	}
	var reservaFeita protocolo.ReservarResp
	json.Unmarshal(respReserva.Dados, &reservaFeita)
	fmt.Println("Reserva confirmada. ID: ", reservaFeita.ReservaID)
}

func reservarManual(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	var itens []protocolo.ItemPedido
	for {
		id := lerLinha(leitor, "ID da carona (Enter para finalizar): ")
		if id == "" {
			break
		}
		inicio, _ := strconv.Atoi(lerLinha(leitor, "Trecho inicial: "))
		fim, _ := strconv.Atoi(lerLinha(leitor, "Trecho final (igual ao inicial se for apenas um trecho): "))
		itens = append(itens, protocolo.ItemPedido{CaronaID: id, TrechoInicio: inicio, TrechoFim: fim})
	}
	if len(itens) == 0 {
		fmt.Println("Nada para reservar.")
		return
	}

	resp := enviar(protocolo.Reservar, protocolo.ReservarReq{Itens: itens})
	if !resp.Ok {
		fmt.Println("Reserva recusada: ", resp.Erro)
		return
	}
	var dados protocolo.ReservarResp
	json.Unmarshal(resp.Dados, &dados)
	fmt.Println("Reserva confirmada. ID: ", dados.ReservaID)
}

func listarMinhasReservas(enviar func(string, interface{}) protocolo.Resposta) {
	resp := enviar(protocolo.ListarMinhasReservas, struct{}{})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	var dados protocolo.ListarMinhasReservasResp
	json.Unmarshal(resp.Dados, &dados)
	if len(dados.Reservas) == 0 {
		fmt.Println("Voce nao tem reservas ativas.")
		return
	}
	for _, r := range dados.Reservas {
		fmt.Printf("reserva=%s\n", r.ID)
		for _, item := range r.Itens {
			fmt.Printf("   %s -> %s (carona %s, trechos %d-%d, R$ %.2f)\n",
				item.Origem, item.Destino, item.CaronaID, item.TrechoInicio, item.TrechoFim, item.Preco)
		}
	}
}

func cancelarReserva(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	id := lerLinha(leitor, "ID da reserva: ")
	resp := enviar(protocolo.CancelarReserva, protocolo.CancelarReservaReq{ReservaID: id})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	fmt.Println("Reserva cancelada.")
}
