package ensemble

import (
	"context"
	"fmt"
	"go-stock-prediction/pkg/logger"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"go-stock-prediction/pkg/service/predict/iface"
)

type EnsembleAlgorithm struct {
	algorithms []iface.PredictionAlgorithm
	name       string
}

func New(algorithms []iface.PredictionAlgorithm) *EnsembleAlgorithm {
	return &EnsembleAlgorithm{
		algorithms: algorithms,
		name:       "Ensemble",
	}
}

func (e *EnsembleAlgorithm) GetName() string {
	return e.name
}

func (e *EnsembleAlgorithm) GetAccuracy() float64 {
	if len(e.algorithms) == 0 {
		return 0
	}
	total := 0.0
	count := 0
	for _, algo := range e.algorithms {
		acc := algo.GetAccuracy()
		if acc > 0 {
			total += acc
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func (e *EnsembleAlgorithm) Predict(ctx context.Context, data *modelssvc.StockData) (*modelssvc.Prediction, error) {
	if len(e.algorithms) == 0 {
		return nil, fmt.Errorf("ensemble has no algorithms")
	}

	type result struct {
		prediction *modelssvc.Prediction
		weight     float64
	}

	var results []result

	for _, algo := range e.algorithms {
		pred, err := algo.Predict(ctx, data)
		if err != nil {
			logger.Logger.Warnf("[ensemble] algorithm %s failed: %v, skipping", algo.GetName(), err)
			continue
		}
		weight := algo.GetAccuracy()
		results = append(results, result{prediction: pred, weight: weight})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("all ensemble algorithms failed")
	}

	totalWeight := 0.0
	for _, r := range results {
		totalWeight += r.weight
	}
	if totalWeight == 0 {
		totalWeight = float64(len(results))
		for i := range results {
			results[i].weight = 1.0
		}
	}

	weightedPredictedPrice := 0.0
	weightedCurrentPrice := 0.0
	weightedConfidence := 0.0

	for _, r := range results {
		w := r.weight / totalWeight
		weightedPredictedPrice += r.prediction.PredictedPrice * w
		weightedCurrentPrice += r.prediction.CurrentPrice * w
		weightedConfidence += r.prediction.Confidence * w
	}

	return &modelssvc.Prediction{
		PredictedPrice: weightedPredictedPrice,
		CurrentPrice:   weightedCurrentPrice,
		Confidence:     weightedConfidence,
		Symbol:         results[0].prediction.Symbol,
	}, nil
}
