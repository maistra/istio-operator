package v2

// TechPreviews contains switches for features that are not GA yet.
type TechPreviewsConfig struct {
	// Metrics configures metrics storage solutions for the mesh.
	// +optional
	WasmExtensions *WasmExtensionsConfig `json:"wasmExtensions,omitempty"`
}

// WasmExtensionsConfig configures ServiceMeshExtension support
type WasmExtensionsConfig struct {
	Enablement `json:",inline"`
}
