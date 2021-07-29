package versions

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

func init() {
	scheme = runtime.NewScheme()
	scheme.AddKnownTypes(v1.SchemeGroupVersion, &v1.ServiceMeshControlPlane{})
	scheme.AddKnownTypes(v2.SchemeGroupVersion, &v2.ServiceMeshControlPlane{})
	decoder = json.NewYAMLSerializer(json.DefaultMetaFactory, scheme, scheme)
}

var scheme *runtime.Scheme
var decoder runtime.Decoder

const (
	clusterIngressName = "istio-ingressgateway"
	clusterEgressName = "istio-egressgateway"
)

var reservedGatewayNames = sets.NewString(clusterIngressName, clusterEgressName)

func validatePrometheusEnabledWhenKialiEnabled(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.IsKialiEnabled() && !spec.IsPrometheusEnabled() {
		return append(allErrors, fmt.Errorf(".spec.addons.prometheus.enabled must be true when .spec.addons.kiali.enabled is true"))
	}
	return allErrors
}

func validatePolicyType(ctx context.Context, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec, v version, allErrors []error) []error {
	// I believe the only settings that aren't supported are Istiod policy and telemetry
	policy := spec.Policy
	if policy == nil {
		return allErrors
	}
	if v == V1_0 || v == V1_1 {
		if policy.Type == v2.PolicyTypeIstiod {
			allErrors = append(allErrors, fmt.Errorf("policy type %s is not supported in version %s", policy.Type, v.String()))
		} else if policy.Type == v2.PolicyTypeRemote {
			if spec.Telemetry == nil || spec.Telemetry.Type != v2.TelemetryTypeRemote {
				allErrors = append(allErrors, fmt.Errorf("if policy type is Remote, telemetry type must also be Remote for v1.x versions"))
			}
		}
		if policy.Mixer != nil {
			if policy.Mixer.Adapters != nil {
				if policy.Mixer.Adapters.KubernetesEnv != nil &&
					(spec.Telemetry == nil || spec.Telemetry.Mixer == nil || spec.Telemetry.Mixer.Adapters == nil ||
						spec.Telemetry.Mixer.Adapters.KubernetesEnv == nil ||
						*spec.Telemetry.Mixer.Adapters.KubernetesEnv != *policy.Mixer.Adapters.KubernetesEnv) {
					allErrors = append(allErrors, fmt.Errorf("if policy.mixer.adapters.kubernetesenv is specified, it must match telemetry.mixer.adapters.kubernetesenv"))
				}
			}
		}
	} else {
		if policy.Remote != nil && policy.Remote.Address != "" && policy.Type != v2.PolicyTypeRemote {
			allErrors = append(allErrors, fmt.Errorf("if policy.remote.address is specified, policy.type must be Remote for v2.x versions"))
		}
	}

	// all versions
	if policy.Type == v2.PolicyTypeRemote {
		if policy.Remote == nil || policy.Remote.Address == "" {
			allErrors = append(allErrors, fmt.Errorf("if policy type is Remote, an address must be specified"))
		}
	}
	if policy.Remote != nil && policy.Remote.CreateService != nil &&
		(spec.Telemetry == nil || spec.Telemetry.Remote == nil || spec.Telemetry.Remote.CreateService == nil ||
			*spec.Telemetry.Remote.CreateService != *policy.Remote.CreateService) {
		allErrors = append(allErrors, fmt.Errorf("if policy.remote.createService is specified, it must match telemetry.remote.createService"))
	}
	if policy.Mixer != nil && policy.Mixer.Adapters != nil && policy.Mixer.Adapters.UseAdapterCRDs != nil && *policy.Mixer.Adapters.UseAdapterCRDs {
		allErrors = append(allErrors, fmt.Errorf("if policy.mixer.adapters.useAdapterCRDs is not supported for this version"))
	}
	return allErrors
}

