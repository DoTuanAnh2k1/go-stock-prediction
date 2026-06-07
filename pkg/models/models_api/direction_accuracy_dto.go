package modelsapi

// DirectionAccuracyRow holds per-algorithm direction accuracy stats for a given market.
type DirectionAccuracyRow struct {
	Algorithm string
	Total     int64
	Correct   int64
}
