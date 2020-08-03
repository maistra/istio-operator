package conversion

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

// XXX: Not all of the settings are mapped correctly, as there are differences
// between v1.0/v1.1 and v2.0

func populatePolicyValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	// Cluster settings
	if in.Policy == nil {
		return nil
	}

	istiod := !(in.Version == "" || in.Version == versions.V1_0.String() || in.Version == versions.V1_1.String())
	if in.Policy.Type == "" {
		if istiod {
			in.Policy.Type = v2.PolicyTypeIstiod
		} else {
			in.Policy.Type = v2.PolicyTypeMixer
		}
	}

	if err := setHelmStringValue(values, "policy.implementation", string(in.Policy.Type)); err != nil {
		return nil
	}

	if in.Policy.Type == v2.PolicyTypeNone {
		return setHelmBoolValue(values, "mixer.policy.enabled", false)
	}

	switch in.Policy.Type {
	case v2.PolicyTypeMixer:
		return populateMixerPolicyValues(in, istiod, values)
	case v2.PolicyTypeRemote:
		return populateRemotePolicyValues(in, values)
	case v2.PolicyTypeIstiod:
		return populateIstiodPolicyValues(in, values)
	}
	setHelmBoolValue(values, "mixer.policy.enabled", false)
	return fmt.Errorf("Unknown policy type: %s", in.Policy.Type)
}

func populateMixerPolicyValues(in *v2.ControlPlaneSpec, istiod bool, values map[string]interface{}) error {
	mixer := in.Policy.Mixer
	if mixer == nil {
		mixer = &v2.MixerPolicyConfig{}
	}

	// Make sure mixer is enabled
	if err := setHelmBoolValue(values, "mixer.enabled", true); err != nil {
		return err
	}

	policyValues := make(map[string]interface{})
	if err := setHelmBoolValue(policyValues, "enabled", true); err != nil {
		return err
	}
	if mixer.EnableChecks != nil {
		if err := setHelmBoolValue(values, "global.disablePolicyChecks", !*mixer.EnableChecks); err != nil {
			return err
		}
	}
	if mixer.FailOpen != nil {
		if err := setHelmBoolValue(values, "global.policyCheckFailOpen", *mixer.FailOpen); err != nil {
			return err
		}
	}

	if mixer.Adapters != nil {
		adaptersValues := make(map[string]interface{})
		if mixer.Adapters.UseAdapterCRDs != nil {
			if err := setHelmBoolValue(adaptersValues, "useAdapterCRDs", *mixer.Adapters.UseAdapterCRDs); err != nil {
				return err
			}
		}
		if mixer.Adapters.KubernetesEnv != nil {
			if err := setHelmBoolValue(adaptersValues, "kubernetesenv.enabled", *mixer.Adapters.KubernetesEnv); err != nil {
				return err
			}
		}
		if len(adaptersValues) > 0 {
			if istiod {
				if err := setHelmValue(policyValues, "adapters", adaptersValues); err != nil {
					return err
				}
			} else {
				if err := setHelmValue(values, "mixer.adapters", adaptersValues); err != nil {
					return err
				}
			}
		}
	}

	// Deployment specific settings
	runtime := mixer.Runtime
	if runtime != nil {
		if err := populateRuntimeValues(runtime, policyValues); err != nil {
			return err
		}

		// set image and resources
		if runtime.Pod.Containers != nil {
			// Mixer container specific config
			if mixerContainer, ok := runtime.Pod.Containers["mixer"]; ok {
				if err := populateContainerConfigValues(&mixerContainer, policyValues); err != nil {
					return err
				}
			}
		}
	}

	if !istiod {
		// move image, podAnnotations, nodeSelector, podAntiAffinityLabelSelector, and
		// podAntiAffinityTermLabelSelector from mixer.policy to mixer for v1.0 and v1.1
		// Note, these may overwrite settings specified in telemetry
		if image, found, _ := unstructured.NestedString(policyValues, "image"); found {
			if err := setHelmValue(values, "mixer.image", image); err != nil {
				return err
			}
		}
		if podAnnotations, found, _ := unstructured.NestedFieldCopy(policyValues, "podAnnotations"); found {
			if err := setHelmValue(values, "mixer.podAnnotations", podAnnotations); err != nil {
				return err
			}
		}
		if nodeSelector, found, _ := unstructured.NestedFieldCopy(policyValues, "nodeSelector"); found {
			if err := setHelmValue(values, "mixer.nodeSelector", nodeSelector); err != nil {
				return err
			}
		}
		if podAntiAffinityLabelSelector, found, _ := unstructured.NestedFieldCopy(policyValues, "podAntiAffinityLabelSelector"); found {
			if err := setHelmValue(values, "mixer.podAntiAffinityLabelSelector", podAntiAffinityLabelSelector); err != nil {
				return err
			}
		}
		if podAntiAffinityTermLabelSelector, found, _ := unstructured.NestedFieldCopy(policyValues, "podAntiAffinityTermLabelSelector"); found {
			if err := setHelmValue(values, "mixer.podAntiAffinityTermLabelSelector", podAntiAffinityTermLabelSelector); err != nil {
				return err
			}
		}
	}

	// set the policy values
	if len(policyValues) > 0 {
		if err := setHelmValue(values, "mixer.policy", policyValues); err != nil {
			return err
		}
	}

	return nil
}

func populateRemotePolicyValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	remote := in.Policy.Remote
	if remote == nil {
		remote = &v2.RemotePolicyConfig{}
	}

	// Make sure mixer is disabled
	if err := setHelmBoolValue(values, "mixer.enabled", false); err != nil {
		return err
	}
	if err := setHelmBoolValue(values, "mixer.policy.enabled", true); err != nil {
		return err
	}

	if err := setHelmStringValue(values, "global.remotePolicyAddress", remote.Address); err != nil {
		return err
	}
	// XXX: this applies to both policy and telemetry
	if err := setHelmBoolValue(values, "global.createRemoteSvcEndpoints", remote.CreateService); err != nil {
		return err
	}
	if remote.EnableChecks != nil {
		if err := setHelmBoolValue(values, "global.disablePolicyChecks", !*remote.EnableChecks); err != nil {
			return err
		}
	}
	if remote.FailOpen != nil {
		if err := setHelmBoolValue(values, "global.policyCheckFailOpen", *remote.FailOpen); err != nil {
			return err
		}
	}

	return nil
}

func populateIstiodPolicyValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	if err := setHelmBoolValue(values, "mixer.enabled", false); err != nil {
		return err
	}
	if err := setHelmBoolValue(values, "mixer.policy.enabled", false); err != nil {
		return err
	}
	return nil
}

func populatePolicyConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec, version versions.Version) error {
	var policyType v2.PolicyType
	if policyTypeStr, ok, err := in.GetString("policy.implementation"); ok && policyTypeStr != "" {
		switch v2.PolicyType(policyTypeStr) {
		case v2.PolicyTypeIstiod:
			policyType = v2.PolicyTypeIstiod
		case v2.PolicyTypeMixer:
			policyType = v2.PolicyTypeMixer
		case v2.PolicyTypeRemote:
			policyType = v2.PolicyTypeRemote
		case v2.PolicyTypeNone:
			policyType = v2.PolicyTypeNone
		default:
			return fmt.Errorf("unkown policy.implementation specified: %s", policyTypeStr)
		}
	} else if err != nil {
		return err
	} else {
		// figure out what we're installing
		if mixerPolicyEnabled, mixerPolicyEnabledSet, err := in.GetBool("mixer.policy.enabled"); err == nil {
			// installing some form of mixer based policy
			if mixerEnabled, mixerEnabledSet, err := in.GetBool("mixer.enabled"); err == nil {
				if !mixerEnabledSet || !mixerPolicyEnabledSet {
					// assume no policy to configure
					return nil
				}
				if mixerEnabled {
					if mixerPolicyEnabled {
						// installing mixer policy
						policyType = v2.PolicyTypeMixer
					} else {
						// mixer policy disabled
						policyType = v2.PolicyTypeNone
					}
				} else if mixerPolicyEnabled {
					// using remote mixer policy
					policyType = v2.PolicyTypeRemote
				} else {
					switch version {
					case versions.V1_0, versions.V1_1:
						// policy disabled
						policyType = v2.PolicyTypeNone
					case versions.V2_0:
						// assume istiod
						policyType = v2.PolicyTypeIstiod
					default:
						return fmt.Errorf("unknown version: %s", version.String())
					}
				}
			} else {
				return err
			}
		} else {
			return err
		}
	}

	if policyType == "" {
		return fmt.Errorf("Could not determine policy type")
	}
	out.Policy = &v2.PolicyConfig{
		Type: policyType,
	}
	switch policyType {
	case v2.PolicyTypeIstiod:
		// no configuration to set
	case v2.PolicyTypeMixer:
		config := &v2.MixerPolicyConfig{}
		if applied, err := populateMixerPolicyConfig(in, config); err != nil {
			return err
		} else if applied {
			out.Policy.Mixer = config
		}
	case v2.PolicyTypeRemote:
		config := &v2.RemotePolicyConfig{}
		if applied, err := populateRemotePolicyConfig(in, config); err != nil {
			return err
		} else if applied {
			out.Policy.Remote = config
		}
	case v2.PolicyTypeNone:
		// no configuration to set
	}

	return nil
}

