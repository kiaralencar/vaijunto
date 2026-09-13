package main

import (
	"fmt"
	"sync"
	"vaijunto/internal/dominio"
)

// Estado é TODO o estado do servidor: todas as caronas, todas as reservas de
// todos os passageiros. Um único mutex protege tudo.
type Estado struct {
	mu             sync.Mutex
	proximoCarona  int
	proximaReserva int
	caronas        map[string]*dominio.Carona
	reservas       map[string]*dominio.Reserva
}

func NovoEstado() *Estado {
	return &Estado{
		caronas:  make(map[string]*dominio.Carona),
		reservas: make(map[string]*dominio.Reserva),
	}
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
	return nil
}

func (e *Estado) ConsultarPassageiros(caronaID string) ([]dominio.PassageiroNoTrecho, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, existe := e.caronas[caronaID]; !existe {
		return nil, fmt.Errorf("carona nao encontrada")
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