func validateTelemetryType(ctx context.Context, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec, v version, allErrors []error) []error {
	telemetry := spec.Telemetry
	if telemetry == nil {
		return allErrors
	}
	if v == V1_0 || v == V1_1 {
		if telemetry.Type == v2.TelemetryTypeIstiod {
			allErrors = append(allErrors, fmt.Errorf("telemetry type %s is not supported in version %s", spec.Telemetry.Type, v.String()))
		} else if telemetry.Type == v2.TelemetryTypeRemote {
			if spec.Policy == nil || spec.Policy.Type != v2.PolicyTypeRemote {
				allErrors = append(allErrors, fmt.Errorf("if telemetry type is Remote, policy type must also be Remote for v1.x versions"))
			}
		}
		if telemetry.Mixer != nil {
			if telemetry.Mixer.Adapters != nil {
				if telemetry.Mixer.Adapters.KubernetesEnv != nil &&
					(spec.Policy == nil || spec.Policy.Mixer == nil || spec.Policy.Mixer.Adapters == nil ||
						spec.Policy.Mixer.Adapters.KubernetesEnv == nil ||
						*spec.Policy.Mixer.Adapters.KubernetesEnv != *telemetry.Mixer.Adapters.KubernetesEnv) {
					allErrors = append(allErrors, fmt.Errorf("if telemetry.mixer.adapters.kubernetesenv is specified, it must match policy.mixer.adapters.kubernetesenv"))
				}
			}
		}
	} else {
		if telemetry.Remote != nil && telemetry.Remote.Address != "" && telemetry.Type != v2.TelemetryTypeRemote {
			allErrors = append(allErrors, fmt.Errorf("if telemetry.remote.address is specified, telemetry.type must be Remote for v2.x versions"))
		}
	}
	// all versions
	if telemetry.Type == v2.TelemetryTypeRemote {
		if telemetry.Remote == nil || telemetry.Remote.Address == "" {
			allErrors = append(allErrors, fmt.Errorf("if telemetry type is Remote, an address must be specified"))
		}
	}
	if telemetry.Remote != nil && telemetry.Remote.CreateService != nil &&
		(spec.Policy == nil || spec.Policy.Remote == nil || spec.Policy.Remote.CreateService == nil ||
			*spec.Policy.Remote.CreateService != *telemetry.Remote.CreateService) {
		allErrors = append(allErrors, fmt.Errorf("if telemetry.remote.createService is specified, it must match policy.remote.createService"))
	}
	if telemetry.Mixer != nil && telemetry.Mixer.Adapters != nil && telemetry.Mixer.Adapters.UseAdapterCRDs != nil && *telemetry.Mixer.Adapters.UseAdapterCRDs {
		allErrors = append(allErrors, fmt.Errorf("if telemetry.mixer.adapters.useAdapterCRDs is not supported for this version"))
	}
	return allErrors
}

func validateGateways(ctx context.Context, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec, v version, cl client.Client, allErrors []error) []error {
	smmr := &v1.ServiceMeshMemberRoll{}
	err := cl.Get(ctx, client.ObjectKey{Name: common.MemberRollName, Namespace: meta.Namespace}, smmr)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return []error{err}
		}
	}
	return validateGatewaysInternal(meta, spec, smmr, allErrors)
}

func validateGatewaysInternal(meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec, smmr *v1.ServiceMeshMemberRoll, allErrors []error) []error {
	meshNamespaces := common.GetMeshNamespaces(meta.Namespace, smmr)
	gatewayNames := sets.NewString()
	if spec.Gateways != nil {
		if spec.Gateways.ClusterIngress != nil {
			validateGatewayNamespace(clusterIngressName, &spec.Gateways.ClusterIngress.GatewayConfig, meshNamespaces, &allErrors)
		}
		if spec.Gateways.ClusterEgress != nil {
			validateGatewayNamespace(clusterEgressName, &spec.Gateways.ClusterEgress.GatewayConfig, meshNamespaces, &allErrors)
		}
		for name, gateway := range spec.Gateways.IngressGateways {
			validateAdditionalGateway(name, &gateway.GatewayConfig, gatewayNames, meshNamespaces, meta.Namespace, &allErrors)
		}
		for name, gateway := range spec.Gateways.EgressGateways {
			validateAdditionalGateway(name, &gateway.GatewayConfig, gatewayNames, meshNamespaces, meta.Namespace, &allErrors)
		}
	}
	return allErrors
}

