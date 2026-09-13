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
		fmt.Println("uso: cliente-motorista <host:porta>")
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
		fmt.Println("\n1) publicar carona  2) listar caronas  3) consultar passageiros  4) cancelar carona  0) sair")
		fmt.Print("> ")
		opcao, _ := leitorTeclado.ReadString('\n')
		switch strings.TrimSpace(opcao) {
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
			fmt.Println("opcao invalida")
		}
	}
}

func lerLinha(leitor *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	linha, _ := leitor.ReadString('\n')
	return strings.TrimSpace(linha)
}

func publicarCarona(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	rotaTexto := lerLinha(leitor, "rota (cidades separadas por virgula, em ordem, ex: Salvador,Vitoria da Conquista): ")
	partes := strings.Split(rotaTexto, ",")
	rota := make([]string, 0, len(partes))
	for _, p := range partes {
		p = strings.TrimSpace(p)
		if p != "" {
			rota = append(rota, p)
		}
	}
	if len(rota) < 2 {
		fmt.Println("rota precisa de pelo menos duas cidades")
		return
	}
	numTrechos := len(rota) - 1

	data := lerLinha(leitor, "data (ex: 2026-09-20): ")

	precos := make([]float64, numTrechos)
	assentos := make([]int, numTrechos)
	for i := 0; i < numTrechos; i++ {
		fmt.Printf("trecho %d: %s -> %s\n", i, rota[i], rota[i+1])
		precos[i], _ = strconv.ParseFloat(lerLinha(leitor, "  preco: "), 64)
		assentos[i], _ = strconv.Atoi(lerLinha(leitor, "  assentos: "))
	}

	resp := enviar(protocolo.PublicarCarona, protocolo.PublicarCaronaReq{
		Rota: rota, Data: data, PrecoPorTrecho: precos, AssentosPorTrecho: assentos,
	})
	if !resp.Ok {
		fmt.Println("erro:", resp.Erro)
		return
	}
	var dados protocolo.PublicarCaronaResp
	json.Unmarshal(resp.Dados, &dados)
	fmt.Println("carona publicada, id:", dados.CaronaID)
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
	}
}

func consultarPassageiros(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	id := lerLinha(leitor, "id da carona: ")
	resp := enviar(protocolo.ConsultarPassageiros, protocolo.ConsultarPassageirosReq{CaronaID: id})
	if !resp.Ok {
		fmt.Println("erro:", resp.Erro)
		return
	}
	var dados protocolo.ConsultarPassageirosResp
	json.Unmarshal(resp.Dados, &dados)
	if len(dados.Passageiros) == 0 {
		fmt.Println("nenhum passageiro nesta carona ainda")
		return
	}
	for _, p := range dados.Passageiros {
		fmt.Printf("passageiro=%s do trecho %d ao %d\n", p.Passageiro, p.TrechoInicio, p.TrechoFim)
	}
}

func cancelarCarona(leitor *bufio.Reader, enviar func(string, interface{}) protocolo.Resposta) {
	id := lerLinha(leitor, "id da carona: ")
	resp := enviar(protocolo.CancelarCarona, protocolo.CancelarCaronaReq{CaronaID: id})
	if !resp.Ok {
		fmt.Println("erro:", resp.Erro)
		return
	}
	fmt.Println("carona cancelada")
}
