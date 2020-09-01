package v1

import (
	"fmt"
	"reflect"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestMarshall(t *testing.T) {
	testCases := []struct {
		name     string
		value    *HelmValues
		expected string
	}{
		{
			name:     "nil",
			value:    nil,
			expected: "null\n",
		},
		{
			name: "nested-map",
			value: NewHelmValues(map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			}),
			expected: "foo:\n" +
				"  bar: baz\n",
		},
		{
			name: "nil-value",
			value: NewHelmValues(map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": nil,
				},
			}),
			expected: "foo:\n" +
				"  bar: null\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualYaml, err := yaml.Marshal(tc.value)
			if err != nil {
				panic(fmt.Sprintf("Unexpected error: %v", err))
			}

			if string(actualYaml) != tc.expected {
				t.Fatalf("Unexpected YAML;\nexpected:\n%v\n\nactual:\n%v", tc.expected, string(actualYaml))
			}
			deserialized := NewHelmValues(map[string]interface{}{})
			if err := yaml.Unmarshal(actualYaml, &deserialized); err != nil {
				panic(fmt.Sprintf("Unexpected error: %v", err))
			}
			if !reflect.DeepEqual(deserialized, tc.value) {
				t.Fatalf("Unexpected deserialized value;\nexpected:\n%v\n\nactual:\n%v", tc.value, deserialized)
			}
			copy := tc.value.DeepCopy()
			if !reflect.DeepEqual(copy, tc.value) {
				t.Fatalf("Unexpected copy value;\nexpected:\n%v\n\nactual:\n%v", tc.value, copy)
			}
		})
	}
}

func TestSetField(t *testing.T) {
	testCases := []struct {
		name          string
		initial       *HelmValues
		value         interface{}
		path          string
		expected      *HelmValues
		errorExpected bool
	}{
		{
			name:    "nil-content",
			initial: NewHelmValues(nil),
			value:   "bar",
			path:    "foo",
			expected: NewHelmValues(
				map[string]interface{}{
					"foo": "bar",
				},
			),
		},
		{
			name:    "add-to-root-level",
			initial: NewHelmValues(map[string]interface{}{}),
			value:   "bar",
			path:    "foo",
			expected: NewHelmValues(
				map[string]interface{}{
					"foo": "bar",
				},
			),
		},
		{
			name: "add-to-existing-map",
			initial: NewHelmValues(map[string]interface{}{
				"map": map[string]interface{}{
					"bar": "baz",
				},
			}),
			value: "bar",
			path:  "map.foo",
			expected: NewHelmValues(
				map[string]interface{}{
					"map": map[string]interface{}{
						"foo": "bar",
						"bar": "baz",
					},
				},
			),
		},
		{
			name:    "create-map",
			initial: NewHelmValues(map[string]interface{}{}),
			value:   "bar",
			path:    "map.foo",
			expected: NewHelmValues(
				map[string]interface{}{
					"map": map[string]interface{}{
						"foo": "bar",
					},
				},
			),
		},
		{
			name: "existing-field-is-not-map",
			initial: NewHelmValues(map[string]interface{}{
				"map": "not-a-map",
			}),
			value:         "bar",
			path:          "map.foo",
			errorExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			actual := tc.initial
			err := actual.SetField(tc.path, tc.value)
			if tc.errorExpected {
				if err == nil {
					t.Fatalf("Expected error to be returned, but it wasn't")
				}
				return
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			}

			if !reflect.DeepEqual(tc.expected, actual) {
				t.Fatalf("Unexpected values;\nexpected:\n---\n%v---\n\nactual:\n---\n%v---", toYAML(tc.expected), toYAML(actual))
			}
		})
	}
}

func toYAML(values *HelmValues) string {
	bytes, err := yaml.Marshal(values)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(bytes)
}
