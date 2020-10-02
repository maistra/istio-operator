package versions

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
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

func validateGateway(name string, gateway *v2.GatewayConfig, gatewayNames sets.String, meshNamespaces sets.String, meshNamespace string, allErrors *[]error) {
	namespace := gateway.Namespace
	if namespace == "" {
		namespace = meshNamespace
	}
	if (gateway.Enabled == nil || *gateway.Enabled) && !meshNamespaces.Has(namespace) {
		*allErrors = append(*allErrors, fmt.Errorf("namespace %s for ingress gateway %s must be part of the mesh", namespace, name))
	}
	namespacedName := namespacedNameString(namespace, name)
	if gatewayNames.Has(namespacedName) {
		*allErrors = append(*allErrors, fmt.Errorf("multiple gateways defined for %s", namespacedName))
	} else {
		gatewayNames.Insert(namespacedName)
	}
	if reservedGatewayNames.Has(name) {
		*allErrors = append(*allErrors, fmt.Errorf("cannot define additional gateway named %s", name))
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
