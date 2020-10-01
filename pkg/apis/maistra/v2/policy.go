package v2

// `PolicyConfig` configures policy aspects of the mesh.
type PolicyConfig struct {
	// `PolicyType` sets the policy implementation
	// defaults to Istiod 1.6+, Mixer pre-1.6. This field is required.
	Type PolicyType `json:"type,omitempty"`
	// Mixer configuration (legacy, v1)
	// `.Values.Mixer.policy.enabled`
	// +optional
	Mixer *MixerPolicyConfig `json:"Mixer,omitempty"`
	// Remote Mixer configuration (legacy, v1)
	// `.Values.global.remotePolicyAddress`
	// +optional
	Remote *RemotePolicyConfig `json:"remote,omitempty"`
}

// PolicyType represents the type of policy implementation used by the mesh.
type PolicyType string

const (
	// `PolicyTypeNone` represents disabling of policy
	PolicyTypeNone PolicyType = "None"
	// `PolicyTypeMixer` represents Mixer, v1 implementation
	PolicyTypeMixer PolicyType = "Mixer"
	// `PolicyTypeRemote` represents remote Mixer, v1 implementation
	PolicyTypeRemote PolicyType = "Remote"
	// `PolicyTypeIstiod` represents Istio, v2 implementation
	PolicyTypeIstiod PolicyType = "Istiod"
)

// `MixerPolicyConfig` configures a Mixer implementation for policy
// `.Values.Mixer.policy.enabled`
type MixerPolicyConfig struct {
	// `EnableChecks` configures whether or not policy checks should be enabled.
	// `.Values.global.disablePolicyChecks | default "true"` (false, inverted logic)
	// Set `EnableChecks` to false to disable policy checks by Mixer.
	// Note that metrics will still be reported to Mixer.
	// +optional
	EnableChecks *bool `json:"enableChecks,omitempty"`
	// `FailOpen` configures how policy checks fail when Mixer cannot be reached. Set this to true to allow traffic through when the Mixer policy service cannot be reached. Set it to false to to deny traffic when the Mixer policy server cannot be reached. The default is false.
	// `.Values.global.policyCheckFailOpen`, maps to MeshConfig.policyCheckFailOpen
	// +optional
	FailOpen *bool `json:"failOpen,omitempty"`
	// `SessionAffinity` configures session affinity for sidecar policy connections.
	// `.Values.Mixer.policy.sessionAffinityEnabled`
	// +optional
	SessionAffinity *bool `json:"sessionAffinity,omitempty"`
	// `Adapters` configures available adapters.
	// +optional
	Adapters *MixerPolicyAdaptersConfig `json:"adapters,omitempty"`
}

// `MixerPolicyAdaptersConfig configures policy adapters for Mixer.
type MixerPolicyAdaptersConfig struct {
	// `UseAdapterCRDs` configures Mixer to support deprecated Mixer CRDs.
	// `.Values.Mixer.policy.adapters.useAdapterCRDs`, removed in istio 1.4, defaults to false
	// Only supported in v1.0, where it defaulted to true
	// +optional
	UseAdapterCRDs *bool `json:"useAdapterCRDs,omitempty"`
	// `Kubernetesenv` configures the use of the `kubernetesenv` adapter. Defaults to true.
	// `.Values.Mixer.policy.adapters.kubernetesenv.enabled
	// +optional
	KubernetesEnv *bool `json:"kubernetesenv,omitempty"`
}

// `RemotePolicyConfig` configures a remote Mixer instance for policy
type RemotePolicyConfig struct {
	// `Address` represents the address of the Mixer server.
	// `.Values.global.remotePolicyAddress`, maps to `MeshConfig.MixerCheckServer`
	Address string `json:"address,omitempty"`
	// `CreateServices` specifies whether or not a Kubernetes Service should be created for the remote policy server.
	// `.Values.global.createRemoteSvcEndpoints`
	// +optional
	CreateService *bool `json:"createService,omitempty"`
	// `EnableChecks` configures whether or not policy checks should be enabled.
	// Set `EnableChecks` to false to disable policy checks by Mixer.
	// Note that metrics will still be reported to Mixer.
	// `.Values.global.disablePolicyChecks | default "true"` (false, inverted logic)
	// +optional
	EnableChecks *bool `json:"enableChecks,omitempty"`
	// `FailOpen` configures how policy checks behave if Mixer cannot be reached.
	// Setting `FailOpen` to true allows traffic through the proxy when Mixer cannot be reached. Setting `FailOpen` to false denies traffic when Mixer cannot be reached. Defaults to false.
	// `.Values.global.policyCheckFailOpen`, maps to `MeshConfig.policyCheckFailOpen`
	// +optional
	FailOpen *bool `json:"failOpen,omitempty"`
}
