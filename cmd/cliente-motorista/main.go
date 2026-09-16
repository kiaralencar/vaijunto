// Cliente motorista: conecta no servidor, faz login, e permite publicar
// carona, listar caronas, consultar passageiros de uma carona e cancelar.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

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

// lerData insiste no mesmo campo até receber uma data real, no formato DD/MM/AAAA
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

// lerValor insiste no mesmo campo até receber um número de verdade (inteiro ou com casas decimais)
func lerValor(leitor *bufio.Reader, prompt string) float64 {
	for {
		texto := lerLinha(leitor, prompt)
		valor, err := strconv.ParseFloat(texto, 64)
		if err != nil || math.IsNaN(valor) || math.IsInf(valor, 0) {
			fmt.Println("Valor invalido! Digite um numero (ex.: 12 ou 12.60).")
			continue
		}
		return valor
	}
}

// lerInteiro é o mesmo princípio, para campos que precisam de um inteiro
func lerInteiro(leitor *bufio.Reader, prompt string) int {
	for {
		texto := lerLinha(leitor, prompt)
		valor, err := strconv.Atoi(texto)
		if err != nil {
			fmt.Println("Valor invalido! Digite um numero inteiro (ex.: 2).")
			continue
		}
		return valor
	}
}

// decodificar tenta decodificar os dados que vieram na resposta do servidor
// na struct esperada. Se a resposta vier corrompida ou num formato
// inesperado, avisa e devolve false em vez de seguir com dados incompletos.
func decodificar(dados []byte, v interface{}) bool {
	if err := json.Unmarshal(dados, v); err != nil {
		fmt.Println("Resposta invalida do servidor:", err)
		return false
	}
	return true
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

	data := lerData(leitor, "Data (formato DD/MM/AAAA): ")

	precos := make([]float64, numTrechos)
	assentos := make([]int, numTrechos)
	for i := 0; i < numTrechos; i++ {
		fmt.Printf("\nTrecho %d: %s -> %s\n", i, rota[i], rota[i+1])
		precos[i] = lerValor(leitor, "Valor: ")
		assentos[i] = lerInteiro(leitor, "Quantidade de assentos: ")
	}

	resp := enviar(protocolo.PublicarCarona, protocolo.PublicarCaronaReq{
		Rota: rota, Data: data, PrecoPorTrecho: precos, AssentosPorTrecho: assentos,
	})
	if !resp.Ok {
		fmt.Println("Erro:", resp.Erro)
		return
	}
	var dados protocolo.PublicarCaronaResp
	if !decodificar(resp.Dados, &dados) {
		return
	}
	fmt.Println("Carona publicada. ID:", dados.CaronaID)
}

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
	if !decodificar(resp.Dados, &dados) {
		return
	}
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
