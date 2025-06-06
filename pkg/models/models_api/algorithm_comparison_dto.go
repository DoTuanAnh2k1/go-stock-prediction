package modelsapi

type AlgorithmComparisonDTO struct {
	Period     string                    `json:"period"`
	Algorithms []AlgorithmPerformanceDTO `json:"algorithms"`
	Winner     string                    `json:"best_algorithm"`
}
