package helm

import (
	"reflect"
	"testing"
)

func TestGetString(t *testing.T) {
	testCases := []struct {
		name        string
		input       HelmValues
		key         string
		expectFound bool
		expected    string
		expectErr   bool
	}{
		{
			name: "valid Key",
			input: HelmValues{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			},
			key:         "foo.bar",
			expectFound: true,
			expected:    "baz",
		},
		{
			name: "invalid Key",
			input: HelmValues{
				"foo": "baz",
			},
			key:         "foo.bar",
			expectFound: false,
			expectErr:   true,
		},
		{
			name: "nonexistent key",
			input: HelmValues{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			},
			key:         "foo.baz",
			expectFound: false,
			expectErr:   false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			result, found, err := test.input.GetString(test.key)
			if test.expectErr {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got an error: %v", err)
				}
				if result != test.expected {
					t.Errorf("Expected %s, but got %s", test.expected, result)
				}
			}

			if found != test.expectFound {
				t.Errorf("Expected found to be %v, but it was %v", test.expectFound, found)
			}
		})
	}
}

func TestSet(t *testing.T) {
	tests := []struct {
		name      string
		input     HelmValues
		key       string
		val       string
		expected  HelmValues
		expectErr bool
	}{
		{
			name: "Valid Key",
			input: HelmValues{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			},
			key: "foo.bar",
			val: "newVal",
			expected: HelmValues{
				"foo": map[string]interface{}{
					"bar": "newVal",
				},
			},
		},
		{
			name: "Non-Existent Key",
			input: HelmValues{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			},
			key: "foo.baz",
			val: "newVal",
			expected: HelmValues{
				"foo": map[string]interface{}{
					"bar": "baz",
					"baz": "newVal",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.input.Set(test.key, test.val)
			if test.expectErr {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got an error: %v", err)
				}
				if !reflect.DeepEqual(test.input, test.expected) {
					t.Errorf("Expected %v, but got %v", test.expected, test.input)
				}
			}
		})
	}
}
