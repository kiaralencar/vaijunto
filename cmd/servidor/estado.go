package main

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"vaijunto/internal/dominio"
)

// dataParaComparar converte "DD/MM/AAAA" num tempo comparável.
func dataParaComparar(data string) time.Time {
	t, err := time.Parse("02/01/2006", data)
	if err != nil {
		return time.Time{}
	}
	return t
}

// Estado é todo o estado do servidor: todas as caronas, todas as reservas de
// todos os passageiros. Um único mutex protege tudo.
type Estado struct {
	mu             sync.Mutex
	proximoCarona  int
	proximaReserva int
	caronas        map[string]*dominio.Carona
	reservas       map[string]*dominio.Reserva
	senhas         map[string]string
}

func NovoEstado() *Estado {
	return &Estado{
		caronas:  make(map[string]*dominio.Carona),
		reservas: make(map[string]*dominio.Reserva),
		senhas:   make(map[string]string),
	}
}

// Autentica resolve o problema de dois clientes usarem o mesmo nome
func (e *Estado) Autentica(nome, senha string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	senhaExistente, jaExiste := e.senhas[nome]
	if !jaExiste {
		e.senhas[nome] = senha
		return nil
	}
	if senhaExistente != senha {
		return fmt.Errorf("nome ja esta em uso com outra senha")
	}
	return nil
}

// BuscarCarona devolve uma carona pelo ID, para os handlers que só precisam
// ler dados dela (ex.: nomes de cidade para exibir numa listagem de reservas).
func (e *Estado) BuscarCarona(caronaID string) (*dominio.Carona, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	c, existe := e.caronas[caronaID]
	return c, existe
}

func (e *Estado) PublicarCarona(motorista string, rota []string, data string, preco []float64, assentos []int) *dominio.Carona {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.proximoCarona++
	id := fmt.Sprintf("c%d", e.proximoCarona)

	livres := make([]int, len(assentos))
	copy(livres, assentos)

	c := &dominio.Carona{
		ID:             id,
		Motorista:      motorista,
		Rota:           rota,
		Data:           data,
		PrecoPorTrecho: preco,
		AssentosLivres: livres,
		Ativa:          true,
	}
	e.caronas[id] = c
	return c
}

func (e *Estado) ListarCaronas() []*dominio.Carona {
	e.mu.Lock()
	defer e.mu.Unlock()

	lista := make([]*dominio.Carona, 0, len(e.caronas))
	for _, c := range e.caronas {
		if c.Ativa {
			lista = append(lista, c)
		}
	}

	// Ordenação: data mais próxima primeiro. Desempate pelo ID (ordem de criação).
	sort.Slice(lista, func(i, j int) bool {
		if lista[i].Data != lista[j].Data {
			return dataParaComparar(lista[i].Data).Before(dataParaComparar(lista[j].Data))
		}
		return lista[i].ID < lista[j].ID
	})
	return lista
}

func (e *Estado) CancelarCarona(motorista, caronaID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	c, existe := e.caronas[caronaID]
	if !existe {
		return fmt.Errorf("carona nao encontrada")
	}
	if c.Motorista != motorista {
		return fmt.Errorf("carona pertence a outro motorista")
	}
	c.Ativa = false

	// Toda reserva que dependia desta carona deixa de valer — mesmo que
	// combinasse trechos de OUTRA carona ainda ativa, porque o itinerário
	// como um todo não pode mais ser cumprido. Os assentos que essa reserva
	// ocupava em outras caronas (ainda ativas) voltam a ficar livres.
	for _, r := range e.reservas {
		if !r.Ativa {
			continue
		}
		afetada := false
		for _, item := range r.Itens {
			if item.CaronaID == caronaID {
				afetada = true
				break
			}
		}
		if !afetada {
			continue
		}
		for _, item := range r.Itens {
			if item.CaronaID == caronaID {
				continue
			}
			if outraCarona, existe := e.caronas[item.CaronaID]; existe {
				for t := item.TrechoInicio; t <= item.TrechoFim; t++ {
					outraCarona.AssentosLivres[t]++
				}
			}
		}
		r.Ativa = false
	}
	return nil
}

