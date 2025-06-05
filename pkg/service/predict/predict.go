package predict

import (
	"context"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"log"
)

// Domain interfaces theo pattern Import -> Calculate -> Export
type DataImporter interface {
	Import(ctx context.Context, symbol string) (*modelssvc.StockData, error)
	ImportBatch(ctx context.Context, symbols []string) ([]*modelssvc.StockData, error)
}

type PredictionAlgorithm interface {
	Predict(ctx context.Context, data *modelssvc.StockData) (*modelssvc.Prediction, error)
	GetName() string
	GetAccuracy() float64 // Độ chính xác của thuật toán
}

type DataExporter interface {
	Export(ctx context.Context, prediction *modelssvc.Prediction) error
	ExportBatch(ctx context.Context, predictions []*modelssvc.Prediction) error
}

// Service implement pipeline chính
type StockService struct {
	importer  DataImporter
	predictor PredictionAlgorithm
	exporter  DataExporter
	logger    *log.Logger
}

func (s *StockService) ProcessStock(ctx context.Context, symbol string) (*modelssvc.Prediction, error) {
	// Bước 1: Import data
	data, err := s.importer.Import(ctx, symbol)
	if err != nil {
		s.logger.Printf("Lỗi import data cho %s: %v", symbol, err)
		return nil, err
	}

	// Bước 2: Calculate prediction
	prediction, err := s.predictor.Predict(ctx, data)
	if err != nil {
		s.logger.Printf("Lỗi predict cho %s: %v", symbol, err)
		return nil, err
	}

	// Bước 3: Export results
	if err := s.exporter.Export(ctx, prediction); err != nil {
		s.logger.Printf("Lỗi export cho %s: %v", symbol, err)
		return nil, err
	}

	s.logger.Printf("Thành công xử lý %s: giá dự đoán %.2f", symbol, prediction.PredictedPrice)
	return prediction, nil
}
