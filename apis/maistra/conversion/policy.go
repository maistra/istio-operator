package conversion

import (
	"fmt"

	v1 "github.com/maistra/istio-operator/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/apis/maistra/v2"
	"github.com/maistra/istio-operator/controllers/versions"
)

// XXX: Not all of the settings are mapped correctly, as there are differences
// between v1.1 and v2.0

func populatePolicyValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	// Cluster settings
	if in.Policy == nil {
		return nil
	}

	if in.Policy.Type != "" {
		if err := setHelmStringValue(values, "policy.implementation", string(in.Policy.Type)); err != nil {
			return err
		}
	}

	switch in.Policy.Type {
	case v2.PolicyTypeNone:
		if err := setHelmBoolValue(values, "mixer.policy.enabled", false); err != nil {
			return err
		}
	case v2.PolicyTypeMixer:
		if err := setHelmBoolValue(values, "mixer.policy.enabled", true); err != nil {
			return err
		}
	case v2.PolicyTypeRemote:
		if err := setHelmBoolValue(values, "mixer.policy.enabled", false); err != nil {
			return err
		}
	case v2.PolicyTypeIstiod:
		if err := setHelmBoolValue(values, "mixer.policy.enabled", false); err != nil {
			return err
		}
	case "":
		// don't configure anything, let defaults take over
	}

	if err := populateMixerPolicyValues(in, true, values); err != nil {
		return err
	}
	if err := populateRemotePolicyValues(in, values); err != nil {
		return err
	}
	if err := populateIstiodPolicyValues(in, values); err != nil {
		return err
	}

	return nil
}

