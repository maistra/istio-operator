package versions

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

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
