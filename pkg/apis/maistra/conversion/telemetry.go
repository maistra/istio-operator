package conversion

import (
	"fmt"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func populateTelemetryValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	telemetry := in.Telemetry
	if telemetry == nil {
		return nil
	}

	istiod := !(in.Version == "" || in.Version == versions.V1_0.String() || in.Version == versions.V1_1.String())

	if in.Telemetry.Type != "" {
		if err := setHelmStringValue(values, "telemetry.implementation", string(in.Telemetry.Type)); err != nil {
			return nil
		}
	}

	switch in.Telemetry.Type {
	case v2.TelemetryTypeNone:
		if istiod {
			if err := setHelmBoolValue(values, "telemetry.enabled", false); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v1.enabled", false); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v2.enabled", false); err != nil {
				return err
			}
		} else {
			if err := setHelmBoolValue(values, "global.istioRemote", false); err != nil {
				return err
			}
		}
		if err := setHelmBoolValue(values, "mixer.telemetry.enabled", false); err != nil {
			return err
		}
	case v2.TelemetryTypeMixer:
		if istiod {
			if err := setHelmBoolValue(values, "telemetry.enabled", true); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v1.enabled", true); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v2.enabled", false); err != nil {
				return err
			}
		} else {
			if err := setHelmBoolValue(values, "global.istioRemote", false); err != nil {
				return err
			}
		}
		if err := setHelmBoolValue(values, "mixer.telemetry.enabled", true); err != nil {
			return err
		}
	case v2.TelemetryTypeRemote:
		if istiod {
			if err := setHelmBoolValue(values, "telemetry.enabled", true); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v1.enabled", true); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v2.enabled", false); err != nil {
				return err
			}
		} else {
			if err := setHelmBoolValue(values, "global.istioRemote", true); err != nil {
				return err
			}
		}
		if err := setHelmBoolValue(values, "mixer.telemetry.enabled", false); err != nil {
			return err
		}
	case v2.TelemetryTypeIstiod:
		if istiod {
			if err := setHelmBoolValue(values, "telemetry.enabled", true); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v1.enabled", false); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v2.enabled", true); err != nil {
				return err
			}
		} else {
			if err := setHelmBoolValue(values, "global.istioRemote", false); err != nil {
				return err
			}
		}
		if err := setHelmBoolValue(values, "mixer.telemetry.enabled", false); err != nil {
			return err
		}
	case "":
		// don't configure anything, let defaults take over
	}

	if err := populateMixerTelemetryValues(in, istiod, values); err != nil {
		return err
	}
	if err := populateRemoteTelemetryValues(in, istiod, values); err != nil {
		return err
	}
	if err := populateIstiodTelemetryValues(in, values); err != nil {
		return err
	}

	return nil
}

func populateMixerTelemetryValues(in *v2.ControlPlaneSpec, istiod bool, values map[string]interface{}) error {
	mixer := in.Telemetry.Mixer
	if mixer == nil {
		mixer = &v2.MixerTelemetryConfig{}
	}

	v1TelemetryValues := make(map[string]interface{})
	if err := populateTelemetryBatchingValues(mixer.Batching, v1TelemetryValues); err != nil {
		return nil
	}

	if mixer.SessionAffinity != nil {
		if err := setHelmBoolValue(v1TelemetryValues, "sessionAffinityEnabled", *mixer.SessionAffinity); err != nil {
			return err
		}
	}

	if mixer.Loadshedding != nil {
		if mixer.Loadshedding.Mode != "" {
			if err := setHelmStringValue(v1TelemetryValues, "loadshedding.mode", mixer.Loadshedding.Mode); err != nil {
				return err
			}
		}
		if mixer.Loadshedding.LatencyThreshold != "" {
			if err := setHelmStringValue(v1TelemetryValues, "loadshedding.latencyThreshold", mixer.Loadshedding.LatencyThreshold); err != nil {
				return err
			}
		}
	}

	if mixer.Adapters != nil {
		telemetryAdaptersValues := make(map[string]interface{})
		if mixer.Adapters.UseAdapterCRDs != nil {
			if err := setHelmBoolValue(telemetryAdaptersValues, "useAdapterCRDs", *mixer.Adapters.UseAdapterCRDs); err != nil {
				return err
			}
		}
		if mixer.Adapters.KubernetesEnv != nil {
			if err := setHelmBoolValue(telemetryAdaptersValues, "kubernetesenv.enabled", *mixer.Adapters.KubernetesEnv); err != nil {
				return err
			}
		}
		if mixer.Adapters.Stdio != nil {
			if mixer.Adapters.Stdio.Enabled != nil {
				if err := setHelmBoolValue(telemetryAdaptersValues, "stdio.enabled", *mixer.Adapters.Stdio.Enabled); err != nil {
					return err
				}
			}
			if mixer.Adapters.Stdio.OutputAsJSON != nil {
				if err := setHelmBoolValue(telemetryAdaptersValues, "stdio.outputAsJson", *mixer.Adapters.Stdio.OutputAsJSON); err != nil {
					return err
				}
			}
		}
		if len(telemetryAdaptersValues) > 0 {
			if err := overwriteHelmValues(values, telemetryAdaptersValues, "mixer", "adapters"); err != nil {
				return err
			}
		}
	}

	// set the telemetry values
	if len(v1TelemetryValues) > 0 {
		if err := overwriteHelmValues(values, v1TelemetryValues, "mixer", "telemetry"); err != nil {
			return err
		}
	}

	return nil
}

