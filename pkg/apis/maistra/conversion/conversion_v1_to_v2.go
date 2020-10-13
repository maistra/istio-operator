package conversion

import (
	"strings"

	conversion "k8s.io/apimachinery/pkg/conversion"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func v1ToV2Hacks(in *v1.ControlPlaneSpec, values *v1.HelmValues) error {
	// adjustments for 3scale
	if in.ThreeScale != nil {
		values.SetField("3scale", in.ThreeScale.DeepCopy().GetContent())
	}

	// move tracing.jaeger.annotations to tracing.jaeger.podAnnotations
	if jaegerAnnotations, ok, err := values.GetFieldNoCopy("tracing.jaeger.annotations"); ok {
		if err := values.SetField("tracing.jaeger.podAnnotations", jaegerAnnotations); err != nil {
			return err
		}
		values.RemoveField("tracing.jaeger.annotations")
	} else if err != nil {
		return err
	}
	// normalize jaeger images
	if agentImage, ok, err := values.GetAndRemoveString("tracing.jaeger.agentImage"); ok {
		if err := values.SetField("tracing.jaeger.agent.image", agentImage); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if allInOneImage, ok, err := values.GetAndRemoveString("tracing.jaeger.allInOneImage"); ok {
		if err := values.SetField("tracing.jaeger.allInOne.image", allInOneImage); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if collectorImage, ok, err := values.GetAndRemoveString("tracing.jaeger.collectorImage"); ok {
		if err := values.SetField("tracing.jaeger.collector.image", collectorImage); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if queryImage, ok, err := values.GetAndRemoveString("tracing.jaeger.queryImage"); ok {
		if err := values.SetField("tracing.jaeger.query.image", queryImage); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}

// Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec converts a v1 ControlPlaneSpec to its v2 equivalent.
func Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec(in *v1.ControlPlaneSpec, out *v2.ControlPlaneSpec, s conversion.Scope) error {

	version, versionErr := versions.ParseVersion(in.Version)
	if versionErr != nil {
		return versionErr
	}
	if err := autoConvert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec(in, out, s); err != nil {
		return err
	}

	// legacy Template field
	if len(in.Profiles) == 0 && in.Template != "" {
		out.Profiles = []string{in.Template}
	}

	// copy to preserve input
	values := in.Istio.DeepCopy()
	if techPreview, ok, err := values.GetMap("techPreview"); ok {
		if len(techPreview) > 0 {
			out.TechPreview = v1.NewHelmValues(techPreview).DeepCopy()
		}
		delete(values.GetContent(), "techPreview")
	} else if err != nil {
		return err
	}

	if err := v1ToV2Hacks(in, values); err != nil {
		return err
	}

	// Cluster settings
	if err := populateClusterConfig(values, out); err != nil {
		return err
	}

	// General
	if err := populateGeneralConfig(values, out); err != nil {
		return err
	}

	// Policy - ensure policy runs before telemetry, as both may use mixer.adapters
	// policy won't remove these from values, but telemetry will
	if err := populatePolicyConfig(values, out, version); err != nil {
		return err
	}

	// Proxy
	if err := populateProxyConfig(values, out); err != nil {
		return err
	}

	// Security
	if err := populateSecurityConfig(values, out); err != nil {
		return err
	}

	// Telemetry
	if err := populateTelemetryConfig(values, out, version); err != nil {
		return err
	}

	// Tracing
	if err := populateTracingConfig(values, out); err != nil {
		return err
	}

	// Gateways
	if err := populateGatewaysConfig(values, out); err != nil {
		return err
	}

	// Addons
	if err := populateAddonsConfig(values, out); err != nil {
		return err
	}

	// Runtime
	if _, err := populateControlPlaneRuntimeConfig(values, out); err != nil {
		return err
	}

	// remove common mixer settings (used by both telemetry and policy)
	values.RemoveField("global.istioRemote")

	// save anything that's left for proper round tripping
	if len(values.GetContent()) > 0 {
		if out.TechPreview == nil {
			out.TechPreview = v1.NewHelmValues(make(map[string]interface{}))
		}
		if err := overwriteHelmValues(out.TechPreview.GetContent(), values.GetContent()); err != nil {
			return err
		}
		if len(out.TechPreview.GetContent()) == 0 {
			out.TechPreview = nil
		}
	}

	return nil
}

func Convert_v1_ControlPlaneStatus_To_v2_ControlPlaneStatus(in *v1.ControlPlaneStatus, out *v2.ControlPlaneStatus, s conversion.Scope) error {
	if err := autoConvert_v1_ControlPlaneStatus_To_v2_ControlPlaneStatus(in, out, s); err != nil {
		return err
	}
	// WARNING: in.OperatorVersion requires manual conversion: does not exist in peer-type
	lastDash := strings.LastIndex(in.ReconciledVersion, "-")
	if lastDash >= 0 {
		out.OperatorVersion = in.ReconciledVersion[:lastDash]
	}
	// WARNING: in.ChartVersion requires manual conversion: does not exist in peer-type
	// WARNING: in.AppliedValues requires manual conversion: does not exist in peer-type
	in.LastAppliedConfiguration.DeepCopyInto(&out.AppliedValues)
	// WARNING: in.AppliedSpec requires manual conversion: does not exist in peer-type
	if err := Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec(&in.LastAppliedConfiguration, &out.AppliedSpec, s); err != nil {
		return err
	}
	return nil
}
