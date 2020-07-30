package v1

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
)

// HelmValues is typedef for Helm .Values
// +kubebuilder:validation:XPreserveUnknownFields
type HelmValues struct {
	data map[string]interface{} `json:"-"`
}

func NewHelmValues(values map[string]interface{}) *HelmValues {
	if values == nil {
		values = make(map[string]interface{})
	}
	return &HelmValues{data: values}
}

func (h *HelmValues) GetContent() map[string]interface{} {
	if h == nil {
		return nil
	}
	return h.data
}

func (h *HelmValues) GetFieldNoCopy(path string) (interface{}, bool, error) {
	if h == nil || h.data == nil {
		return nil, false, nil
	}
	return unstructured.NestedFieldNoCopy(h.data, strings.Split(path, ".")...)
}

func (h *HelmValues) GetBool(path string) (bool, bool, error) {
	if h == nil || h.data == nil {
		return false, false, nil
	}
	return unstructured.NestedBool(h.data, strings.Split(path, ".")...)
}

func (h *HelmValues) GetString(path string) (string, bool, error) {
	if h == nil || h.data == nil {
		return "", false, nil
	}
	return unstructured.NestedString(h.data, strings.Split(path, ".")...)
}

func (h *HelmValues) GetInt64(path string) (int64, bool, error) {
	if h == nil || h.data == nil {
		return 0, false, nil
	}
	return unstructured.NestedInt64(h.data, strings.Split(path, ".")...)
}

func (h *HelmValues) GetStringSlice(path string) ([]string, bool, error) {
	if h == nil || h.data == nil {
		return nil, false, nil
	}
	return unstructured.NestedStringSlice(h.data, strings.Split(path, ".")...)
}

func (h *HelmValues) GetSlice(path string) ([]interface{}, bool, error) {
	if h == nil || h.data == nil {
		return nil, false, nil
	}
	return unstructured.NestedSlice(h.data, strings.Split(path, ".")...)
}

func (h *HelmValues) GetMap(path string) (map[string]interface{}, bool, error) {
	if h == nil || h.data == nil {
		return nil, false, nil
	}
	return unstructured.NestedMap(h.data, strings.Split(path, ".")...)
}

func (h *HelmValues) SetField(path string, value interface{}) error {
	if h == nil {
		panic("Tried to invoke SetField on nil *HelmValues")
	}
	if h.data == nil {
		h.data = map[string]interface{}{}
	}
	return unstructured.SetNestedField(h.data, value, strings.Split(path, ".")...)
}

func (h *HelmValues) UnmarshalJSON(in []byte) error {
	err := json.Unmarshal(in, &h.data)
	if err != nil {
		return err
	}
	return nil
}

func (h *HelmValues) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.data)
}

func (in *HelmValues) DeepCopyInto(out *HelmValues) {
	*out = HelmValues{}
	
	data, err := json.Marshal(in)
	if err != nil {
		// panic ???
		return
	}
	err = json.Unmarshal(data, out)
	if err != nil {
		// panic ???
		return
	}
	return
}
