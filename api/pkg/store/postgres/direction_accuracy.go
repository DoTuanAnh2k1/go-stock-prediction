package postgres

import (
	"fmt"
	"strings"

	modelsapi "go-stock-prediction/pkg/models/models_api"
)

// marketTableMap maps canonical market keys (upper-case) to the corresponding
// prediction table name and the column used as the "algorithm" identifier.
var marketTableMap = map[string]struct {
	table   string
	algoCol string
}{
	"GOLD":   {table: "gold_predictions", algoCol: "algorithm_name"},
	"NASDAQ": {table: "nasdaq_predictions", algoCol: "algorithm_name"},
	"CRYPTO": {table: "crypto_predictions", algoCol: "algorithm_name"},
	"SP500":  {table: "sp500_predictions", algoCol: "algorithm_name"},
}

// GetDirectionAccuracy returns per-algorithm direction accuracy stats for the given market.
// Only rows where direction_correct IS NOT NULL are included in the totals.
func (c *Client) GetDirectionAccuracy(market string) ([]modelsapi.DirectionAccuracyRow, error) {
	info, ok := marketTableMap[strings.ToUpper(market)]
	if !ok {
		return nil, fmt.Errorf("unknown market: %q (valid: GOLD, NASDAQ, CRYPTO, SP500)", market)
	}

	// Raw SQL to avoid GORM model binding — the query shape is identical for every table.
	// PostgreSQL uses FILTER syntax for conditional aggregation instead of CASE WHEN.
	query := fmt.Sprintf(`
		SELECT
			%s AS algorithm,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE direction_correct = true) AS correct
		FROM %s
		WHERE direction_correct IS NOT NULL
		  AND deleted_at IS NULL
		GROUP BY %s
		ORDER BY %s
	`, info.algoCol, info.table, info.algoCol, info.algoCol)

	type rawRow struct {
		Algorithm string
		Total     int64
		Correct   int64
	}

	var rows []rawRow
	if err := c.Db.Raw(query).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("GetDirectionAccuracy(%s): %w", market, err)
	}

	result := make([]modelsapi.DirectionAccuracyRow, len(rows))
	for i, r := range rows {
		result[i] = modelsapi.DirectionAccuracyRow{
			Algorithm: r.Algorithm,
			Total:     r.Total,
			Correct:   r.Correct,
		}
	}
	return result, nil
}
