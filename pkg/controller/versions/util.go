package versions

import (
	"context"
	"fmt"
	"strings"
	"time"

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

var (
	scheme  *runtime.Scheme
	decoder runtime.Decoder
)

const (
	clusterIngressName = "istio-ingressgateway"
	clusterEgressName  = "istio-egressgateway"
)

var reservedGatewayNames = sets.NewString(clusterIngressName, clusterEgressName)

func checkControlPlaneModeNotSet(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.Mode != "" {
		return append(allErrors, fmt.Errorf("the spec.mode field is only supported in version 2.4 and above"))
	} else if spec.TechPreview != nil {
		if _, found, _ := spec.TechPreview.GetString(v2.TechPreviewControlPlaneModeKey); found {
			return append(allErrors,
				fmt.Errorf("the spec.techPreview.%s field is only supported in version 2.3",
					v2.TechPreviewControlPlaneModeKey))
		}
	}
	return allErrors
}

func checkMeshConfigNotSet(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.MeshConfig != nil {
		return append(allErrors, fmt.Errorf("the spec.meshConfig field is only supported in version 2.4 and above"))
	}
	return allErrors
}

func checkDiscoverySelectorsNotSet(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.MeshConfig != nil && spec.MeshConfig.DiscoverySelectors != nil {
		return append(allErrors, fmt.Errorf("the spec.meshConfig.discoverySelectors field is only supported in version 2.4 and above"))
	}
	return allErrors
}

func validateOverlappingCaCertConfigMapNames(meta *metav1.ObjectMeta, newSmcp *v2.ControlPlaneSpec, smcps *v2.ServiceMeshControlPlaneList, allErrors []error) []error {
	overlappingCaCertNameErr := fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName")
	for _, smcp := range smcps.Items {
		if meta.GetUID() == smcp.GetUID() {
			continue
		}
		// do not allow creating an SMCP when at least one cluster-wide SMCP exists and CA root cert config map names overlap
		if (smcp.Spec.IsClusterScoped() || newSmcp.IsClusterScoped()) && newSmcp.GetCaCertConfigMapName() == smcp.Spec.GetCaCertConfigMapName() {
			return append(allErrors, overlappingCaCertNameErr)
		}
	}
	return allErrors
}

func validateGlobal(ctx context.Context, version Ver, meta *metav1.ObjectMeta, newSmcp *v2.ControlPlaneSpec, cl client.Client, allErrors []error) []error {
	smcps := v2.ServiceMeshControlPlaneList{}
	err := cl.List(ctx, &smcps)
	if err != nil {
		return append(allErrors, err)
	}

	otherSmcpExists := fmt.Errorf("a cluster-scoped SMCP may only be created when no other SMCPs exist")
	//overlappingCaCertNameErr := fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName")
	if newSmcp.IsClusterScoped() {
		// an SMCP already exists and new one is created
		if len(smcps.Items) == 1 && smcps.Items[0].UID != meta.GetUID() {
			currentSmcp := smcps.Items[0].Spec
			if currentSmcp.IsClusterScoped() {
				// allow cluster-wide gateway controller when another cluster-wide non gateway controller already exists
				// this is the case where openshift-ingress controller and cluster-wide mesh co-exist
				if (newSmcp.IsGatewayController() && !currentSmcp.IsGatewayController()) || (!newSmcp.IsGatewayController() && currentSmcp.IsGatewayController()) {
					goto ValidateOverlappingConfigMapNames
				}
				// do not allow more than 1 cluster-wide gateway controller
				if newSmcp.IsGatewayController() && currentSmcp.IsGatewayController() {
					return append(allErrors, otherSmcpExists)
				}
			}
			// only cluster-wide gateway controller can be created when another SMCP exist
			if !newSmcp.IsGatewayController() {
				return append(allErrors, otherSmcpExists)
			}
		}
		if len(smcps.Items) > 1 {
			if newSmcp.IsGatewayController() && countGatewayControllers(smcps.Items) > 1 ||
				!newSmcp.IsGatewayController() && countGatewayControllers(smcps.Items) == 0 {
				return append(allErrors, otherSmcpExists)
			}
		}
	} else {
		for _, smcp := range smcps.Items {
			if smcp.UID == meta.GetUID() {
				continue
			}
			// do not allow creating multi-tenant mesh when another (non-gateway controller) cluster-wide mesh already exists
			if smcp.Spec.IsClusterScoped() && !smcp.Spec.IsGatewayController() {
				return append(allErrors,
					fmt.Errorf("no other SMCPs may be created when a cluster-scoped SMCP exists"))
			}
		}
	}
ValidateOverlappingConfigMapNames:
	allErrors = append(allErrors, validateOverlappingCaCertConfigMapNames(meta, newSmcp, &smcps, allErrors)...)
	return allErrors
}

func countGatewayControllers(existingSmcps []v2.ServiceMeshControlPlane) int {
	var foundGatewayControllersCount int
	for _, smcp := range existingSmcps {
		if smcp.Spec.IsClusterScoped() && smcp.Spec.IsGatewayController() {
			foundGatewayControllersCount++
		}
	}
	return foundGatewayControllersCount
}

func checkDiscoverySelectors(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.Mode != v2.ClusterWideMode && spec.MeshConfig != nil && spec.MeshConfig.DiscoverySelectors != nil {
		return append(allErrors, fmt.Errorf("spec.meshConfig.discoverySelectors may only be used when spec.mode is ClusterWide"))
	}
	return allErrors
}

func validatePrometheusEnabledWhenKialiEnabled(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.IsKialiEnabled() && !spec.IsPrometheusEnabled() {
		return append(allErrors, fmt.Errorf(".spec.addons.prometheus.enabled must be true when .spec.addons.kiali.enabled is true"))
	}
	return allErrors
}

func validatePolicyType(spec *v2.ControlPlaneSpec, v Ver, allErrors []error) []error {
	// I believe the only settings that aren't supported are Istiod policy and telemetry
	policy := spec.Policy
	if policy == nil {
		return allErrors
	}
	if v == V1_1 {
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

func validateTelemetryType(spec *v2.ControlPlaneSpec, v Ver, allErrors []error) []error {
	telemetry := spec.Telemetry
	if telemetry == nil {
		return allErrors
	}
	if v == V1_1 {
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

func validateGateways(ctx context.Context, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec, cl client.Client, allErrors []error) []error {
	smmr := &v1.ServiceMeshMemberRoll{}
	err := cl.Get(ctx, client.ObjectKey{Name: common.MemberRollName, Namespace: meta.Namespace}, smmr)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return []error{err}
		}
	}
	return validateGatewaysInternal(meta, spec, allErrors)
}

func validateGatewaysInternal(meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec, allErrors []error) []error {
	gatewayNames := sets.NewString()
	if spec.Gateways != nil {
		for name, gateway := range spec.Gateways.IngressGateways {
			validateAdditionalGateway(name, &gateway.GatewayConfig, gatewayNames, meta.Namespace, &allErrors)
		}
		for name, gateway := range spec.Gateways.EgressGateways {
			validateAdditionalGateway(name, &gateway.GatewayConfig, gatewayNames, meta.Namespace, &allErrors)
		}
	}
	return allErrors
}

func validateAdditionalGateway(name string, gateway *v2.GatewayConfig, gatewayNames sets.String, meshNamespace string, allErrors *[]error) {
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

func validateProtocolDetection(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.Proxy == nil || spec.Proxy.Networking == nil || spec.Proxy.Networking.Protocol == nil || spec.Proxy.Networking.Protocol.AutoDetect == nil {
		return allErrors
	}
	autoDetect := spec.Proxy.Networking.Protocol.AutoDetect
	if autoDetect.Timeout != "" {
		if _, err := time.ParseDuration(autoDetect.Timeout); err != nil {
			allErrors = append(allErrors, fmt.Errorf("failed parsing spec.proxy.networking.protocol.autoDetect.timeout, not a valid duration: %s", err.Error()))
		}
	}
	return allErrors
}

func errForEnabledValue(obj *v1.HelmValues, path string) error {
	val, ok, _ := obj.GetFieldNoCopy(path)
	if ok {
		switch typedVal := val.(type) {
		case string:
			if strings.EqualFold(typedVal, "true") {
				return fmt.Errorf("%s=%t is not supported", path, true)
			}
		case bool:
			if typedVal {
				return fmt.Errorf("%s=%t is not supported", path, true)
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
