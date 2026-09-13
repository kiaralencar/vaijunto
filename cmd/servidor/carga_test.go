package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"vaijunto/internal/protocolo"
)

// clienteTeste fala com o servidor exatamente como um cliente de verdade
// falaria: só enxerga o protocolo (JSON + '\n'), nunca o Estado diretamente.
type clienteTeste struct {
	conn   net.Conn
	leitor *bufio.Reader
}

func (c *clienteTeste) enviar(tipo string, dados interface{}) protocolo.Resposta {
	corpo, _ := json.Marshal(dados)
	pedido := protocolo.Pedido{Tipo: tipo, Dados: corpo}
	linha, _ := json.Marshal(pedido)
	c.conn.Write(append(linha, '\n'))

	respLinha, err := c.leitor.ReadString('\n')
	if err != nil {
		return protocolo.Resposta{Ok: false, Erro: "conexao caiu: " + err.Error()}
	}
	var resp protocolo.Resposta
	json.Unmarshal([]byte(respLinha), &resp)
	return resp
}

func conectaClienteTeste(endereco, nome string) (*clienteTeste, error) {
	conn, err := net.Dial("tcp", endereco)
	if err != nil {
		return nil, err
	}
	c := &clienteTeste{conn: conn, leitor: bufio.NewReader(conn)}
	resp := c.enviar(protocolo.Login, protocolo.LoginReq{Nome: nome})
	if !resp.Ok {
		return nil, fmt.Errorf("login recusado: %s", resp.Erro)
	}
	return c, nil
}

func iniciaServidorTeste(t *testing.T) (string, *Estado) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("erro ao escutar: %v", err)
	}
	estado := NovoEstado()

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go atendeCliente(conn, estado)
		}
	}()

	t.Cleanup(func() { l.Close() })
	return l.Addr().String(), estado
}

