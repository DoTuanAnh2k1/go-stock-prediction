package registry

import "go-stock-prediction/pkg/service/predict/iface"

// AlgorithmDef holds all information needed to register and use a prediction algorithm.
type AlgorithmDef struct {
	Key         string
	DisplayName string
	Config      map[string]interface{}
	// IsComposite marks algorithms that require base algorithm instances (e.g. Ensemble).
	IsComposite bool
	// Factory creates a fresh instance of this algorithm.
	Factory func() iface.PredictionAlgorithm
	// CompositeFactory receives the already-built base algorithms map.
	// Only used when IsComposite is true.
	CompositeFactory func(bases map[string]iface.PredictionAlgorithm) iface.PredictionAlgorithm
}

var defs []AlgorithmDef

// Register adds an algorithm definition. Call this from algorithms.go init().
func Register(d AlgorithmDef) {
	defs = append(defs, d)
}

// All returns all registered algorithm definitions.
func All() []AlgorithmDef {
	return defs
}

// Build instantiates all algorithms. Base algorithms are built first;
// composite algorithms (like Ensemble) receive the base instances map.
func Build() map[string]iface.PredictionAlgorithm {
	m := make(map[string]iface.PredictionAlgorithm, len(defs))
	// First pass: base algorithms
	for _, d := range defs {
		if !d.IsComposite {
			m[d.Key] = d.Factory()
		}
	}
	// Second pass: composites receive the base map
	for _, d := range defs {
		if d.IsComposite && d.CompositeFactory != nil {
			m[d.Key] = d.CompositeFactory(m)
		}
	}
	return m
}
