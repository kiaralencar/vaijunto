// Cliente passageiro: conecta no servidor, faz login, e permite listar
// caronas, reservar um itinerário (um ou mais trechos, de uma ou mais
// caronas), listar e cancelar suas reservas.
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
		fmt.Println("uso: cliente-passageiro <host:porta>")
		return
	}

	conn, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		fmt.Println("erro ao conectar:", err)
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
			fmt.Println("conexao com o servidor caiu:", err)
			os.Exit(1)
		}
		var resp protocolo.Resposta
		json.Unmarshal([]byte(respLinha), &resp)
		return resp
	}

	fmt.Print("seu nome: ")
	nome, _ := leitorTeclado.ReadString('\n')
	nome = strings.TrimSpace(nome)

	resp := enviar(protocolo.Login, protocolo.LoginReq{Nome: nome})
	if !resp.Ok {
		fmt.Println("erro no login:", resp.Erro)
		return
	}
	fmt.Println("login efetuado como", nome)

	for {
		fmt.Println("\n1) listar caronas  2) buscar itinerarios e reservar  3) reservar manualmente  4) minhas reservas  5) cancelar reserva  0) sair")
		fmt.Print("> ")
		opcao, _ := leitorTeclado.ReadString('\n')
		switch strings.TrimSpace(opcao) {
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
			fmt.Println("opcao invalida")
		}
	}
}

func lerLinha(leitor *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	linha, _ := leitor.ReadString('\n')
	return strings.TrimSpace(linha)
}

func listarCaronas(enviar func(string, interface{}) protocolo.Resposta) {
	resp := enviar(protocolo.ListarCaronas, struct{}{})
	if !resp.Ok {
		fmt.Println("erro:", resp.Erro)
		return
	}
	var dados protocolo.ListarCaronasResp
	json.Unmarshal(resp.Dados, &dados)
	if len(dados.Caronas) == 0 {
		fmt.Println("nenhuma carona publicada ainda")
		return
	}
	for _, c := range dados.Caronas {
		fmt.Printf("id=%s motorista=%s rota=%v data=%s assentos_livres=%v preco_por_trecho=%v\n",
			c.ID, c.Motorista, c.Rota, c.Data, c.AssentosLivres, c.PrecoPorTrecho)
		for i := 0; i < len(c.Rota)-1; i++ {
			fmt.Printf("    trecho %d: %s -> %s\n", i, c.Rota[i], c.Rota[i+1])
		}
	}
}

func buscarEReservar(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	origem := lerLinha(leitor, "origem: ")
	destino := lerLinha(leitor, "destino: ")
	data := lerLinha(leitor, "data (ex: 2026-09-20): ")

	resp := enviar(protocolo.BuscarItinerarios, protocolo.BuscarItinerariosReq{
		Origem: origem, Destino: destino, Data: data,
	})
	if !resp.Ok {
		fmt.Println("erro:", resp.Erro)
		return
	}
	var dados protocolo.BuscarItinerariosResp
	json.Unmarshal(resp.Dados, &dados)
	if len(dados.Itinerarios) == 0 {
		fmt.Println("nenhum itinerario encontrado para essa origem/destino/data")
		return
	}

	for i, it := range dados.Itinerarios {
		fmt.Printf("[%d] preco total=%.2f itens=%v\n", i, it.Preco, it.Itens)
	}

	escolha := lerLinha(leitor, "escolha o itinerario pelo numero (vazio para nao reservar): ")
	if escolha == "" {
		return
	}
	idx, err := strconv.Atoi(escolha)
	if err != nil || idx < 0 || idx >= len(dados.Itinerarios) {
		fmt.Println("numero invalido")
		return
	}

	respReserva := enviar(protocolo.Reservar, protocolo.ReservarReq{Itens: dados.Itinerarios[idx].Itens})
	if !respReserva.Ok {
		fmt.Println("reserva recusada:", respReserva.Erro)
		return
	}
	var reservaFeita protocolo.ReservarResp
	json.Unmarshal(respReserva.Dados, &reservaFeita)
	fmt.Println("reserva confirmada, id:", reservaFeita.ReservaID)
}

// reservarManual deixa o passageiro adicionar quantos itens quiser antes de confirmar
func reservarManual(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	var itens []protocolo.ItemPedido
	for {
		id := lerLinha(leitor, "id da carona (vazio para terminar de adicionar trechos): ")
		if id == "" {
			break
		}
		inicio, _ := strconv.Atoi(lerLinha(leitor, "  trecho inicial (0-based): "))
		fim, _ := strconv.Atoi(lerLinha(leitor, "  trecho final (0-based, igual ao inicial se for so um trecho): "))
		itens = append(itens, protocolo.ItemPedido{CaronaID: id, TrechoInicio: inicio, TrechoFim: fim})
	}
	if len(itens) == 0 {
		fmt.Println("nada para reservar")
		return
	}

	resp := enviar(protocolo.Reservar, protocolo.ReservarReq{Itens: itens})
	if !resp.Ok {
		fmt.Println("reserva recusada:", resp.Erro)
		return
	}
	var dados protocolo.ReservarResp
	json.Unmarshal(resp.Dados, &dados)
	fmt.Println("reserva confirmada, id:", dados.ReservaID)
}

func listarMinhasReservas(enviar func(string, interface{}) protocolo.Resposta) {
	resp := enviar(protocolo.ListarMinhasReservas, struct{}{})
	if !resp.Ok {
		fmt.Println("erro:", resp.Erro)
		return
	}
	var dados protocolo.ListarMinhasReservasResp
	json.Unmarshal(resp.Dados, &dados)
	if len(dados.Reservas) == 0 {
		fmt.Println("voce nao tem reservas ativas")
		return
	}
	for _, r := range dados.Reservas {
		fmt.Printf("reserva=%s itens=%v\n", r.ID, r.Itens)
	}
}

func cancelarReserva(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	id := lerLinha(leitor, "id da reserva: ")
	resp := enviar(protocolo.CancelarReserva, protocolo.CancelarReservaReq{ReservaID: id})
	if !resp.Ok {
		fmt.Println("erro:", resp.Erro)
		return
	}
	fmt.Println("reserva cancelada")
}
