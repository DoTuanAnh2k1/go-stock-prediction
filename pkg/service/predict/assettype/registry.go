package assettype

var types []AssetPredictionType

// Register adds an asset prediction type. Call from your package's init().
func Register(t AssetPredictionType) {
	types = append(types, t)
}

// All returns all registered asset prediction types.
func All() []AssetPredictionType {
	return types
}

// Get returns the type for the given key, or nil if not found.
func Get(key string) AssetPredictionType {
	for _, t := range types {
		if t.TypeKey() == key {
			return t
		}
	}
	return nil
}
