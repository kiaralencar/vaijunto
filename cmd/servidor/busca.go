package main

import (
	"sort"
	"vaijunto/internal/dominio"
)

// Aresta representa um trecho com assento livre, no grafo de busca. De/para
// são cidades (os nós). Cada aresta sabe a que carona pertence, para que o
// caminho encontrado possa virar um pedido de reserva de verdade.
type aresta struct {
	caronaID string
	trecho   int
	de, para string
	preco    float64
}

// itinerarioEncontrado é o resultado de um caminho no grafo: uma lista de
// itens de reserva e o preço total somado.
type itinerarioEncontrado struct {
	itens []dominio.ItemReserva
	preco float64
}

// Limites de segurança para a busca não "explodir"
const (
	profundidadeMaximaBusca = 6  // No maximo 6 trechos por itinerario
	maxItinerarios          = 10 // Para de procurar depois de achar 10 caminhos
)

// BuscarItinerarios monta o grafo a partir das caronas ativas daquela data, 
// e faz uma DFS de origem a destino.
func (e *Estado) BuscarItinerarios(origem, destino, data string) []itinerarioEncontrado {
	e.mu.Lock() // Bloqueia o estado para que não haja alteração de caronas durante a busca
	defer e.mu.Unlock() // Desbloqueia o estado no final da função

	// Monta o grafo: cada cidade é um nó, cada trecho com assento livre é uma aresta.
	grafo := make(map[string][]aresta)
	for _, c := range e.caronas {
		if !c.Ativa || c.Data != data {
			continue
		}
		for t := 0; t < c.NumTrechos(); t++ {
			if c.AssentosLivres[t] <= 0 {
				continue // Sem assento neste trecho: nao vira aresta
			}
			de, para := c.Rota[t], c.Rota[t+1]
			grafo[de] = append(grafo[de], aresta{
				caronaID: c.ID,
				trecho:   t,
				de:       de,
				para:     para,
				preco:    c.PrecoPorTrecho[t],
			})
		}
	}

	// DFS (Depth First Search) para achar caminhos de origem a destino, respeitando
	// os limites de profundidade e quantidade de itinerarios encontrados.
	var resultados []itinerarioEncontrado
	visitado := map[string]bool{origem: true}
	var caminho []aresta

	var dfs func(atual string)
	dfs = func(atual string) {
		if len(resultados) >= maxItinerarios {
			return // Itinerários demais? Para de procurar.
		}
		if atual == destino && len(caminho) > 0 {
			resultados = append(resultados, montaItinerario(caminho))
			return // Achou o destino? Para de procurar. 
		}
		if len(caminho) >= profundidadeMaximaBusca {
			return // Já foi fundo demais? Para de procurar.
		}
		for _, a := range grafo[atual] {
			if visitado[a.para] {
				continue // Nao revisita cidade: evita ciclo infinito
			}
			visitado[a.para] = true
			caminho = append(caminho, a)

			dfs(a.para)

			// Desfaz a escolha para tentar o proximo caminho (backtracking)
			caminho = caminho[:len(caminho)-1]
			visitado[a.para] = false
		}
	}
	dfs(origem)

	// Critério de ordenação: mais barato primeiro. Em caso de empate no preço,
	// o itinerário com menos itens (menos baldeações entre motoristas) vem primeiro.
	sort.Slice(resultados, func(i, j int) bool {
		if resultados[i].preco != resultados[j].preco {
			return resultados[i].preco < resultados[j].preco
		}
		return len(resultados[i].itens) < len(resultados[j].itens)
	})

	return resultados
}

// montaItinerario junta arestas consecutivas da mesma carona num único ItemReserva
func montaItinerario(caminho []aresta) itinerarioEncontrado {
	var itens []dominio.ItemReserva
	var preco float64

	for _, a := range caminho {
		preco += a.preco
		n := len(itens)
		if n > 0 && itens[n-1].CaronaID == a.caronaID && itens[n-1].TrechoFim == a.trecho-1 {
			itens[n-1].TrechoFim = a.trecho
			continue
		}
		itens = append(itens, dominio.ItemReserva{
			CaronaID:     a.caronaID,
			TrechoInicio: a.trecho,
			TrechoFim:    a.trecho,
		})
	}

	return itinerarioEncontrado{itens: itens, preco: preco}
}
