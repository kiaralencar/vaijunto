// Package protocolo define o formato das mensagens trocadas entre cliente e servidor.
package protocolo

import "encoding/json"

// Tipos de operação suportados.
const (
	Login                = "LOGIN"
	PublicarCarona       = "PUBLICAR_CARONA"
	ListarCaronas        = "LISTAR_CARONAS"
	ConsultarPassageiros = "CONSULTAR_PASSAGEIROS"
	CancelarCarona       = "CANCELAR_CARONA"
	BuscarItinerarios    = "BUSCAR_ITINERARIOS"
	Reservar             = "RESERVAR"
	ListarMinhasReservas = "LISTAR_MINHAS_RESERVAS"
	CancelarReserva      = "CANCELAR_RESERVA"
)

// Pedido é o envelope de toda mensagem cliente -> servidor.
type Pedido struct {
	Tipo  string          `json:"tipo"`
	ID    string          `json:"id,omitempty"`
	Dados json.RawMessage `json:"dados,omitempty"`
}

// Resposta é o envelope de toda mensagem servidor -> cliente.
type Resposta struct {
	Tipo  string          `json:"tipo"`
	ID    string          `json:"id,omitempty"`
	Ok    bool            `json:"ok"`
	Erro  string          `json:"erro,omitempty"`
	Dados json.RawMessage `json:"dados,omitempty"`
}

type LoginReq struct {
	Nome string `json:"nome"`
}

type PublicarCaronaReq struct {
	Rota              []string  `json:"rota"`
	Data              string    `json:"data"`
	PrecoPorTrecho    []float64 `json:"preco_por_trecho"`
	AssentosPorTrecho []int     `json:"assentos_por_trecho"`
}

type PublicarCaronaResp struct {
	CaronaID string `json:"carona_id"`
}

type CaronaResumo struct {
	ID             string    `json:"id"`
	Motorista      string    `json:"motorista"`
	Rota           []string  `json:"rota"`
	Data           string    `json:"data"`
	PrecoPorTrecho []float64 `json:"preco_por_trecho"`
	AssentosLivres []int     `json:"assentos_livres"`
}

type ListarCaronasResp struct {
	Caronas []CaronaResumo `json:"caronas"`
}

type CancelarCaronaReq struct {
	CaronaID string `json:"carona_id"`
}

type ConsultarPassageirosReq struct {
	CaronaID string `json:"carona_id"`
}

type PassageiroTrecho struct {
	Passageiro   string `json:"passageiro"`
	TrechoInicio int    `json:"trecho_inicio"`
	TrechoFim    int    `json:"trecho_fim"`
}

type ConsultarPassageirosResp struct {
	Passageiros []PassageiroTrecho `json:"passageiros"`
}

// Um pedido de reserva é uma lista de itens. Cada item é um intervalo de
// trechos de uma carona. 

type ItemPedido struct {
	CaronaID     string `json:"carona_id"`
	TrechoInicio int    `json:"trecho_inicio"`
	TrechoFim    int    `json:"trecho_fim"`
}

type ReservarReq struct {
	Itens []ItemPedido `json:"itens"`
}

type ReservarResp struct {
	ReservaID string `json:"reserva_id"`
}

type ReservaResumo struct {
	ID    string       `json:"id"`
	Itens []ItemPedido `json:"itens"`
}

type ListarMinhasReservasResp struct {
	Reservas []ReservaResumo `json:"reservas"`
}

type CancelarReservaReq struct {
	ReservaID string `json:"reserva_id"`
}

type BuscarItinerariosReq struct {
	Origem  string `json:"origem"`
	Destino string `json:"destino"`
	Data    string `json:"data"`
}

type ItinerarioEncontrado struct {
	Itens []ItemPedido `json:"itens"`
	Preco float64      `json:"preco"`
}

type BuscarItinerariosResp struct {
	Itinerarios []ItinerarioEncontrado `json:"itinerarios"`
}
