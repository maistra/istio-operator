package conversion

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// toValues converts in to a generic values.yaml format
func toValues(in interface{}) (map[string]interface{}, error) {
	out := make(map[string]interface{})
	bytes, err := yaml.Marshal(in)
	if err == nil {
		err = yaml.Unmarshal(bytes, out, nil)
	}
	return out, err
}

func setHelmValue(obj map[string]interface{}, path string, value interface{}) error {
	return unstructured.SetNestedField(obj, value, strings.Split(path, ".")...)
}

func setHelmStringValue(obj map[string]interface{}, path string, value string) error {
	return setHelmValue(obj, path, value)
}

func setHelmIntValue(obj map[string]interface{}, path string, value int64) error {
	return setHelmValue(obj, path, value)
}

func setHelmBoolValue(obj map[string]interface{}, path string, value bool) error {
	return setHelmValue(obj, path, value)
}

func setHelmSliceValue(obj map[string]interface{}, path string, value []string) error {
	return setHelmValue(obj, path, value)
}

func setHelmMapValue(obj map[string]interface{}, path string, value map[string]string) error {
	return setHelmValue(obj, path, value)
}
