// Package dominio define as entidades do problema: carona, trecho, reserva.
// Servidor e clientes não compartilham este pacote diretamente (cada cliente só
// conhece o protocolo), mas o servidor inteiro é construído em cima destas structs.
package dominio

// Carona é publicada por um motorista.
type Carona struct {
	ID             string
	Motorista      string
	Rota           []string
	Data           string
	PrecoPorTrecho []float64
	AssentosLivres []int
	Ativa          bool
}

// NumTrechos devolve quantos trechos esta carona tem.
func (c *Carona) NumTrechos() int {
	return len(c.Rota) - 1
}

// ItemReserva referencia um intervalo de trechos consecutivos de uma carona.
type ItemReserva struct {
	CaronaID     string
	TrechoInicio int
	TrechoFim    int
}

// Reserva é o resultado de uma confirmação atômica: todos os Itens foram
// garantidos ao mesmo tempo, ou a reserva nunca chega a existir.
type Reserva struct {
	ID         string
	Passageiro string
	Itens      []ItemReserva
	Ativa      bool
}

// PassageiroNoTrecho é usado para responder "quem está reservado nesta carona".
type PassageiroNoTrecho struct {
	Passageiro   string
	TrechoInicio int
	TrechoFim    int
}