// ConsultarPassageiros só devolve os passageiros de uma carona para o
// motorista dela — outro motorista não pode ver quem reservou a
// carona de outra pessoa.
func (e *Estado) ConsultarPassageiros(solicitante, caronaID string) ([]dominio.PassageiroNoTrecho, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	c, existe := e.caronas[caronaID]
	if !existe {
		return nil, fmt.Errorf("carona nao encontrada")
	}
	if c.Motorista != solicitante {
		return nil, fmt.Errorf("carona pertence a outro motorista")
	}

	var passageiros []dominio.PassageiroNoTrecho
	for _, r := range e.reservas {
		if !r.Ativa {
			continue
		}
		for _, item := range r.Itens {
			if item.CaronaID == caronaID {
				passageiros = append(passageiros, dominio.PassageiroNoTrecho{
					Passageiro:   r.Passageiro,
					TrechoInicio: item.TrechoInicio,
					TrechoFim:    item.TrechoFim,
				})
			}
		}
	}
	return passageiros, nil
}

// Reservar é a operação central do sistema. Recebe uma lista de itens (cada
// um: uma carona + um intervalo de trechos) e só confirma se todos os
// trechos de todos os itens tiverem assento livre. 
func (e *Estado) Reservar(passageiro string, itens []dominio.ItemReserva) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(itens) == 0 {
		return "", fmt.Errorf("nenhum item para reservar")
	}

	// Primeira passada: validação.
	for _, item := range itens {
		carona, existe := e.caronas[item.CaronaID]
		if !existe || !carona.Ativa {
			return "", fmt.Errorf("carona %s nao encontrada ou inativa", item.CaronaID)
		}
		if item.TrechoInicio < 0 || item.TrechoFim >= len(carona.AssentosLivres) || item.TrechoInicio > item.TrechoFim {
			return "", fmt.Errorf("intervalo de trechos invalido para carona %s", item.CaronaID)
		}
		for t := item.TrechoInicio; t <= item.TrechoFim; t++ {
			if carona.AssentosLivres[t] <= 0 {
				return "", fmt.Errorf("sem assento livre no trecho %d da carona %s", t, item.CaronaID)
			}
		}
	}

	// Segunda passada: aplicação.
	for _, item := range itens {
		carona := e.caronas[item.CaronaID]
		for t := item.TrechoInicio; t <= item.TrechoFim; t++ {
			carona.AssentosLivres[t]--
		}
	}

	e.proximaReserva++
	id := fmt.Sprintf("r%d", e.proximaReserva)
	e.reservas[id] = &dominio.Reserva{
		ID:         id,
		Passageiro: passageiro,
		Itens:      itens,
		Ativa:      true,
	}
	return id, nil
}

func (e *Estado) CancelarReserva(passageiro, reservaID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	r, existe := e.reservas[reservaID]
	if !existe || !r.Ativa {
		return fmt.Errorf("reserva nao encontrada")
	}
	if r.Passageiro != passageiro {
		return fmt.Errorf("reserva pertence a outro passageiro")
	}

	for _, item := range r.Itens {
		carona, existe := e.caronas[item.CaronaID]
		if !existe {
			continue
		}
		for t := item.TrechoInicio; t <= item.TrechoFim; t++ {
			carona.AssentosLivres[t]++
		}
	}
	r.Ativa = false
	return nil
}

func (e *Estado) ListarReservas(passageiro string) []*dominio.Reserva {
	e.mu.Lock()
	defer e.mu.Unlock()

	var lista []*dominio.Reserva
	for _, r := range e.reservas {
		if r.Passageiro == passageiro && r.Ativa {
			lista = append(lista, r)
		}
	}
	return lista
}
