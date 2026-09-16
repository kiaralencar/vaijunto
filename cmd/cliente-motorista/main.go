// Cliente motorista: conecta no servidor, faz login, e permite publicar
// carona, listar caronas, consultar passageiros de uma carona e cancelar.
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
		fmt.Println("Uso: cliente-motorista <host:porta>")
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

		menuMotorista(leitorTeclado, enviar)
		fmt.Println("\n(Sessao encerrada — para fechar o programa de vez, use Ctrl+C)")
	}
}

func menuMotorista(leitorTeclado *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	for {
		fmt.Println("\n[1] Publicar carona " +
			"\n[2] Listar caronas" +
			"\n[3] Consultar passageiros " +
			"\n[4] Cancelar carona" +
			"\n[0] Voltar ao menu inicial")
		opcao := lerLinha(leitorTeclado, "> ")
		switch opcao {
		case "1":
			publicarCarona(leitorTeclado, enviar)
		case "2":
			listarCaronas(enviar)
		case "3":
			consultarPassageiros(leitorTeclado, enviar)
		case "4":
			cancelarCarona(leitorTeclado, enviar)
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

func publicarCarona(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	rotaTexto := lerLinha(leitor, "Rota:\nOBS.: Escreva as cidades separadas por virgula, "+
		"em ordem. Ex: Salvador, Vitoria da Conquista): ")
	partes := strings.Split(rotaTexto, ",")
	rota := make([]string, 0, len(partes))
	for _, p := range partes {
		p = strings.TrimSpace(p)
		if p != "" {
			rota = append(rota, p)
		}
	}
	if len(rota) < 2 {
		fmt.Println("A rota precisa de pelo menos duas cidades")
		return
	}
	numTrechos := len(rota) - 1

	data := lerLinha(leitor, "Data (formato DD/MM/AAAA): ")

	precos := make([]float64, numTrechos)
	assentos := make([]int, numTrechos)
	for i := 0; i < numTrechos; i++ {
		fmt.Printf("Trecho %d: %s -> %s\n", i, rota[i], rota[i+1])
		precos[i], _ = strconv.ParseFloat(lerLinha(leitor, "Valor: "), 64)
		assentos[i], _ = strconv.Atoi(lerLinha(leitor, "Quantidade de assentos: "))
	}

	resp := enviar(protocolo.PublicarCarona, protocolo.PublicarCaronaReq{
		Rota: rota, Data: data, PrecoPorTrecho: precos, AssentosPorTrecho: assentos,
	})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	var dados protocolo.PublicarCaronaResp
	json.Unmarshal(resp.Dados, &dados)
	fmt.Println("Carona publicada. ID: ", dados.CaronaID)
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
		fmt.Println("Nenhuma carona publicada aind.a")
		return
	}
	for _, c := range dados.Caronas {
		fmt.Printf("\nID: %s\nMotorista: %s\nRota: %v\nData: %s\nAssentos livres: %v\nPreco por trecho: %v\n",
			c.ID, c.Motorista, c.Rota, c.Data, c.AssentosLivres, c.PrecoPorTrecho)
	}
}

func consultarPassageiros(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	id := lerLinha(leitor, "ID da carona: ")
	resp := enviar(protocolo.ConsultarPassageiros, protocolo.ConsultarPassageirosReq{CaronaID: id})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	var dados protocolo.ConsultarPassageirosResp
	json.Unmarshal(resp.Dados, &dados)
	if len(dados.Passageiros) == 0 {
		fmt.Println("Nenhum passageiro nesta carona ainda.")
		return
	}
	for _, p := range dados.Passageiros {
		fmt.Printf("Passageiro %s do trecho %d ao %d\n", p.Passageiro, p.TrechoInicio, p.TrechoFim)
	}
}

func cancelarCarona(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	id := lerLinha(leitor, "ID da carona: ")
	resp := enviar(protocolo.CancelarCarona, protocolo.CancelarCaronaReq{CaronaID: id})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	fmt.Println("Carona cancelada")
}
