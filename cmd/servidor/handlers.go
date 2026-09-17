package main

import (
	"encoding/json"
	"vaijunto/internal/dominio"
	"vaijunto/internal/protocolo"
)

// processaLinha decodifica uma linha no envelope Pedido e 
// despacha para o handler certo pelo campo Tipo.
func processaLinha(linha string, estado *Estado, sessao *Sessao) protocolo.Resposta {
	var pedido protocolo.Pedido
	if err := json.Unmarshal([]byte(linha), &pedido); err != nil {
		// Mensagem malformada: responde com erro e não derruba a conexão nemo servidor.
		return protocolo.Resposta{Ok: false, Erro: "mensagem malformada: " + err.Error()}
	}

	switch pedido.Tipo {
	case protocolo.Login:
		return trataLogin(pedido, estado, sessao)
	case protocolo.PublicarCarona:
		return trataPublicarCarona(pedido, estado, sessao)
	case protocolo.ListarCaronas:
		return trataListarCaronas(pedido, estado)
	case protocolo.Reservar:
		return trataReservar(pedido, estado, sessao)
	case protocolo.CancelarReserva:
		return trataCancelarReserva(pedido, estado, sessao)
	case protocolo.ListarMinhasReservas:
		return trataListarMinhasReservas(pedido, estado, sessao)
	case protocolo.CancelarCarona:
		return trataCancelarCarona(pedido, estado, sessao)
	case protocolo.ConsultarPassageiros:
		return trataConsultarPassageiros(pedido, estado, sessao)
	case protocolo.BuscarItinerarios:
		return trataBuscarItinerarios(pedido, estado)
	default:
		return erro(pedido, "tipo de operacao desconhecido: "+pedido.Tipo)
	}
}

// Monta uma resposta de erro para o pedido dado
func erro(pedido protocolo.Pedido, msg string) protocolo.Resposta {
	return protocolo.Resposta{Tipo: pedido.Tipo, ID: pedido.ID, Ok: false, Erro: msg}
}

// Monta uma resposta de sucesso para o pedido dado
func ok(pedido protocolo.Pedido, dados interface{}) protocolo.Resposta {
	b, err := json.Marshal(dados)
	if err != nil {
		return erro(pedido, "erro interno ao montar resposta: "+err.Error())
	}
	return protocolo.Resposta{Tipo: pedido.Tipo, ID: pedido.ID, Ok: true, Dados: b}
}

// exigeLogin é usado pelos handlers que precisam saber quem está pedindo.
func exigeLogin(pedido protocolo.Pedido, sessao *Sessao) (protocolo.Resposta, bool) {
	if sessao.Nome == "" {
		return erro(pedido, "faca login primeiro"), false
	}
	return protocolo.Resposta{}, true
}

// Trata o pedido de login, criando a senha se for a primeira vez que o nome é usado.
func trataLogin(pedido protocolo.Pedido, estado *Estado, sessao *Sessao) protocolo.Resposta {
	var req protocolo.LoginReq
	if err := json.Unmarshal(pedido.Dados, &req); err != nil || req.Nome == "" {
		return erro(pedido, "nome invalido")
	}
	if err := estado.Autentica(req.Nome, req.Senha); err != nil {
		return erro(pedido, err.Error())
	}
	sessao.Nome = req.Nome
	return ok(pedido, struct{}{})
}

// Trata o pedido de publicar uma nova carona, criando a carona no estado.
func trataPublicarCarona(pedido protocolo.Pedido, estado *Estado, sessao *Sessao) protocolo.Resposta {
	if resp, apto := exigeLogin(pedido, sessao); !apto {
		return resp
	}
	var req protocolo.PublicarCaronaReq
	if err := json.Unmarshal(pedido.Dados, &req); err != nil {
		return erro(pedido, "dados invalidos")
	}
	if len(req.Rota) < 2 {
		return erro(pedido, "rota precisa de pelo menos duas cidades")
	}
	numTrechos := len(req.Rota) - 1
	if len(req.PrecoPorTrecho) != numTrechos || len(req.AssentosPorTrecho) != numTrechos {
		return erro(pedido, "preco_por_trecho e assentos_por_trecho devem ter um item por trecho")
	}

	c := estado.PublicarCarona(sessao.Nome, req.Rota, req.Data, req.PrecoPorTrecho, req.AssentosPorTrecho)
	return ok(pedido, protocolo.PublicarCaronaResp{CaronaID: c.ID})
}

