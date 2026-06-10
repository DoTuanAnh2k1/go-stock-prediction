package db

import (
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
	"time"
)

// ===============================
// STOCK PRICE SERVICES
// ===============================

func GetAllStockPrices() ([]modelsapi.StockPriceDTO, error) {
	stockPrices, err := repository.GetSingleton().GetAllStockPrices()
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.StockPriceDTO, len(stockPrices))
	for i, price := range stockPrices {
		result[i] = modelsapi.StockPriceDTO{
			ID:            price.ID,
			StockID:       price.StockID,
			TradingDate:   price.TradingDate,
			OpenPrice:     price.OpenPrice,
			HighPrice:     price.HighPrice,
			LowPrice:      price.LowPrice,
			ClosePrice:    price.ClosePrice,
			Volume:        price.Volume,
			Value:         price.Value,
			Change:        price.Change,
			ChangePercent: price.ChangePercent,
		}
	}
	return result, nil
}

func GetStockPriceByID(id uint) (*modelsapi.StockPriceDTO, error) {
	stockPrice, err := repository.GetSingleton().GetStockPriceByID(id)
	if err != nil {
		return nil, err
	}

	return &modelsapi.StockPriceDTO{
		ID:            stockPrice.ID,
		StockID:       stockPrice.StockID,
		TradingDate:   stockPrice.TradingDate,
		OpenPrice:     stockPrice.OpenPrice,
		HighPrice:     stockPrice.HighPrice,
		LowPrice:      stockPrice.LowPrice,
		ClosePrice:    stockPrice.ClosePrice,
		Volume:        stockPrice.Volume,
		Value:         stockPrice.Value,
		Change:        stockPrice.Change,
		ChangePercent: stockPrice.ChangePercent,
	}, nil
}

func GetStockPricesByStockID(stockID uint) ([]modelsapi.StockPriceDTO, error) {
	stockPrices, err := repository.GetSingleton().GetStockPricesByStockID(stockID)
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.StockPriceDTO, len(stockPrices))
	for i, price := range stockPrices {
		result[i] = modelsapi.StockPriceDTO{
			ID:            price.ID,
			StockID:       price.StockID,
			TradingDate:   price.TradingDate,
			OpenPrice:     price.OpenPrice,
			HighPrice:     price.HighPrice,
			LowPrice:      price.LowPrice,
			ClosePrice:    price.ClosePrice,
			Volume:        price.Volume,
			Value:         price.Value,
			Change:        price.Change,
			ChangePercent: price.ChangePercent,
		}
	}
	return result, nil
}

func GetStockPricesByStockIDAndDateRange(stockID uint, fromDate, toDate time.Time) ([]modelsapi.StockPriceDTO, error) {
	stockPrices, err := repository.GetSingleton().GetStockPricesByStockIDAndDateRange(stockID, fromDate, toDate)
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.StockPriceDTO, len(stockPrices))
	for i, price := range stockPrices {
		result[i] = modelsapi.StockPriceDTO{
			ID:            price.ID,
			StockID:       price.StockID,
			TradingDate:   price.TradingDate,
			OpenPrice:     price.OpenPrice,
			HighPrice:     price.HighPrice,
			LowPrice:      price.LowPrice,
			ClosePrice:    price.ClosePrice,
			Volume:        price.Volume,
			Value:         price.Value,
			Change:        price.Change,
			ChangePercent: price.ChangePercent,
		}
	}
	return result, nil
}

func GetLatestStockPriceByStockID(stockID uint) (*modelsapi.StockPriceDTO, error) {
	stockPrice, err := repository.GetSingleton().GetLatestStockPriceByStockID(stockID)
	if err != nil {
		return nil, err
	}

	return &modelsapi.StockPriceDTO{
		ID:            stockPrice.ID,
		StockID:       stockPrice.StockID,
		TradingDate:   stockPrice.TradingDate,
		OpenPrice:     stockPrice.OpenPrice,
		HighPrice:     stockPrice.HighPrice,
		LowPrice:      stockPrice.LowPrice,
		ClosePrice:    stockPrice.ClosePrice,
		Volume:        stockPrice.Volume,
		Value:         stockPrice.Value,
		Change:        stockPrice.Change,
		ChangePercent: stockPrice.ChangePercent,
	}, nil
}

func GetLatestStockPricesForVN30() ([]modelsapi.StockPriceDTO, error) {
	stockPrices, err := repository.GetSingleton().GetLatestStockPricesForVN30()
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.StockPriceDTO, len(stockPrices))
	for i, price := range stockPrices {
		result[i] = modelsapi.StockPriceDTO{
			ID:            price.ID,
			StockID:       price.StockID,
			TradingDate:   price.TradingDate,
			OpenPrice:     price.OpenPrice,
			HighPrice:     price.HighPrice,
			LowPrice:      price.LowPrice,
			ClosePrice:    price.ClosePrice,
			Volume:        price.Volume,
			Value:         price.Value,
			Change:        price.Change,
			ChangePercent: price.ChangePercent,
		}
	}
	return result, nil
}

func CreateStockPrice(req modelsapi.CreateStockPriceRequest) (*modelsapi.StockPriceDTO, error) {
	stockPrice := &modelsdb.StockPrice{
		StockID:       req.StockID,
		TradingDate:   req.TradingDate,
		OpenPrice:     req.OpenPrice,
		HighPrice:     req.HighPrice,
		LowPrice:      req.LowPrice,
		ClosePrice:    req.ClosePrice,
		Volume:        req.Volume,
		Value:         req.Value,
		Change:        req.Change,
		ChangePercent: req.ChangePercent,
	}

	err := repository.GetSingleton().CreateStockPrice(stockPrice)
	if err != nil {
		return nil, err
	}

	return &modelsapi.StockPriceDTO{
		ID:            stockPrice.ID,
		StockID:       stockPrice.StockID,
		TradingDate:   stockPrice.TradingDate,
		OpenPrice:     stockPrice.OpenPrice,
		HighPrice:     stockPrice.HighPrice,
		LowPrice:      stockPrice.LowPrice,
		ClosePrice:    stockPrice.ClosePrice,
		Volume:        stockPrice.Volume,
		Value:         stockPrice.Value,
		Change:        stockPrice.Change,
		ChangePercent: stockPrice.ChangePercent,
	}, nil
}

func UpsertStockPrice(req modelsapi.CreateStockPriceRequest) (*modelsapi.StockPriceDTO, error) {
	stockPrice := &modelsdb.StockPrice{
		StockID:       req.StockID,
		TradingDate:   req.TradingDate,
		OpenPrice:     req.OpenPrice,
		HighPrice:     req.HighPrice,
		LowPrice:      req.LowPrice,
		ClosePrice:    req.ClosePrice,
		Volume:        req.Volume,
		Value:         req.Value,
		Change:        req.Change,
		ChangePercent: req.ChangePercent,
	}

	err := repository.GetSingleton().UpsertStockPrice(stockPrice)
	if err != nil {
		return nil, err
	}

	return &modelsapi.StockPriceDTO{
		ID:            stockPrice.ID,
		StockID:       stockPrice.StockID,
		TradingDate:   stockPrice.TradingDate,
		OpenPrice:     stockPrice.OpenPrice,
		HighPrice:     stockPrice.HighPrice,
		LowPrice:      stockPrice.LowPrice,
		ClosePrice:    stockPrice.ClosePrice,
		Volume:        stockPrice.Volume,
		Value:         stockPrice.Value,
		Change:        stockPrice.Change,
		ChangePercent: stockPrice.ChangePercent,
	}, nil
}
