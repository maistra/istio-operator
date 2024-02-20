package integration_operator

// Istio is a representation of the istio YAML file

type UpdateStrategy struct {
	Type                                       string `yaml:"type"`
	InactiveRevisionDeletionGracePeriodSeconds int    `yaml:"inactiveRevisionDeletionGracePeriodSeconds"`
}

type Requests struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
	// Add other optional fields here as needed
}

type Metadata struct {
	Name string `yaml:"name"`
}
type Resources struct {
	Requests *Requests `yaml:"requests,omitempty"`
	// Add other optional fields here as needed
}
type Pilot struct {
	Resources *Resources `yaml:"resources,omitempty"`
	// Add other optional fields here as needed
}
type RawValues struct {
	Pilot *Pilot `yaml:"pilot,omitempty"`
	// Add other optional fields here as needed
}

type Spec struct {
	Version        string          `yaml:"version"`
	Namespace      string          `yaml:"namespace"`
	UpdateStrategy *UpdateStrategy `yaml:"updateStrategy,omitempty"`
	RawValues      *RawValues      `yaml:"rawValues,omitempty"`
}

type Istio struct {
	ApiVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}
