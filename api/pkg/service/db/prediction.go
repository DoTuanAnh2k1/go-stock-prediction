package db

import (
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
)

// ===============================
// PREDICTION SERVICES
// ===============================

func GetAllPredictions() ([]modelsapi.PredictionDTO, error) {
	predictions, err := repository.GetSingleton().GetAllPredictions()
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.PredictionDTO, len(predictions))
	for i, pred := range predictions {
		result[i] = modelsapi.PredictionDTO{
			ID:             pred.ID,
			StockID:        pred.StockID,
			PredictedPrice: pred.PredictedPrice,
			Confidence:     pred.Confidence,
			AlgorithmName:  pred.AlgorithmName,
			PredictionDate: pred.PredictionDate,
			TargetDate:     pred.TargetDate,
			ActualPrice:    pred.ActualPrice,
			Accuracy:       pred.Accuracy,
		}
	}
	return result, nil
}

func GetPredictionByID(id uint) (*modelsapi.PredictionDTO, error) {
	prediction, err := repository.GetSingleton().GetPredictionByID(id)
	if err != nil {
		return nil, err
	}

	return &modelsapi.PredictionDTO{
		ID:             prediction.ID,
		StockID:        prediction.StockID,
		PredictedPrice: prediction.PredictedPrice,
		Confidence:     prediction.Confidence,
		AlgorithmName:  prediction.AlgorithmName,
		PredictionDate: prediction.PredictionDate,
		TargetDate:     prediction.TargetDate,
		ActualPrice:    prediction.ActualPrice,
		Accuracy:       prediction.Accuracy,
	}, nil
}

func GetPredictionsByStockID(stockID uint) ([]modelsapi.PredictionDTO, error) {
	predictions, err := repository.GetSingleton().GetPredictionsByStockID(stockID)
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.PredictionDTO, len(predictions))
	for i, pred := range predictions {
		result[i] = modelsapi.PredictionDTO{
			ID:             pred.ID,
			StockID:        pred.StockID,
			PredictedPrice: pred.PredictedPrice,
			Confidence:     pred.Confidence,
			AlgorithmName:  pred.AlgorithmName,
			PredictionDate: pred.PredictionDate,
			TargetDate:     pred.TargetDate,
			ActualPrice:    pred.ActualPrice,
			Accuracy:       pred.Accuracy,
		}
	}
	return result, nil
}

func GetLatestPredictionsByStockID(stockID uint, limit int) ([]modelsapi.PredictionDTO, error) {
	predictions, err := repository.GetSingleton().GetLatestPredictionsByStockID(stockID, limit)
	if err != nil {
		return nil, err
	}

	result := make([]modelsapi.PredictionDTO, len(predictions))
	for i, pred := range predictions {
		result[i] = modelsapi.PredictionDTO{
			ID:             pred.ID,
			StockID:        pred.StockID,
			PredictedPrice: pred.PredictedPrice,
			Confidence:     pred.Confidence,
			AlgorithmName:  pred.AlgorithmName,
			PredictionDate: pred.PredictionDate,
			TargetDate:     pred.TargetDate,
			ActualPrice:    pred.ActualPrice,
			Accuracy:       pred.Accuracy,
		}
	}
	return result, nil
}

func CreatePrediction(req modelsapi.CreatePredictionRequest) (*modelsapi.PredictionDTO, error) {
	prediction := &modelsdb.Prediction{
		StockID:        req.StockID,
		PredictedPrice: req.PredictedPrice,
		Confidence:     req.Confidence,
		AlgorithmName:  req.AlgorithmName,
		PredictionDate: req.PredictionDate,
		TargetDate:     req.TargetDate,
	}

	err := repository.GetSingleton().CreatePrediction(prediction)
	if err != nil {
		return nil, err
	}

	return &modelsapi.PredictionDTO{
		ID:             prediction.ID,
		StockID:        prediction.StockID,
		PredictedPrice: prediction.PredictedPrice,
		Confidence:     prediction.Confidence,
		AlgorithmName:  prediction.AlgorithmName,
		PredictionDate: prediction.PredictionDate,
		TargetDate:     prediction.TargetDate,
		ActualPrice:    prediction.ActualPrice,
		Accuracy:       prediction.Accuracy,
	}, nil
}
