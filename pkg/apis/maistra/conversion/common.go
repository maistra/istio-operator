package conversion

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
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
	if in == nil {
		return nil, nil
	}
	out := make(map[string]interface{})
	bytes, err := yaml.Marshal(in)
	if err == nil {
		err = yaml.Unmarshal(bytes, &out)
	}
	return out, err
}

func fromValues(in interface{}, out interface{}) error {
	if in == nil {
		return nil
	}
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
		err = yaml.Unmarshal(bytes, &out)
	}
	return out, err
}

func stringToInt32Slice(in string) ([]int32, error) {
	if in == "" {
		return make([]int32, 0), nil
	}
	inslice := strings.Split(in, ",")
	out := make([]int32, len(inslice))
	for index, strval := range inslice {
		intval, err := strconv.ParseInt(strval, 10, 32)
		if err != nil {
			return nil, err
		}
		out[index] = int32(intval)
	}
	return out, nil
}

func int32SliceToString(in []int32) string {
	strslice := make([]string, len(in))
	for index, intval := range in {
		strslice[index] = strconv.FormatInt(int64(intval), 10)
	}
	return strings.Join(strslice, ",")
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

func setHelmFloatValue(obj map[string]interface{}, path string, value float64) error {
	return setHelmValue(obj, path, value)
}

func setHelmBoolValue(obj map[string]interface{}, path string, value bool) error {
	return setHelmValue(obj, path, value)
}

func setHelmStringSliceValue(obj map[string]interface{}, path string, value []string) error {
	vallen := len(value)
	rawval := make([]interface{}, vallen)
	for index, val := range value {
		rawval[index] = val
	}
	return setHelmValue(obj, path, rawval)
}

func setHelmStringMapValue(obj map[string]interface{}, path string, value map[string]string) error {
	rawValue, err := toValues(value)
	if err != nil {
		return err
	}
	return setHelmValue(obj, path, rawValue)
}
