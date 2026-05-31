package goldpredict

import (
	"context"
	"go-stock-prediction/pkg/service/predict/assettype"
)

type goldAssetType struct{}

func (g *goldAssetType) TypeKey() string  { return "gold" }
func (g *goldAssetType) TypeName() string { return "Gold (SJC / XAU)" }
func (g *goldAssetType) Init()            { Init() }
func (g *goldAssetType) RunPredictions(ctx context.Context) (int, error) {
	return RunNow()
}

func init() {
	assettype.Register(&goldAssetType{})
}
