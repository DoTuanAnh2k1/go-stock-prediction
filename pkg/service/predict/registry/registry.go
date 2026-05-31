package registry

// AlgorithmDef holds metadata about a prediction algorithm.
// Factory fields have been removed — prediction is now handled by the Python service.
// This registry is kept for API metadata purposes only (/api/training/algorithms).
type AlgorithmDef struct {
	Key         string
	DisplayName string
	Config      map[string]interface{}
	IsComposite bool
}

var defs []AlgorithmDef

// Register adds an algorithm definition. Called from algorithms.go init().
func Register(d AlgorithmDef) {
	defs = append(defs, d)
}

// All returns all registered algorithm definitions.
func All() []AlgorithmDef {
	return defs
}