func validateAdditionalGateway(name string, gateway *v2.GatewayConfig, gatewayNames sets.String, meshNamespaces sets.String, meshNamespace string, allErrors *[]error) {
	validateGatewayNamespace(name, gateway, meshNamespaces, allErrors)

	namespace := gateway.Namespace
	if namespace == "" {
		namespace = meshNamespace
	}
	namespacedName := namespacedNameString(namespace, name)
	if gatewayNames.Has(namespacedName) {
		*allErrors = append(*allErrors, fmt.Errorf("multiple gateways defined for %q", namespacedName))
	} else {
		gatewayNames.Insert(namespacedName)
	}

	if reservedGatewayNames.Has(name) {
		*allErrors = append(*allErrors, fmt.Errorf("cannot define additional gateway named %q", name))
	}
}

func validateGatewayNamespace(name string, gateway *v2.GatewayConfig, meshNamespaces sets.String, allErrors *[]error) {
	if (gateway.Enabled == nil || *gateway.Enabled) && gateway.Namespace != "" && !meshNamespaces.Has(gateway.Namespace) {
		*allErrors = append(*allErrors, fmt.Errorf("namespace %q for gateway %q is not configured as a mesh member", gateway.Namespace, name))
	}
}

func errForEnabledValue(obj *v1.HelmValues, path string, disallowed bool) error {
	val, ok, _ := obj.GetFieldNoCopy(path)
	if ok {
		switch typedVal := val.(type) {
		case string:
			if strconv.FormatBool(disallowed) == strings.ToLower(typedVal) {
				return fmt.Errorf("%s=%t is not supported", path, disallowed)
			}
		case bool:
			if disallowed == typedVal {
				return fmt.Errorf("%s=%t is not supported", path, disallowed)
			}
		}
	}
	return nil
}

func errForValue(obj *v1.HelmValues, path string, value string) error {
	val, ok, _ := obj.GetFieldNoCopy(path)
	if ok {
		switch typedVal := val.(type) {
		case string:
			if typedVal == value {
				return fmt.Errorf("%s=%s is not supported", path, value)
			}
		}
	}
	return nil
}

func errForStringValue(obj *v1.HelmValues, path string, allowedValues sets.String) error {
	if val, ok, err := obj.GetString(path); ok && !allowedValues.Has(val) {
		return fmt.Errorf("%s=%s is not allowed", path, val)
	} else if err != nil {
		return fmt.Errorf("expected string value at %s: %s", path, err)
	}
	return nil
}

func getMapKeys(obj *v1.HelmValues, path string) []string {
	val, ok, err := obj.GetFieldNoCopy(path)
	if err != nil || !ok {
		return []string{}
	}
	mapVal, ok := val.(map[string]interface{})
	if !ok {
		return []string{}
	}
	keys := make([]string, len(mapVal))
	for k := range mapVal {
		keys = append(keys, k)
	}
	return keys
}

func namespacedNameString(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

type ValidationError interface {
	errors.Aggregate
}

type validationError struct {
	aggregate errors.Aggregate
}

func (e *validationError) Error() string {
	return e.aggregate.Error()
}

func (e *validationError) Errors() []error {
	return e.aggregate.Errors()
}

func (e *validationError) Is(target error) bool {
	return e.aggregate.Is(target)
}

func NewValidationError(errlist ...error) ValidationError {
	if len(errlist) == 0 {
		return nil
	}
	return &validationError{
		aggregate: errors.NewAggregate(errlist),
	}
}

func IsValidationError(err error) bool {
	_, ok := err.(*validationError)
	return ok
}

type dependencyMissingError struct {
	dependency string
	err        error
}

func (e *dependencyMissingError) Error() string {
	return e.err.Error()
}

func NewDependencyMissingError(dependency string, err error) error {
	return &dependencyMissingError{
		dependency: dependency,
		err:        err,
	}
}

func IsDependencyMissingError(err error) bool {
	_, ok := err.(*dependencyMissingError)
	return ok
}

func GetMissingDependency(err error) string {
	if e, ok := err.(*dependencyMissingError); ok {
		return e.dependency
	}
	return ""
}
