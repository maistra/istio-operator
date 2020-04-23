package v1

import (
	"encoding/json"
	"fmt"
	"strings"
)

// HelmValues is typedef for Helm .Values
// +kubebuilder:validation:XPreserveUnknownFields
type HelmValues struct {
	data map[string]interface{} `json:"-"`
}

func NewHelmValues(values map[string]interface{}) *HelmValues {
	return &HelmValues{data: values}
}

func (h *HelmValues) GetContent() map[string]interface{} {
	if h == nil {
		return nil
	}
	return h.data
}

func (h *HelmValues) GetField(path string) (interface{}, bool, error) {
	if h == nil || h.data == nil {
		return nil, false, nil
	}

	var val interface{} = h.data
	fields := strings.Split(path, ".")
	for i, field := range fields {
		if m, ok := val.(map[string]interface{}); ok {
			val, ok = m[field]
			if !ok {
				return nil, false, nil
			}
		} else {
			return nil, false, fmt.Errorf("%v accessor error: %v is of the type %T, expected map[string]interface{}", strings.Join(fields[:i+1], "."), val, val)
		}
	}
	return val, true, nil
}

func (h *HelmValues) GetBool(path string) (bool, bool, error) {
	value, found, err := h.GetField(path)
	if !found || err != nil {
		return false, found, err
	}
	b, ok := value.(bool)
	if !ok {
		return false, false, fmt.Errorf("%v accessor error: %v is of the type %T, expected bool", path, value, value)
	}
	return b, true, nil
}

func (h *HelmValues) GetString(path string) (string, bool, error) {
	value, found, err := h.GetField(path)
	if !found || err != nil {
		return "", found, err
	}
	s, ok := value.(string)
	if !ok {
		return "", false, fmt.Errorf("%v accessor error: %v is of the type %T, expected string", path, value, value)
	}
	return s, true, nil

}

func (h *HelmValues) GetMap(path string) (map[string]interface{}, bool, error) {
	value, found, err := h.GetField(path)
	if !found || err != nil {
		return nil, found, err
	}
	m, ok := value.(map[string]interface{})
	if !ok {
		return nil, false, fmt.Errorf("%v accessor error: %v is of the type %T, expected map[string]interface{}", path, value, value)
	}
	return m, true, nil
}

func (h *HelmValues) SetField(path string, value interface{}) error {
	if h == nil {
		panic("Tried to invoke SetField on nil *HelmValues")
	}
	if h.data == nil {
		h.data = map[string]interface{}{}
	}
	m := h.data
	fields := strings.Split(path, ".")
	for i, field := range fields[:len(fields)-1] {
		if val, ok := m[field]; ok {
			if valMap, ok := val.(map[string]interface{}); ok {
				m = valMap
			} else {
				return fmt.Errorf("value cannot be set because %v is not a map[string]interface{}", strings.Join(fields[:i+1], "."))
			}
		} else {
			newVal := make(map[string]interface{})
			m[field] = newVal
			m = newVal
		}
	}
	m[fields[len(fields)-1]] = value
	return nil
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
