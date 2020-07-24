package conversion

import (
	"fmt"
	"strings"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func setMetadataLabels(labels map[string]interface{}, out *v2.MetadataConfig) error {
	if len(labels) > 0 {
		out.Labels = make(map[string]string)
		for key, value := range labels {
			if stringValue, ok := value.(string); ok {
				out.Labels[key] = stringValue
			} else {
				return fmt.Errorf("error casting label value to string")
			}
		}
	}
	return nil
}

func setMetadataAnnotations(annotations map[string]interface{}, out *v2.MetadataConfig) error {
	if len(annotations) > 0 {
		out.Annotations = make(map[string]string)
		for key, value := range annotations {
			if stringValue, ok := value.(string); ok {
				out.Annotations[key] = stringValue
			} else {
				return fmt.Errorf("error casting annotation value to string")
			}
		}
	}
	return nil
}

// toValues converts in to a generic values.yaml format
func toValues(in interface{}) (map[string]interface{}, error) {
	out := make(map[string]interface{})
	bytes, err := yaml.Marshal(in)
	if err == nil {
		err = yaml.Unmarshal(bytes, out, nil)
	}
	return out, err
}

func fromValues(in interface{}, out interface{}) error {
	inYAML, err := yaml.Marshal(in)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(inYAML, out)
}

func sliceToValues(in []interface{}) ([]interface{}, error) {
	out := make([]interface{}, len(in))
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

func setHelmStringSliceValue(obj map[string]interface{}, path string, value []string) error {
	return setHelmValue(obj, path, value)
}

func setHelmStringMapValue(obj map[string]interface{}, path string, value map[string]string) error {
	return setHelmValue(obj, path, value)
}

func getHelmBoolValue(obj map[string]interface{}, path string) *bool {
	val, found, err := unstructured.NestedFieldCopy(obj, strings.Split(path, ".")...)
	if !found || err != nil {
		return nil
	} else if valString := val.(string); strings.ToLower(valString) == "true" {
		ret := true
		return &ret
	} else {
		ret := false
		return &ret
	}
}

func getHelmStringValue(obj map[string]interface{}, path string) string {
	val, found, err := unstructured.NestedFieldCopy(obj, strings.Split(path, ".")...)
	if !found || err != nil {
		return ""
	} else if valString, ok := val.(string); ok {
		return valString
	}
	return ""
}

func getHelmStringSliceValue(obj map[string]interface{}, path string) []string {
	val, found, err := unstructured.NestedFieldCopy(obj, strings.Split(path, ".")...)
	if !found || err != nil {
		return nil
	} else if valString, ok := val.([]string); ok {
		return valString
	}
	return nil
}

func getHelmStringMapValue(obj map[string]interface{}, path string) map[string]string {
	val, found, err := unstructured.NestedFieldCopy(obj, strings.Split(path, ".")...)
	if !found || err != nil {
		return nil
	} else if mapVal, ok := val.(map[string]string); ok {
		return mapVal
	}
	return nil
}