// Trata o pedido de listar todas as caronas ativas, retornando um resumo de cada uma.
func trataListarCaronas(pedido protocolo.Pedido, estado *Estado) protocolo.Resposta {
	caronas := estado.ListarCaronas()
	resumos := make([]protocolo.CaronaResumo, 0, len(caronas))
	for _, c := range caronas {
		resumos = append(resumos, protocolo.CaronaResumo{
			ID:             c.ID,
			Motorista:      c.Motorista,
			Rota:           c.Rota,
			Data:           c.Data,
			PrecoPorTrecho: c.PrecoPorTrecho,
			AssentosLivres: c.AssentosLivres,
		})
	}
	return ok(pedido, protocolo.ListarCaronasResp{Caronas: resumos})
}

// Trata o pedido de cancelar uma carona, verificando se o motorista é o dono da carona
func trataCancelarCarona(pedido protocolo.Pedido, estado *Estado, sessao *Sessao) protocolo.Resposta {
	if resp, apto := exigeLogin(pedido, sessao); !apto {
		return resp
	}
	var req protocolo.CancelarCaronaReq
	if err := json.Unmarshal(pedido.Dados, &req); err != nil {
		return erro(pedido, "dados invalidos")
	}
	if err := estado.CancelarCarona(sessao.Nome, req.CaronaID); err != nil {
		return erro(pedido, err.Error())
	}
	return ok(pedido, struct{}{})
}

// Trata o pedido de consultar passageiros de uma carona, verificando se o motorista é o dono da carona
func trataConsultarPassageiros(pedido protocolo.Pedido, estado *Estado, sessao *Sessao) protocolo.Resposta {
	if resp, apto := exigeLogin(pedido, sessao); !apto {
		return resp
	}
	var req protocolo.ConsultarPassageirosReq
	if err := json.Unmarshal(pedido.Dados, &req); err != nil {
		return erro(pedido, "dados invalidos")
	}
	passageiros, err := estado.ConsultarPassageiros(sessao.Nome, req.CaronaID)
	if err != nil {
		return erro(pedido, err.Error())
	}
	lista := make([]protocolo.PassageiroTrecho, len(passageiros))
	for i, p := range passageiros {
		lista[i] = protocolo.PassageiroTrecho{
			Passageiro:   p.Passageiro,
			TrechoInicio: p.TrechoInicio,
			TrechoFim:    p.TrechoFim,
		}
	}
	return ok(pedido, protocolo.ConsultarPassageirosResp{Passageiros: lista})
}

// Trata o pedido de buscar itinerários, retornando uma lista de itinerários encontrados
func trataBuscarItinerarios(pedido protocolo.Pedido, estado *Estado) protocolo.Resposta {
	var req protocolo.BuscarItinerariosReq
	if err := json.Unmarshal(pedido.Dados, &req); err != nil {
		return erro(pedido, "dados invalidos")
	}
	if req.Origem == "" || req.Destino == "" || req.Data == "" {
		return erro(pedido, "origem, destino e data sao obrigatorios")
	}

	encontrados := estado.BuscarItinerarios(req.Origem, req.Destino, req.Data)
	itinerarios := make([]protocolo.ItinerarioEncontrado, 0, len(encontrados))
	for _, it := range encontrados {
		itens := make([]protocolo.ItemPedido, len(it.itens))
		for i, item := range it.itens {
			itemResp := protocolo.ItemPedido{
				CaronaID:     item.CaronaID,
				TrechoInicio: item.TrechoInicio,
				TrechoFim:    item.TrechoFim,
			}
			if carona, existe := estado.BuscarCarona(item.CaronaID); existe {
				itemResp.Origem = carona.Rota[item.TrechoInicio]
				itemResp.Destino = carona.Rota[item.TrechoFim+1]
				for t := item.TrechoInicio; t <= item.TrechoFim; t++ {
					itemResp.Preco += carona.PrecoPorTrecho[t]
				}
			}
			itens[i] = itemResp
		}
		itinerarios = append(itinerarios, protocolo.ItinerarioEncontrado{Itens: itens, Preco: it.preco})
	}
	return ok(pedido, protocolo.BuscarItinerariosResp{Itinerarios: itinerarios})
}

