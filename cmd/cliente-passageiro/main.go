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
	"time"

	"vaijunto/internal/protocolo"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: cliente-passageiro <host:porta>")
		return
	}

	conn, err := net.Dial("tcp", os.Args[1]) // Única chamada que já disca e conecta
	if err != nil {
		fmt.Println("Erro ao conectar:", err)
		return
	}
	defer conn.Close()

	leitorServidor := bufio.NewReader(conn) // Lê da rede
	leitorTeclado := bufio.NewReader(os.Stdin) // Lê do teclado

	// Envia um pedido ao servidor e espera a resposta.
	enviar := func(tipo string, dados interface{}) protocolo.Resposta {
		corpo, err := json.Marshal(dados)
		if err != nil {
			fmt.Println("Erro interno ao montar o pedido:", err)
			os.Exit(1)
		}
		pedido := protocolo.Pedido{Tipo: tipo, Dados: corpo}
		linha, err := json.Marshal(pedido)
		if err != nil {
			fmt.Println("Erro interno ao montar o pedido:", err)
			os.Exit(1)
		}
		if _, err := conn.Write(append(linha, '\n')); err != nil {
			fmt.Println("Conexao com o servidor caiu ao enviar:", err)
			os.Exit(1)
		}

		respLinha, err := leitorServidor.ReadString('\n')
		if err != nil {
			fmt.Println("Conexao com o servidor caiu:", err)
			os.Exit(1)
		}
		var resp protocolo.Resposta
		if err := json.Unmarshal([]byte(respLinha), &resp); err != nil {
			fmt.Println("Resposta invalida do servidor:", err)
			os.Exit(1)
		}
		return resp
	}

	// Loop principal: pede login, e depois entra no menu do passageiro.
	for {
		nome := lerLinha(leitorTeclado, "\nNome: ")
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

// Exibe as opções para o passageiro, e chama a função correspondente à opção escolhida.
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

// lerLinha lê uma linha do teclado, mostrando o prompt, e devolve a linha sem espaços
func lerLinha(leitor *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	linha, err := leitor.ReadString('\n')
	if err != nil {
		fmt.Println("\nEntrada encerrada, fechando o cliente.")
		os.Exit(0)
	}
	return strings.TrimSpace(linha)
}

// lerData insiste no mesmo campo até receber uma data real, exatamente no
// formato DD/MM/AAAA (dia/mês/ano válidos de verdade, não só o formato).
func lerData(leitor *bufio.Reader, prompt string) string {
	for {
		data := lerLinha(leitor, prompt)
		if _, err := time.Parse("02/01/2006", data); err != nil {
			fmt.Println("Data invalida! Use exatamente o formato DD/MM/AAAA (ex.: 20/09/2026).")
			continue
		}
		return data
	}
}

// lerInteiro insiste no mesmo campo até receber um número inteiro de verdade
func lerInteiro(leitor *bufio.Reader, prompt string) int {
	for {
		texto := lerLinha(leitor, prompt)
		valor, err := strconv.Atoi(texto)
		if err != nil {
			fmt.Println("Valor invalido! Digite um numero inteiro.")
			continue
		}
		return valor
	}
}

// Tenta decodificar os dados que vieram na resposta do servidor
// na struct esperada. Se a resposta vier corrompida ou num formato
// inesperado, avisa e devolve false em vez de seguir com dados incompletos.
func decodificar(dados []byte, v interface{}) bool {
	if err := json.Unmarshal(dados, v); err != nil {
		fmt.Println("Resposta invalida do servidor:", err)
		return false
	}
	return true
}

// Exibe a lista de caronas publicadas no servidor.
func listarCaronas(enviar func(string, interface{}) protocolo.Resposta) {
	resp := enviar(protocolo.ListarCaronas, struct{}{})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	var dados protocolo.ListarCaronasResp
	if !decodificar(resp.Dados, &dados) {
		return
	}
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

// Permite ao passageiro buscar itinerários e reservar um deles.
func buscarEReservar(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	origem := lerLinha(leitor, "Origem: ")
	destino := lerLinha(leitor, "Destino: ")
	data := lerData(leitor, "Data (formato DD/MM/AAAA): ")

	resp := enviar(protocolo.BuscarItinerarios, protocolo.BuscarItinerariosReq{
		Origem: origem, Destino: destino, Data: data,
	})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	var dados protocolo.BuscarItinerariosResp
	if !decodificar(resp.Dados, &dados) {
		return
	}
	if len(dados.Itinerarios) == 0 {
		fmt.Println("Nenhum itinerário encontrado para essa origem/destino/data.")
		return
	}

	// Exibe os itinerários encontrados, com seus trechos e preços.
	for i, it := range dados.Itinerarios {
		fmt.Printf("[%d] Preco total = %.2f\n", i, it.Preco)
		for _, item := range it.Itens {
			fmt.Printf("    %s -> %s (Carona %s | Trechos %d-%d)\n",
				item.Origem, item.Destino, item.CaronaID, item.TrechoInicio, item.TrechoFim)
		}
	}

	// Passageiro escolhe um itinerário para reservar ou cancela a operação.
	var idx int
	for {
		escolha := lerLinha(leitor, "Escolha o itinerario pelo numero (Enter para nao reservar): ")
		if escolha == "" {
			return
		}
		valor, err := strconv.Atoi(escolha)
		if err != nil || valor < 0 || valor >= len(dados.Itinerarios) {
			fmt.Println("Numero invalido! Escolha um dos itinerarios listados acima.")
			continue
		}
		idx = valor
		break
	}

	respReserva := enviar(protocolo.Reservar, protocolo.ReservarReq{Itens: dados.Itinerarios[idx].Itens})
	if !respReserva.Ok {
		fmt.Println("Reserva recusada: ", respReserva.Erro) // Erro em caso de carona cheia, trecho inválido, etc.
		return
	}
	var reservaFeita protocolo.ReservarResp
	if !decodificar(respReserva.Dados, &reservaFeita) {
		return
	}
	fmt.Println("Reserva confirmada. ID:", reservaFeita.ReservaID)
}

// Permite ao passageiro reservar manualmente, informando o ID da carona e os trechos.
func reservarManual(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	var itens []protocolo.ItemPedido
	for {
		id := lerLinha(leitor, "ID da carona (Enter para finalizar): ")
		if id == "" {
			break
		}

		// O passageiro informa o trecho inicial e final que deseja reservar. 
		// Se for apenas um trecho, o final é igual ao inicial
		inicio := lerInteiro(leitor, "Trecho inicial: ")
		fim := lerInteiro(leitor, "Trecho final (igual ao inicial se for apenas um trecho): ")
		itens = append(itens, protocolo.ItemPedido{CaronaID: id, TrechoInicio: inicio, TrechoFim: fim})
	}
	if len(itens) == 0 {
		fmt.Println("Nada para reservar.")
		return
	}

	resp := enviar(protocolo.Reservar, protocolo.ReservarReq{Itens: itens})
	if !resp.Ok {
		fmt.Println("Reserva recusada: ", resp.Erro) // Erro em caso de carona cheia, trecho inválido, etc.
		return
	}
	var dados protocolo.ReservarResp
	if !decodificar(resp.Dados, &dados) {
		return
	}
	fmt.Println("Reserva confirmada. ID:", dados.ReservaID)
}

// Permite ao passageiro listar suas reservas ativas.
func listarMinhasReservas(enviar func(string, interface{}) protocolo.Resposta) {
	resp := enviar(protocolo.ListarMinhasReservas, struct{}{})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	var dados protocolo.ListarMinhasReservasResp
	if !decodificar(resp.Dados, &dados) {
		return
	}
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

// Permite ao passageiro cancelar uma reserva.
func cancelarReserva(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	id := lerLinha(leitor, "ID da reserva: ")
	resp := enviar(protocolo.CancelarReserva, protocolo.CancelarReservaReq{ReservaID: id})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	fmt.Println("Reserva cancelada.")
}
