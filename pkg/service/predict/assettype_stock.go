package predict

import (
	"context"
	"go-stock-prediction/pkg/service/predict/assettype"
)

type stockAssetType struct{}

func (s *stockAssetType) TypeKey() string  { return "stock" }
func (s *stockAssetType) TypeName() string { return "VN30 Stocks" }
func (s *stockAssetType) Init()            { Init() }
func (s *stockAssetType) RunPredictions(ctx context.Context) (int, error) {
	return 0, CronjobDailyPrediction()
}

func init() {
	assettype.Register(&stockAssetType{})
}
