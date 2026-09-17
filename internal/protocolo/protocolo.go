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

// Requisição do login, que solicita nome e senha.
type LoginReq struct {
	Nome  string `json:"nome"`
	Senha string `json:"senha,omitempty"`
}

// Requisição da publicação de uma carona feita por um motorista.
type PublicarCaronaReq struct {
	Rota              []string  `json:"rota"`
	Data              string    `json:"data"`
	PrecoPorTrecho    []float64 `json:"preco_por_trecho"`
	AssentosPorTrecho []int     `json:"assentos_por_trecho"`
}

// Resposta da publicação de uma carona, que retorna o ID da carona criada.
type PublicarCaronaResp struct {
	CaronaID string `json:"carona_id"`
}

// Detalhes de uma carona
type CaronaResumo struct {
	ID             string    `json:"id"`
	Motorista      string    `json:"motorista"`
	Rota           []string  `json:"rota"`
	Data           string    `json:"data"`
	PrecoPorTrecho []float64 `json:"preco_por_trecho"`
	AssentosLivres []int     `json:"assentos_livres"`
}

// Listagem de caronas disponíveis
type ListarCaronasResp struct {
	Caronas []CaronaResumo `json:"caronas"`
}

// Requisição para cancelar uma carona de acordo com o ID
type CancelarCaronaReq struct {
	CaronaID string `json:"carona_id"`
}

// Requisição para consultar passageiros de uma carona de acordo com o ID
type ConsultarPassageirosReq struct {
	CaronaID string `json:"carona_id"`
}

// Struct que indica o passageiro e o trecho que ele está viajando
type PassageiroTrecho struct {
	Passageiro   string `json:"passageiro"`
	TrechoInicio int    `json:"trecho_inicio"`
	TrechoFim    int    `json:"trecho_fim"`
}

// Resposta da consulta de passageiros, mostrando também seus respectivos trechos
type ConsultarPassageirosResp struct {
	Passageiros []PassageiroTrecho `json:"passageiros"`
}

// ItemPedido é usado tanto em pedidos quanto em respostas mais ricas.
// Algumas operações preenchem também Origem/Destino/Preco, só para exibição
// (o servidor ignora esses três campos quando o item chega dentro de um pedido).
type ItemPedido struct {
	CaronaID     string  `json:"carona_id"`
	TrechoInicio int     `json:"trecho_inicio"`
	TrechoFim    int     `json:"trecho_fim"`
	Origem       string  `json:"origem,omitempty"`
	Destino      string  `json:"destino,omitempty"`
	Preco        float64 `json:"preco,omitempty"`
}

// Um pedido de reserva é uma lista de itens. 
// Cada item é um intervalo de trechos de uma carona. 
type ReservarReq struct {
	Itens []ItemPedido `json:"itens"`
}

// A resposta da reserva indica o ID gerado
type ReservarResp struct {
	ReservaID string `json:"reserva_id"`
}

// ReservaResumo é usado na listagem de reservas do usuário
type ReservaResumo struct {
	ID    string       `json:"id"`
	Itens []ItemPedido `json:"itens"`
}

// Exibe todas as reservas ativas do passageiro
type ListarMinhasReservasResp struct {
	Reservas []ReservaResumo `json:"reservas"`
}

// Requisição para cancelar uma reserva de acordo com o ID
type CancelarReservaReq struct {
	ReservaID string `json:"reserva_id"`
}

// Requisição para a busca de itinerários .
type BuscarItinerariosReq struct {
	Origem  string `json:"origem"`
	Destino string `json:"destino"`
	Data    string `json:"data"`
}

// Lista de itens que compõem um itinerário encontrado.
type ItinerarioEncontrado struct {
	Itens []ItemPedido `json:"itens"`
	Preco float64      `json:"preco"`
}

// Resposta da busca de itinerários, que retorna uma lista de itinerários encontrados.
type BuscarItinerariosResp struct {
	Itinerarios []ItinerarioEncontrado `json:"itinerarios"`
}
