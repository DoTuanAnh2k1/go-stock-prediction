package db

import (
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
)

// ===============================
// STOCK SERVICES
// ===============================

func GetAllStocks() ([]modelsapi.StockDTO, error) {
	stocks, err := repository.GetSingleton().GetAllStocks()
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.StockDTO, len(stocks))
	for i, stock := range stocks {
		result[i] = modelsapi.StockDTO{
			ID:          stock.ID,
			Symbol:      stock.Symbol,
			CompanyName: stock.CompanyName,
			ExchangeID:  stock.ExchangeID,
			IsVN30:      stock.IsVN30,
			IsVN100:     stock.IsVN100,
			Sector:      stock.Sector,
			ListingDate: stock.ListingDate,
		}
	}
	return result, nil
}

func GetStockByID(id uint) (*modelsapi.StockDTO, error) {
	stock, err := repository.GetSingleton().GetStockByID(id)
	if err != nil {
		return nil, err
	}

	return &modelsapi.StockDTO{
		ID:          stock.ID,
		Symbol:      stock.Symbol,
		CompanyName: stock.CompanyName,
		ExchangeID:  stock.ExchangeID,
		IsVN30:      stock.IsVN30,
		IsVN100:     stock.IsVN100,
		Sector:      stock.Sector,
		ListingDate: stock.ListingDate,
	}, nil
}

func GetStockBySymbol(symbol string) (*modelsapi.StockDTO, error) {
	stock, err := repository.GetSingleton().GetStockBySymbol(symbol)
	if err != nil {
		return nil, err
	}

	return &modelsapi.StockDTO{
		ID:          stock.ID,
		Symbol:      stock.Symbol,
		CompanyName: stock.CompanyName,
		ExchangeID:  stock.ExchangeID,
		IsVN30:      stock.IsVN30,
		IsVN100:     stock.IsVN100,
		Sector:      stock.Sector,
		ListingDate: stock.ListingDate,
	}, nil
}

func GetVN30Stocks() ([]modelsapi.StockDTO, error) {
	stocks, err := repository.GetSingleton().GetVN30Stocks()
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.StockDTO, len(stocks))
	for i, stock := range stocks {
		result[i] = modelsapi.StockDTO{
			ID:          stock.ID,
			Symbol:      stock.Symbol,
			CompanyName: stock.CompanyName,
			ExchangeID:  stock.ExchangeID,
			IsVN30:      stock.IsVN30,
			IsVN100:     stock.IsVN100,
			Sector:      stock.Sector,
			ListingDate: stock.ListingDate,
		}
	}
	return result, nil
}

func GetVN100Stocks() ([]modelsapi.StockDTO, error) {
	stocks, err := repository.GetSingleton().GetVN100Stocks()
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.StockDTO, len(stocks))
	for i, stock := range stocks {
		result[i] = modelsapi.StockDTO{
			ID:          stock.ID,
			Symbol:      stock.Symbol,
			CompanyName: stock.CompanyName,
			ExchangeID:  stock.ExchangeID,
			IsVN30:      stock.IsVN30,
			IsVN100:     stock.IsVN100,
			Sector:      stock.Sector,
			ListingDate: stock.ListingDate,
		}
	}
	return result, nil
}

func GetStocksByExchange(exchangeID uint) ([]modelsapi.StockDTO, error) {
	stocks, err := repository.GetSingleton().GetStocksByExchange(exchangeID)
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.StockDTO, len(stocks))
	for i, stock := range stocks {
		result[i] = modelsapi.StockDTO{
			ID:          stock.ID,
			Symbol:      stock.Symbol,
			CompanyName: stock.CompanyName,
			ExchangeID:  stock.ExchangeID,
			IsVN30:      stock.IsVN30,
			IsVN100:     stock.IsVN100,
			Sector:      stock.Sector,
			ListingDate: stock.ListingDate,
		}
	}
	return result, nil
}

func GetStocksBySector(sector string) ([]modelsapi.StockDTO, error) {
	stocks, err := repository.GetSingleton().GetStocksBySector(sector)
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.StockDTO, len(stocks))
	for i, stock := range stocks {
		result[i] = modelsapi.StockDTO{
			ID:          stock.ID,
			Symbol:      stock.Symbol,
			CompanyName: stock.CompanyName,
			ExchangeID:  stock.ExchangeID,
			IsVN30:      stock.IsVN30,
			IsVN100:     stock.IsVN100,
			Sector:      stock.Sector,
			ListingDate: stock.ListingDate,
		}
	}
	return result, nil
}

func CreateStock(req modelsapi.CreateStockRequest) (*modelsapi.StockDTO, error) {
	stock := &modelsdb.Stock{
		Symbol:      req.Symbol,
		CompanyName: req.CompanyName,
		ExchangeID:  req.ExchangeID,
		IsVN30:      req.IsVN30,
		IsVN100:     req.IsVN100,
		Sector:      req.Sector,
		ListingDate: req.ListingDate,
	}

	err := repository.GetSingleton().CreateStock(stock)
	if err != nil {
		return nil, err
	}

	return &modelsapi.StockDTO{
		ID:          stock.ID,
		Symbol:      stock.Symbol,
		CompanyName: stock.CompanyName,
		ExchangeID:  stock.ExchangeID,
		IsVN30:      stock.IsVN30,
		IsVN100:     stock.IsVN100,
		Sector:      stock.Sector,
		ListingDate: stock.ListingDate,
	}, nil
}

func UpdateStock(id uint, req modelsapi.UpdateStockRequest) (*modelsapi.StockDTO, error) {
	stock, err := repository.GetSingleton().GetStockByID(id)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.CompanyName != nil {
		stock.CompanyName = *req.CompanyName
	}
	if req.IsVN30 != nil {
		stock.IsVN30 = *req.IsVN30
	}
	if req.IsVN100 != nil {
		stock.IsVN100 = *req.IsVN100
	}
	if req.Sector != nil {
		stock.Sector = *req.Sector
	}
	if req.ListingDate != nil {
		stock.ListingDate = req.ListingDate
	}

	err = repository.GetSingleton().UpdateStock(stock)
	if err != nil {
		return nil, err
	}

	return &modelsapi.StockDTO{
		ID:          stock.ID,
		Symbol:      stock.Symbol,
		CompanyName: stock.CompanyName,
		ExchangeID:  stock.ExchangeID,
		IsVN30:      stock.IsVN30,
		IsVN100:     stock.IsVN100,
		Sector:      stock.Sector,
		ListingDate: stock.ListingDate,
	}, nil
}

func DeleteStock(id uint) error {
	return repository.GetSingleton().DeleteStock(id)
}