func TestReservaConcorrenteNaoVendeAssentoDuasVezes(t *testing.T) {
	endereco, _ := iniciaServidorTeste(t)

	const capacidade = 5
	const numClientes = 60

	motorista, err := conectaClienteTeste(endereco, "motorista-teste")
	if err != nil {
		t.Fatalf("erro ao conectar motorista: %v", err)
	}
	defer motorista.conn.Close()

	respPublica := motorista.enviar(protocolo.PublicarCarona, protocolo.PublicarCaronaReq{
		Rota:              []string{"A", "B"},
		Data:              "2026-09-20",
		PrecoPorTrecho:    []float64{10},
		AssentosPorTrecho: []int{capacidade},
	})
	if !respPublica.Ok {
		t.Fatalf("erro ao publicar carona: %s", respPublica.Erro)
	}
	var dadosCarona protocolo.PublicarCaronaResp
	json.Unmarshal(respPublica.Dados, &dadosCarona)
	caronaID := dadosCarona.CaronaID

	var mu sync.Mutex
	var sucessos int
	latencias := make([]time.Duration, 0, numClientes)

	var wg sync.WaitGroup
	wg.Add(numClientes)
	for i := 0; i < numClientes; i++ {
		go func(i int) {
			defer wg.Done()

			passageiro, err := conectaClienteTeste(endereco, fmt.Sprintf("passageiro-%d", i))
			if err != nil {
				t.Errorf("erro ao conectar passageiro %d: %v", i, err)
				return
			}
			defer passageiro.conn.Close()

			inicio := time.Now()
			resp := passageiro.enviar(protocolo.Reservar, protocolo.ReservarReq{
				Itens: []protocolo.ItemPedido{{CaronaID: caronaID, TrechoInicio: 0, TrechoFim: 0}},
			})
			duracao := time.Since(inicio)

			mu.Lock()
			latencias = append(latencias, duracao)
			if resp.Ok {
				sucessos++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if sucessos != capacidade {
		t.Fatalf("esperava exatamente %d reservas confirmadas (capacidade), mas foram %d de %d tentativas",
			capacidade, sucessos, numClientes)
	}

	// Confere que o número de passageiros confirmados bate com a
	// capacidade, ou seja, nenhum assento foi vendido duas vezes.
	auditor, err := conectaClienteTeste(endereco, "auditor")
	if err != nil {
		t.Fatalf("erro ao conectar auditor: %v", err)
	}
	defer auditor.conn.Close()

	respConsulta := auditor.enviar(protocolo.ConsultarPassageiros, protocolo.ConsultarPassageirosReq{CaronaID: caronaID})
	var dadosConsulta protocolo.ConsultarPassageirosResp
	json.Unmarshal(respConsulta.Dados, &dadosConsulta)
	if len(dadosConsulta.Passageiros) != capacidade {
		t.Fatalf("esperava %d passageiros confirmados no trecho, mas ha %d", capacidade, len(dadosConsulta.Passageiros))
	}

	relataLatencias(t, latencias)
}

func TestReservaMultiCaronaEAtomica(t *testing.T) {
	endereco, _ := iniciaServidorTeste(t)

	motorista, err := conectaClienteTeste(endereco, "motorista-teste")
	if err != nil {
		t.Fatalf("erro ao conectar motorista: %v", err)
	}
	defer motorista.conn.Close()

	respC1 := motorista.enviar(protocolo.PublicarCarona, protocolo.PublicarCaronaReq{
		Rota: []string{"A", "B"}, Data: "2026-09-20",
		PrecoPorTrecho: []float64{10}, AssentosPorTrecho: []int{1},
	})
	var c1 protocolo.PublicarCaronaResp
	json.Unmarshal(respC1.Dados, &c1)

	respC2 := motorista.enviar(protocolo.PublicarCarona, protocolo.PublicarCaronaReq{
		Rota: []string{"B", "C"}, Data: "2026-09-20",
		PrecoPorTrecho: []float64{10}, AssentosPorTrecho: []int{0}, // sem assento, de proposito
	})
	var c2 protocolo.PublicarCaronaResp
	json.Unmarshal(respC2.Dados, &c2)

	passageiro, err := conectaClienteTeste(endereco, "passageiro-teste")
	if err != nil {
		t.Fatalf("erro ao conectar passageiro: %v", err)
	}
	defer passageiro.conn.Close()

	resp := passageiro.enviar(protocolo.Reservar, protocolo.ReservarReq{
		Itens: []protocolo.ItemPedido{
			{CaronaID: c1.CaronaID, TrechoInicio: 0, TrechoFim: 0},
			{CaronaID: c2.CaronaID, TrechoInicio: 0, TrechoFim: 0},
		},
	})
	if resp.Ok {
		t.Fatalf("a reserva deveria ter sido recusada (carona 2 sem assento), mas foi confirmada")
	}

	// O trecho da carona 1 tem que continuar livre: não pode ter sido
	// decrementado só porque o pedido incluía também a carona 2 sem assento.
	respListar := passageiro.enviar(protocolo.ListarCaronas, struct{}{})
	var listagem protocolo.ListarCaronasResp
	json.Unmarshal(respListar.Dados, &listagem)
	for _, c := range listagem.Caronas {
		if c.ID == c1.CaronaID && c.AssentosLivres[0] != 1 {
			t.Fatalf("carona 1 deveria continuar com 1 assento livre (reserva deveria ser tudo ou nada), tem %d",
				c.AssentosLivres[0])
		}
	}
}

// relataLatencias imprime média, mediana e máximo do tempo de resposta.
func relataLatencias(t *testing.T, latencias []time.Duration) {
	t.Helper()
	if len(latencias) == 0 {
		return
	}
	sort.Slice(latencias, func(i, j int) bool { return latencias[i] < latencias[j] })

	var soma time.Duration
	for _, l := range latencias {
		soma += l
	}
	media := soma / time.Duration(len(latencias))
	mediana := latencias[len(latencias)/2]
	maximo := latencias[len(latencias)-1]

	t.Logf("tempo de resposta sob carga (%d requisicoes concorrentes): media=%v mediana=%v max=%v",
		len(latencias), media, mediana, maximo)
}
