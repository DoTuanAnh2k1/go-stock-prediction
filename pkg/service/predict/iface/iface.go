package iface

import (
	"context"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
)

type PredictionAlgorithm interface {
	Predict(ctx context.Context, data *modelssvc.StockData) (*modelssvc.Prediction, error)
	GetName() string
	GetAccuracy() float64
}
