package db

import (
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
)

// ===============================
// EXCHANGE SERVICES
// ===============================

func GetAllExchanges() ([]modelsapi.ExchangeDTO, error) {
	exchanges, err := repository.GetSingleton().GetAllExchanges()
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.ExchangeDTO, len(exchanges))
	for i, exchange := range exchanges {
		result[i] = modelsapi.ExchangeDTO{
			ID:       exchange.ID,
			Code:     exchange.Code,
			Name:     exchange.Name,
			Timezone: exchange.Timezone,
		}
	}
	return result, nil
}

func GetExchangeByID(id uint) (*modelsapi.ExchangeDTO, error) {
	exchange, err := repository.GetSingleton().GetExchangeByID(id)
	if err != nil {
		return nil, err
	}

	return &modelsapi.ExchangeDTO{
		ID:       exchange.ID,
		Code:     exchange.Code,
		Name:     exchange.Name,
		Timezone: exchange.Timezone,
	}, nil
}

func GetExchangeByCode(code string) (*modelsapi.ExchangeDTO, error) {
	exchange, err := repository.GetSingleton().GetExchangeByCode(code)
	if err != nil {
		return nil, err
	}

	return &modelsapi.ExchangeDTO{
		ID:       exchange.ID,
		Code:     exchange.Code,
		Name:     exchange.Name,
		Timezone: exchange.Timezone,
	}, nil
}

func CreateExchange(req modelsapi.CreateExchangeRequest) (*modelsapi.ExchangeDTO, error) {
	exchange := &modelsdb.Exchange{
		Code:     req.Code,
		Name:     req.Name,
		Timezone: req.Timezone,
	}

	err := repository.GetSingleton().CreateExchange(exchange)
	if err != nil {
		return nil, err
	}

	return &modelsapi.ExchangeDTO{
		ID:       exchange.ID,
		Code:     exchange.Code,
		Name:     exchange.Name,
		Timezone: exchange.Timezone,
	}, nil
}