func populateTelemetryBatchingValues(in *v2.TelemetryBatchingConfig, telemetryBatchingValues map[string]interface{}) error {
	if in == nil {
		return nil
	}
	if in.MaxTime != "" {
		if err := setHelmStringValue(telemetryBatchingValues, "reportBatchMaxTime", in.MaxTime); err != nil {
			return err
		}
	}
	if in.MaxEntries != nil {
		return setHelmIntValue(telemetryBatchingValues, "reportBatchMaxEntries", int64(*in.MaxEntries))
	}
	return nil
}

func populateRemoteTelemetryValues(in *v2.ControlPlaneSpec, istiod bool, values map[string]interface{}) error {
	remote := in.Telemetry.Remote
	if remote == nil {
		remote = &v2.RemoteTelemetryConfig{}
	}

	if remote.Address != "" {
		if err := setHelmStringValue(values, "global.remoteTelemetryAddress", remote.Address); err != nil {
			return err
		}
	}
	// XXX: this applies to both policy and telemetry
	if remote.CreateService != nil {
		if err := setHelmBoolValue(values, "global.createRemoteSvcEndpoints", *remote.CreateService); err != nil {
			return err
		}
	}

	v1TelemetryValues := make(map[string]interface{})
	if err := populateTelemetryBatchingValues(remote.Batching, v1TelemetryValues); err != nil {
		return nil
	}

	// set the telemetry values
	if len(v1TelemetryValues) > 0 {
		if err := overwriteHelmValues(values, v1TelemetryValues, "mixer", "telemetry"); err != nil {
			return err
		}
	}

	return nil
}

func populateIstiodTelemetryValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	return nil
}

func populateTelemetryConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec, version versions.Version) error {
	var telemetryType v2.TelemetryType
	if telemetryTypeStr, ok, err := in.GetAndRemoveString("telemetry.implementation"); ok && telemetryTypeStr != "" {
		switch v2.TelemetryType(telemetryTypeStr) {
		case v2.TelemetryTypeIstiod:
			telemetryType = v2.TelemetryTypeIstiod
		case v2.TelemetryTypeMixer:
			telemetryType = v2.TelemetryTypeMixer
		case v2.TelemetryTypeRemote:
			telemetryType = v2.TelemetryTypeRemote
		case v2.TelemetryTypeNone:
			telemetryType = v2.TelemetryTypeNone
		default:
			return fmt.Errorf("unkown telemetry.implementation specified: %s", telemetryTypeStr)
		}
	} else if err != nil {
		return err
	} else {
		// now it's complicated
		// we're converting from native v1 resource.  try to guess the type
		var mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled bool
		if mixerTelemetryEnabled, mixerTelemetryEnabledSet, err = in.GetAndRemoveBool("mixer.telemetry.enabled"); err != nil {
			return err
		}
		if remoteEnabled, _, err = in.GetBool("global.istioRemote"); err != nil {
			return err
		}
		// if mixer.telemetry.enabled is unset, assume version specific default
		switch version {
		case versions.V1_0:
			fallthrough
		case versions.V1_1:
			if remoteEnabled {
				// using remote telemetry, which takes precedence over mixer (in the charts, at least)
				telemetryType = v2.TelemetryTypeRemote
			} else if mixerTelemetryEnabled {
				// mixer telemetry explicitly enabled
				telemetryType = v2.TelemetryTypeMixer
			} else if mixerTelemetryEnabledSet {
				// mixer is explicitly disabled
				telemetryType = v2.TelemetryTypeNone
			} else {
				// don't set telemetry type, let the defaults do their thing
				telemetryType = ""
			}
		case versions.V2_0:
			remoteTelemetryAddress, _, _ := in.GetString("global.remoteTelemetryAddress")
			if remoteEnabled || remoteTelemetryAddress != "" {
				// special case if copying over an old v1 resource an bumping the version to v2.0
				telemetryType = v2.TelemetryTypeRemote
			} else {
				// leave the defaults
				telemetryType = ""
			}
		default:
			return fmt.Errorf("Unknown version")
		}
	}

	telemetry := &v2.TelemetryConfig{}
	setTelemetry := false
	if telemetryType != "" {
		setTelemetry = true
		telemetry.Type = telemetryType
	}

	// some funky handling here, as some mixer settings are duplicated, so try
	// to get them in the right bucket
	remote := &v2.RemoteTelemetryConfig{}
	mixer := &v2.MixerTelemetryConfig{}
	if telemetryType == v2.TelemetryTypeRemote {
		if applied, err := populateRemoteTelemetryConfig(in, remote); err != nil {
			return err
		} else if applied {
			setTelemetry = true
			telemetry.Remote = remote
		}
		if applied, err := populateMixerTelemetryConfig(in, mixer); err != nil {
			return err
		} else if applied {
			setTelemetry = true
			telemetry.Mixer = mixer
		}
	} else {
		if applied, err := populateMixerTelemetryConfig(in, mixer); err != nil {
			return err
		} else if applied {
			setTelemetry = true
			telemetry.Mixer = mixer
		}
		if applied, err := populateRemoteTelemetryConfig(in, remote); err != nil {
			return err
		} else if applied {
			setTelemetry = true
			telemetry.Remote = remote
		}
	}

	// remove auto-populated values
	in.RemoveField("mixer.telemetry.enabled")
	in.RemoveField("telemetry.enabled")
	in.RemoveField("telemetry.v1.enabled")
	in.RemoveField("telemetry.v2.enabled")

	if setTelemetry {
		out.Telemetry = telemetry
	}

	return nil
}

func populateMixerTelemetryConfig(in *v1.HelmValues, out *v2.MixerTelemetryConfig) (bool, error) {
	setValues := false

	rawMixerValues, ok, err := in.GetMap("mixer")
	if err != nil {
		return false, err
	} else if !ok || len(rawMixerValues) == 0 {
		rawMixerValues = make(map[string]interface{})
	}
	mixerValues := v1.NewHelmValues(rawMixerValues)

	rawV1TelemetryValues, ok, err := mixerValues.GetMap("telemetry")
	if err != nil {
		return false, err
	} else if !ok || len(rawV1TelemetryValues) == 0 {
		rawV1TelemetryValues = make(map[string]interface{})
	}
	v1TelemetryValues := v1.NewHelmValues(rawV1TelemetryValues)

	if sessionAffinityEnabled, ok, err := v1TelemetryValues.GetAndRemoveBool("sessionAffinityEnabled"); ok {
		out.SessionAffinity = &sessionAffinityEnabled
		setValues = true
	} else if err != nil {
		return false, nil
	}

	loadshedding := &v2.TelemetryLoadSheddingConfig{}
	setLoadshedding := false
	if mode, ok, err := v1TelemetryValues.GetAndRemoveString("loadshedding.mode"); ok && mode != "" {
		loadshedding.Mode = mode
		setLoadshedding = true
	} else if err != nil {
		return false, nil
	}
	if latencyThreshold, ok, err := v1TelemetryValues.GetAndRemoveString("loadshedding.latencyThreshold"); ok && latencyThreshold != "" {
		loadshedding.LatencyThreshold = latencyThreshold
		setLoadshedding = true
	} else if err != nil {
		return false, nil
	}
	if setLoadshedding {
		out.Loadshedding = loadshedding
		setValues = true
	}

	batching := &v2.TelemetryBatchingConfig{}
	if applied, err := populateTelemetryBatchingConfig(v1TelemetryValues, batching); err != nil {
		return false, nil
	} else if applied {
		setValues = true
		out.Batching = batching
	}

	var telemetryAdaptersValues *v1.HelmValues
	if rawAdaptersValues, ok, err := mixerValues.GetMap("adapters"); ok {
		telemetryAdaptersValues = v1.NewHelmValues(rawAdaptersValues)
	} else if err != nil {
		return false, err
	}

	if telemetryAdaptersValues != nil {
		adapters := &v2.MixerTelemetryAdaptersConfig{}
		setAdapters := false
		if useAdapterCRDs, ok, err := telemetryAdaptersValues.GetAndRemoveBool("useAdapterCRDs"); ok {
			adapters.UseAdapterCRDs = &useAdapterCRDs
			setAdapters = true
		} else if err != nil {
			return false, err
		}
		if kubernetesenv, ok, err := telemetryAdaptersValues.GetAndRemoveBool("kubernetesenv.enabled"); ok {
			adapters.KubernetesEnv = &kubernetesenv
			setAdapters = true
		} else if err != nil {
			return false, err
		}
		stdio := &v2.MixerTelemetryStdioConfig{}
		setStdio := false
		if enabled, ok, err := telemetryAdaptersValues.GetAndRemoveBool("stdio.enabled"); ok {
			stdio.Enabled = &enabled
			setStdio = true
		} else if err != nil {
			return false, err
		}
		if outputAsJSON, ok, err := telemetryAdaptersValues.GetAndRemoveBool("stdio.outputAsJson"); ok {
			stdio.OutputAsJSON = &outputAsJSON
		} else if err != nil {
			return false, err
		}
		if setStdio {
			adapters.Stdio = stdio
			setAdapters = true
		}
		if setAdapters {
			out.Adapters = adapters
			setValues = true
		}
		if len(telemetryAdaptersValues.GetContent()) == 0 {
			mixerValues.RemoveField("adapters")
		} else if err := mixerValues.SetField("adapters", telemetryAdaptersValues.GetContent()); err != nil {
			return false, err
		}
	}

	// update the mixer settings
	if len(v1TelemetryValues.GetContent()) == 0 {
		mixerValues.RemoveField("telemetry")
	} else if err := mixerValues.SetField("telemetry", v1TelemetryValues.GetContent()); err != nil {
		return false, err
	}
	if len(mixerValues.GetContent()) == 0 {
		in.RemoveField("mixer")
	} else if err := in.SetField("mixer", mixerValues.GetContent()); err != nil {
		return false, err
	}

	return setValues, nil
}