// Trata o pedido de reservar uma carona, convertendo os itens e repassando para o estado
func trataReservar(pedido protocolo.Pedido, estado *Estado, sessao *Sessao) protocolo.Resposta {
	if resp, apto := exigeLogin(pedido, sessao); !apto {
		return resp
	}
	var req protocolo.ReservarReq
	if err := json.Unmarshal(pedido.Dados, &req); err != nil || len(req.Itens) == 0 {
		return erro(pedido, "itens invalidos")
	}

	itens := make([]dominio.ItemReserva, len(req.Itens))
	for i, it := range req.Itens {
		itens[i] = dominio.ItemReserva{
			CaronaID:     it.CaronaID,
			TrechoInicio: it.TrechoInicio,
			TrechoFim:    it.TrechoFim,
		}
	}

	id, err := estado.Reservar(sessao.Nome, itens)
	if err != nil {
		return erro(pedido, err.Error())
	}
	return ok(pedido, protocolo.ReservarResp{ReservaID: id})
}

// Trata o pedido de cancelar uma reserva, convertendo os dados e repassando para o estado
func trataCancelarReserva(pedido protocolo.Pedido, estado *Estado, sessao *Sessao) protocolo.Resposta {
	if resp, apto := exigeLogin(pedido, sessao); !apto {
		return resp
	}
	var req protocolo.CancelarReservaReq
	if err := json.Unmarshal(pedido.Dados, &req); err != nil {
		return erro(pedido, "dados invalidos")
	}
	if err := estado.CancelarReserva(sessao.Nome, req.ReservaID); err != nil {
		return erro(pedido, err.Error())
	}
	return ok(pedido, struct{}{})
}

// Trata o pedido de listar todas as reservas ativas do passageiro, retornando um resumo de cada uma.
func trataListarMinhasReservas(pedido protocolo.Pedido, estado *Estado, sessao *Sessao) protocolo.Resposta {
	if resp, apto := exigeLogin(pedido, sessao); !apto {
		return resp
	}
	reservas := estado.ListarReservas(sessao.Nome)
	resumos := make([]protocolo.ReservaResumo, 0, len(reservas))
	for _, r := range reservas {
		itens := make([]protocolo.ItemPedido, len(r.Itens))
		for i, it := range r.Itens {
			item := protocolo.ItemPedido{
				CaronaID:     it.CaronaID,
				TrechoInicio: it.TrechoInicio,
				TrechoFim:    it.TrechoFim,
			}
			
			if carona, existe := estado.BuscarCarona(it.CaronaID); existe {
				item.Origem = carona.Rota[it.TrechoInicio]
				item.Destino = carona.Rota[it.TrechoFim+1]
				for t := it.TrechoInicio; t <= it.TrechoFim; t++ {
					item.Preco += carona.PrecoPorTrecho[t]
				}
			}
			itens[i] = item
		}
		resumos = append(resumos, protocolo.ReservaResumo{ID: r.ID, Itens: itens})
	}
	return ok(pedido, protocolo.ListarMinhasReservasResp{Reservas: resumos})
}