func populateMixerPolicyConfig(in *v1.HelmValues, out *v2.MixerPolicyConfig) (bool, error) {
	setValues := false

	rawMixerValues, ok, err := in.GetMap("mixer")
	if err != nil {
		return false, err
	} else if !ok || len(rawMixerValues) == 0 {
		rawMixerValues = make(map[string]interface{})
	}
	mixerValues := v1.NewHelmValues(rawMixerValues)

	rawPolicyValues, ok, err := mixerValues.GetMap("policy")
	if err != nil {
		return false, err
	} else if !ok || len(rawPolicyValues) == 0 {
		rawPolicyValues = make(map[string]interface{})
	}
	policyValues := v1.NewHelmValues(rawPolicyValues)

	if disablePolicyChecks, ok, err := in.GetBool("global.disablePolicyChecks"); ok {
		enablePolicyChecks := !disablePolicyChecks
		out.EnableChecks = &enablePolicyChecks
		setValues = true
	} else if err != nil {
		return false, err
	}
	if policyCheckFailOpen, ok, err := in.GetBool("global.policyCheckFailOpen"); ok {
		out.FailOpen = &policyCheckFailOpen
		setValues = true
	} else if err != nil {
		return false, err
	}

	var adaptersValues *v1.HelmValues
	// check policy first, as mixer values are used with telemetry
	if rawAdaptersValues, ok, err := policyValues.GetMap("adapters"); ok {
		adaptersValues = v1.NewHelmValues(rawAdaptersValues)
	} else if err != nil {
		return false, err
	} else if rawAdaptersValues, ok, err := mixerValues.GetMap("adapters"); ok {
		adaptersValues = v1.NewHelmValues(rawAdaptersValues)
	} else if err != nil {
		return false, err
	}

	if adaptersValues != nil {
		adapters := &v2.MixerPolicyAdaptersConfig{}
		setAdapters := false
		if useAdapterCRDs, ok, err := adaptersValues.GetBool("useAdapterCRDs"); ok {
			adapters.UseAdapterCRDs = &useAdapterCRDs
			setAdapters = true
		} else if err != nil {
			return false, err
		}
		if kubernetesenv, ok, err := adaptersValues.GetBool("kubernetesenv.enabled"); ok {
			adapters.KubernetesEnv = &kubernetesenv
			setAdapters = true
		} else if err != nil {
			return false, err
		}
		if setAdapters {
			out.Adapters = adapters
			setValues = true
		}
	}

	// Deployment specific settings
	runtime := &v2.ComponentRuntimeConfig{}
	// istiod
	if applied, err := runtimeValuesToComponentRuntimeConfig(policyValues, runtime); err != nil {
		return false, err
	} else if applied {
		out.Runtime = runtime
		setValues = true
	}
	// non-istiod (this will just overwrite whatever was previously written)
	if applied, err := runtimeValuesToComponentRuntimeConfig(mixerValues, runtime); err != nil {
		return false, err
	} else if applied {
		out.Runtime = runtime
		setValues = true
	}

	// Container
	container := v2.ContainerConfig{}
	// non-istiod
	if applied, err := populateContainerConfig(mixerValues, &container); err != nil {
		return false, err
	} else if applied {
		if out.Runtime == nil {
			out.Runtime = runtime
			runtime.Pod.Containers = make(map[string]v2.ContainerConfig)
		} else if runtime.Pod.Containers == nil {
			runtime.Pod.Containers = make(map[string]v2.ContainerConfig)
		}
		out.Runtime.Pod.Containers["mixer"] = container
		setValues = true
	}
	// istiod (this will just overwrite whatever was previously written)
	if applied, err := populateContainerConfig(policyValues, &container); err != nil {
		return false, err
	} else if applied {
		if out.Runtime == nil {
			out.Runtime = runtime
			runtime.Pod.Containers = make(map[string]v2.ContainerConfig)
		} else if runtime.Pod.Containers == nil {
			runtime.Pod.Containers = make(map[string]v2.ContainerConfig)
		}
		out.Runtime.Pod.Containers["mixer"] = container
		setValues = true
	}

	return setValues, nil
}

func populateRemotePolicyConfig(in *v1.HelmValues, out *v2.RemotePolicyConfig) (bool, error) {
	setValues := false

	if remotePolicyAddress, ok, err := in.GetString("global.remotePolicyAddress"); ok {
		out.Address = remotePolicyAddress
		setValues = true
	} else if err != nil {
		return false, err
	}
	if createRemoteSvcEndpoints, ok, err := in.GetBool("global.createRemoteSvcEndpoints"); ok {
		out.CreateService = createRemoteSvcEndpoints
		setValues = true
	} else if err != nil {
		return false, err
	}
	if disablePolicyChecks, ok, err := in.GetBool("global.disablePolicyChecks"); ok {
		enableChecks := !disablePolicyChecks
		out.EnableChecks = &enableChecks
		setValues = true
	} else if err != nil {
		return false, err
	}
	if policyCheckFailOpen, ok, err := in.GetBool("global.policyCheckFailOpen"); ok {
		out.FailOpen = &policyCheckFailOpen
		setValues = true
	} else if err != nil {
		return false, err
	}

	return setValues, nil
}