func populateMixerPolicyValues(in *v2.ControlPlaneSpec, istiod bool, values map[string]interface{}) error {
	mixer := in.Policy.Mixer
	if mixer == nil {
		mixer = &v2.MixerPolicyConfig{}
	}

	policyValues := make(map[string]interface{})
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
	if mixer.SessionAffinity != nil {
		if err := setHelmBoolValue(policyValues, "sessionAffinityEnabled", *mixer.SessionAffinity); err != nil {
			return err
		}
	}

	if mixer.Adapters != nil {
		policyAdaptersValues := make(map[string]interface{})
		if mixer.Adapters.UseAdapterCRDs != nil {
			if err := setHelmBoolValue(policyAdaptersValues, "useAdapterCRDs", *mixer.Adapters.UseAdapterCRDs); err != nil {
				return err
			}
		}
		if mixer.Adapters.KubernetesEnv != nil {
			if err := setHelmBoolValue(policyAdaptersValues, "kubernetesenv.enabled", *mixer.Adapters.KubernetesEnv); err != nil {
				return err
			}
		}
		if len(policyAdaptersValues) > 0 {
			if istiod {
				if err := setHelmValue(policyValues, "adapters", policyAdaptersValues); err != nil {
					return err
				}
			} else {
				if err := overwriteHelmValues(values, policyAdaptersValues, "mixer", "adapters"); err != nil {
					return err
				}
			}
		}
	}

	// set the policy values
	if len(policyValues) > 0 {
		if err := overwriteHelmValues(values, policyValues, "mixer", "policy"); err != nil {
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

	if remote.Address != "" {
		if err := setHelmStringValue(values, "global.remotePolicyAddress", remote.Address); err != nil {
			return err
		}
	}
	// XXX: this applies to both policy and telemetry
	if remote.CreateService != nil {
		if err := setHelmBoolValue(values, "global.createRemoteSvcEndpoints", *remote.CreateService); err != nil {
			return err
		}
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
	return nil
}

func populatePolicyConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec, version versions.Version) error {
	var policyType v2.PolicyType
	if policyTypeStr, ok, err := in.GetAndRemoveString("policy.implementation"); ok && policyTypeStr != "" {
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
			return fmt.Errorf("unknown policy.implementation specified: %s", policyTypeStr)
		}
	} else if err != nil {
		return err
	} else {
		// now it's complicated
		// we're converting from native v1 resource.  try to guess the type
		var mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled bool
		if mixerPolicyEnabled, mixerPolicyEnabledSet, err = in.GetAndRemoveBool("mixer.policy.enabled"); err != nil {
			return err
		}
		if remoteEnabled, _, err = in.GetBool("global.istioRemote"); err != nil {
			return err
		}
		// if mixer.policy.enabled is unset, assume version specific default
		policyType = version.Strategy().GetPolicyType(in, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled)
	}

	policy := &v2.PolicyConfig{}
	setPolicy := false
	if policyType != "" {
		setPolicy = true
		policy.Type = policyType
	}

	// some funky handling here, as some mixer settings are duplicated, so try
	// to get them in the right bucket
	mixer := &v2.MixerPolicyConfig{}
	remote := &v2.RemotePolicyConfig{}
	if policyType == v2.PolicyTypeRemote {
		if applied, err := populateRemotePolicyConfig(in, remote); err != nil {
			return err
		} else if applied {
			setPolicy = true
			policy.Remote = remote
		}
		if applied, err := populateMixerPolicyConfig(in, true, mixer); err != nil {
			return err
		} else if applied {
			setPolicy = true
			policy.Mixer = mixer
		}
	} else {
		if applied, err := populateMixerPolicyConfig(in, true, mixer); err != nil {
			return err
		} else if applied {
			setPolicy = true
			policy.Mixer = mixer
		}
		if applied, err := populateRemotePolicyConfig(in, remote); err != nil {
			return err
		} else if applied {
			setPolicy = true
			policy.Remote = remote
		}
	}

	if setPolicy {
		out.Policy = policy
	}

	// remove auto-populated values
	in.RemoveField("mixer.policy.enabled")

	return nil
}

func populateMixerPolicyConfig(in *v1.HelmValues, istiod bool, out *v2.MixerPolicyConfig) (bool, error) {
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

	if disablePolicyChecks, ok, err := in.GetAndRemoveBool("global.disablePolicyChecks"); ok {
		enablePolicyChecks := !disablePolicyChecks
		out.EnableChecks = &enablePolicyChecks
		setValues = true
	} else if err != nil {
		return false, err
	}
	if policyCheckFailOpen, ok, err := in.GetAndRemoveBool("global.policyCheckFailOpen"); ok {
		out.FailOpen = &policyCheckFailOpen
		setValues = true
	} else if err != nil {
		return false, err
	}
	if sessionAffinityEnabled, ok, err := policyValues.GetAndRemoveBool("sessionAffinityEnabled"); ok {
		out.SessionAffinity = &sessionAffinityEnabled
		setValues = true
	} else if err != nil {
		return false, nil
	}

	var policyAdaptersValues *v1.HelmValues
	if istiod {
		if rawAdaptersValues, ok, err := policyValues.GetMap("adapters"); ok {
			policyAdaptersValues = v1.NewHelmValues(rawAdaptersValues)
		} else if err != nil {
			return false, err
		}
	} else if rawAdaptersValues, ok, err := mixerValues.GetMap("adapters"); ok {
		policyAdaptersValues = v1.NewHelmValues(rawAdaptersValues)
	} else if err != nil {
		return false, err
	}

	if policyAdaptersValues != nil {
		adapters := &v2.MixerPolicyAdaptersConfig{}
		setAdapters := false
		if useAdapterCRDs, ok, err := policyAdaptersValues.GetBool("useAdapterCRDs"); ok {
			adapters.UseAdapterCRDs = &useAdapterCRDs
			setAdapters = true
		} else if err != nil {
			return false, err
		}
		if kubernetesenv, ok, err := policyAdaptersValues.GetBool("kubernetesenv.enabled"); ok {
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

	// update the mixer settings
	if istiod {
		policyAdaptersValues.RemoveField("useAdapterCRDs")
		policyAdaptersValues.RemoveField("kubernetesenv.enabled")
		if len(policyAdaptersValues.GetContent()) == 0 {
			policyValues.RemoveField("adapters")
		} else if err := policyValues.SetField("adapters", policyAdaptersValues.GetContent()); err != nil {
			return false, err
		}
	} else {
		if len(policyAdaptersValues.GetContent()) == 0 {
			mixerValues.RemoveField("adapters")
		} else if err := mixerValues.SetField("adapters", policyAdaptersValues.GetContent()); err != nil {
			return false, err
		}
	}
	if len(policyValues.GetContent()) == 0 {
		mixerValues.RemoveField("policy")
	} else if err := mixerValues.SetField("policy", policyValues.GetContent()); err != nil {
		return false, err
	}
	if len(mixerValues.GetContent()) == 0 {
		in.RemoveField("mixer")
	} else if err := in.SetField("mixer", mixerValues.GetContent()); err != nil {
		return false, err
	}

	return setValues, nil
}

func populateRemotePolicyConfig(in *v1.HelmValues, out *v2.RemotePolicyConfig) (bool, error) {
	setValues := false

	if remotePolicyAddress, ok, err := in.GetAndRemoveString("global.remotePolicyAddress"); ok && remotePolicyAddress != "" {
		out.Address = remotePolicyAddress
		setValues = true
	} else if err != nil {
		return false, err
	}
	if createRemoteSvcEndpoints, ok, err := in.GetBool("global.createRemoteSvcEndpoints"); ok {
		out.CreateService = &createRemoteSvcEndpoints
		setValues = true
	} else if err != nil {
		return false, err
	}
	if disablePolicyChecks, ok, err := in.GetAndRemoveBool("global.disablePolicyChecks"); ok {
		enableChecks := !disablePolicyChecks
		out.EnableChecks = &enableChecks
		setValues = true
	} else if err != nil {
		return false, err
	}
	if policyCheckFailOpen, ok, err := in.GetAndRemoveBool("global.policyCheckFailOpen"); ok {
		out.FailOpen = &policyCheckFailOpen
		setValues = true
	} else if err != nil {
		return false, err
	}

	return setValues, nil
}
