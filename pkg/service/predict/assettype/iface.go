// Package assettype defines the extensibility point for adding new categories
// of assets to predict (stocks, gold, crypto, forex, etc.).
//
// To add a new asset type (e.g. crypto):
//  1. Create pkg/service/predict/crypto/ with the prediction logic
//  2. Implement AssetPredictionType in that package
//  3. Call assettype.Register() from an init() function in the package
//  4. Add blank import in cmd/prediction/main.go: _ "go-stock-prediction/pkg/service/predict/crypto"
//  5. Add DB model + repository methods + gRPC TriggerCryptoPredict RPC
//  6. Add API routes in pkg/server/api_trigger_crypto_predict.go
//  7. Add frontend page frontend/src/pages/Crypto.tsx + route in App.tsx
package assettype

import "context"

// AssetPredictionType represents a category of assets that can be predicted.
type AssetPredictionType interface {
	// TypeKey returns the unique identifier (e.g. "stock", "gold", "crypto").
	TypeKey() string
	// TypeName returns a human-readable display name.
	TypeName() string
	// Init registers cron jobs and startup tasks for this asset type.
	Init()
	// RunPredictions triggers an immediate prediction run for all assets of this type.
	// Returns the number of predictions saved and any error.
	RunPredictions(ctx context.Context) (int, error)
}
