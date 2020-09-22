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

var reservedGatewayNames = sets.NewString("istio-ingressgateway", "istio-egressgateway")

func validatePrometheusEnabledWhenKialiEnabled(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.Addons != nil && spec.Addons.Kiali != nil && spec.Addons.Kiali.Enabled != nil && *spec.Addons.Kiali.Enabled == true &&
		(spec.Addons.Prometheus == nil || spec.Addons.Prometheus.Enabled == nil || *spec.Addons.Prometheus.Enabled != true) {
		return append(allErrors, fmt.Errorf(".spec.addons.prometheus.enabled must be true when .spec.addons.kiali.enabled is true"))
	}
	return allErrors
}

func validatePolicyType(ctx context.Context, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec, v version, allErrors []error) []error {
	// I believe the only settings that aren't supported are Istiod policy and telemetry
	if spec.Policy != nil && spec.Policy.Type == v2.PolicyTypeIstiod {
		allErrors = append(allErrors, fmt.Errorf("policy type %s is not supported in version %s", spec.Policy.Type, v.String()))
	}
	return allErrors
}

func validateTelemetryType(ctx context.Context, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec, v version, allErrors []error) []error {
	if spec.Telemetry != nil && spec.Telemetry.Type == v2.TelemetryTypeIstiod {
		allErrors = append(allErrors, fmt.Errorf("telemetry type %s is not supported in version %s", spec.Telemetry.Type, v.String()))
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

	meshNamespaces := common.GetMeshNamespaces(meta.Namespace, smmr)
	gatewayNames := sets.NewString()
	if spec.Gateways != nil {
		for name, gateway := range spec.Gateways.IngressGateways {
			validateGateway(name, &gateway.GatewayConfig, gatewayNames, meshNamespaces, meta.Namespace, &allErrors)
		}
		for name, gateway := range spec.Gateways.EgressGateways {
			validateGateway(name, &gateway.GatewayConfig, gatewayNames, meshNamespaces, meta.Namespace, &allErrors)
		}
	}
	return allErrors
}

func validateGateway(name string, gateway *v2.GatewayConfig, gatewayNames sets.String, meshNamespaces sets.String, meshNamespace string, allErrors *[]error) {
	namespace := gateway.Namespace
	if namespace == "" {
		namespace = meshNamespace
	}
	if (gateway.Enabled == nil || *gateway.Enabled) && !meshNamespaces.Has(namespace) {
		*allErrors = append(*allErrors, fmt.Errorf("namespace %q for ingress gateway %q must be part of the mesh", namespace, name))
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