func populateTelemetryBatchingConfig(in *v1.HelmValues, out *v2.TelemetryBatchingConfig) (bool, error) {
	setValues := false
	if reportBatchMaxTime, ok, err := in.GetAndRemoveString("reportBatchMaxTime"); ok {
		out.MaxTime = reportBatchMaxTime
		setValues = true
	} else if err != nil {
		return false, err
	}
	if rawReportBatchMaxEntries, ok, err := in.GetAndRemoveInt64("reportBatchMaxEntries"); ok {
		reportBatchMaxEntries := int32(rawReportBatchMaxEntries)
		out.MaxEntries = &reportBatchMaxEntries
		setValues = true
	} else if err != nil {
		return false, err
	}

	return setValues, nil
}

func populateRemoteTelemetryConfig(in *v1.HelmValues, out *v2.RemoteTelemetryConfig) (bool, error) {
	setValues := false

	if remoteTelemetryAddress, ok, err := in.GetAndRemoveString("global.remoteTelemetryAddress"); ok && remoteTelemetryAddress != "" {
		out.Address = remoteTelemetryAddress
		setValues = true
	} else if err != nil {
		return false, err
	}
	if createRemoteSvcEndpoints, ok, err := in.GetAndRemoveBool("global.createRemoteSvcEndpoints"); ok {
		out.CreateService = &createRemoteSvcEndpoints
		setValues = true
	} else if err != nil {
		return false, err
	}

	rawV1TelemetryValues, ok, err := in.GetMap("mixer.telemetry")
	if err != nil {
		return false, err
	} else if !ok || len(rawV1TelemetryValues) == 0 {
		rawV1TelemetryValues = make(map[string]interface{})
	}
	v1TelemetryValues := v1.NewHelmValues(rawV1TelemetryValues)

	batching := &v2.TelemetryBatchingConfig{}
	if applied, err := populateTelemetryBatchingConfig(v1TelemetryValues, batching); err != nil {
		return false, nil
	} else if applied {
		out.Batching = batching
		setValues = true
	}

	if len(v1TelemetryValues.GetContent()) == 0 {
		in.RemoveField("mixer.telemetry")
	} else if err := in.SetField("mixer.telemetry", v1TelemetryValues.GetContent()); err != nil {
		return false, err
	}

	return setValues, nil
}
